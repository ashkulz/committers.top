package github

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"most-active-github-users-counter/net"
)

const root string = "https://api.github.com/"
const usersPerGraphQLBatch = 10

// excludeLogins lists accounts that are not real human users.
// "claude" is the Anthropic Claude Code co-author account whose
// contributionsCollection causes GitHub GraphQL to return 504.
var excludeLogins = []string{
	"claude",
}

type HTTPGithubClient struct {
	wrappers []net.Wrapper
}

func (client HTTPGithubClient) Request(url string, body string) ([]byte, error) {
	httpClient := &http.Client{}
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest("POST", url, strings.NewReader(body))
	} else {
		req, err = http.NewRequest("GET", url, nil)
	}

	if err != nil {
		return []byte{}, err
	}

	return net.Compose(client.wrappers...)(net.MakeRequester(httpClient))(req)
}

func (client HTTPGithubClient) CurrentUser() (User, error) {
	body, err := client.Request(fmt.Sprintf("%suser", root), "")
	if err != nil {
		return User{}, err
	}

	user := User{}
	if err := json.Unmarshal(body, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (client HTTPGithubClient) User(login string) (User, error) {
	body, err := client.Request(fmt.Sprintf("%susers/%s", root, login), "")
	if err != nil {
		return User{}, err
	}

	user := User{}
	if err := json.Unmarshal(body, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

// SearchUsers fetches top users by followers using a two-step approach:
// 1. REST API GET /search/users with sort=followers (officially supported)
// 2. GraphQL user(login:) batch queries for detailed data (contributions, orgs)
func (client HTTPGithubClient) SearchUsers(query UserSearchQuery) (GithubSearchResults, error) {
	logins, totalUsersCount, err := client.searchUserLogins(query)
	if err != nil {
		return GithubSearchResults{}, err
	}

	users, err := client.fetchUserDetails(logins)
	if err != nil {
		return GithubSearchResults{}, err
	}

	return GithubSearchResults{
		Users:                users,
		MinimumFollowerCount: minFollowers(users),
		TotalUserCount:       totalUsersCount,
	}, nil
}

// searchUserLogins uses REST API to get user logins sorted by followers.
// The REST API officially supports sort=followers for user search.
func (client HTTPGithubClient) searchUserLogins(query UserSearchQuery) ([]string, int, error) {
	var logins []string
	seen := map[string]bool{}
	totalUsersCount := 0
	perPage := 100
	maxPages := (query.MaxUsers + perPage - 1) / perPage

	retryCount := 0
	maxRetryCount := 10

	for page := 1; page <= maxPages; page++ {
		q := query.Q
		if query.MinFollowers > 0 {
			q = fmt.Sprintf("%s followers:>=%d", q, query.MinFollowers)
		}
		requestURL := fmt.Sprintf(
			"%ssearch/users?q=%s&sort=%s&order=%s&per_page=%d&page=%d",
			root, url.QueryEscape(q), query.Sort, query.Order, perPage, page,
		)

		body, err := client.Request(requestURL, "")
		if err != nil {
			retryCount++
			if retryCount < maxRetryCount {
				log.Println("error making REST search request... retrying")
				time.Sleep(10 * time.Second)
				page--
				continue
			}
			return nil, 0, fmt.Errorf("too many REST search errors: %w", err)
		}

		var result restSearchResponse
		if err := json.Unmarshal(body, &result); err != nil {
			retryCount++
			if retryCount < maxRetryCount {
				log.Println("error unmarshalling REST search JSON... retrying")
				time.Sleep(10 * time.Second)
				page--
				continue
			}
			return nil, 0, fmt.Errorf("too many REST search JSON errors: %w", err)
		}

		if result.Message != "" {
			retryCount++
			if retryCount < maxRetryCount {
				log.Printf("REST search API error (retrying): %s", result.Message)
				time.Sleep(10 * time.Second)
				page--
				continue
			}
			return nil, 0, fmt.Errorf("too many REST search API errors: %s", result.Message)
		}

		retryCount = 0
		totalUsersCount = result.TotalCount

		for _, item := range result.Items {
			if isExcluded(item.Login) {
				continue
			}
			if !seen[item.Login] {
				seen[item.Login] = true
				logins = append(logins, item.Login)
			}
		}

		if len(result.Items) < perPage {
			break
		}

		// Avoid secondary rate limit
		time.Sleep(2 * time.Second)

		if len(logins) >= query.MaxUsers {
			logins = logins[:query.MaxUsers]
			break
		}
	}

	return logins, totalUsersCount, nil
}

// fetchUserDetails uses GraphQL user(login:) batch queries to get
// contributions, organizations, and follower counts.
func (client HTTPGithubClient) fetchUserDetails(logins []string) ([]User, error) {
	var users []User
	maxRetryCount := 5

	for i := 0; i < len(logins); i += usersPerGraphQLBatch {
		end := i + usersPerGraphQLBatch
		if end > len(logins) {
			end = len(logins)
		}
		batch := logins[i:end]
		batchNum := i/usersPerGraphQLBatch + 1

		dataNode, err := client.fetchGraphQLBatch(batch, batchNum, maxRetryCount)
		if err != nil {
			return nil, err
		}

		for _, login := range batch {
			key := "u_" + sanitizeLogin(login)
			userNode, ok := dataNode[key]
			if !ok || userNode == nil {
				log.Printf("user %s not found in GraphQL response, skipping", login)
				continue
			}

			user, err := parseUserNode(userNode.(map[string]interface{}))
			if err != nil {
				log.Printf("error parsing user %s, skipping: %v", login, err)
				continue
			}
			users = append(users, user)
		}

		// Avoid secondary rate limit between batches
		if i+usersPerGraphQLBatch < len(logins) {
			time.Sleep(2 * time.Second)
		}
	}

	return users, nil
}

// fetchGraphQLBatch fetches a single batch of users via GraphQL with retries and exponential backoff.
func (client HTTPGithubClient) fetchGraphQLBatch(batch []string, batchNum int, maxRetryCount int) (map[string]interface{}, error) {
	graphQlString := buildBatchUserQuery(batch)

	for retryCount := 0; ; retryCount++ {
		body, err := client.Request("https://api.github.com/graphql", graphQlString)
		if err != nil {
			if retryCount >= maxRetryCount {
				return nil, fmt.Errorf("batch %d: too many request errors: %w", batchNum, err)
			}
			log.Printf("error making graphql request (batch %d)... retrying", batchNum)
			time.Sleep(time.Duration(retryBackoff(retryCount)) * time.Second)
			continue
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			if retryCount >= maxRetryCount {
				return nil, fmt.Errorf("batch %d: too many JSON parse errors: %w", batchNum, err)
			}
			log.Printf("error unmarshalling JSON response (batch %d)... retrying", batchNum)
			time.Sleep(time.Duration(retryBackoff(retryCount)) * time.Second)
			continue
		}

		if val, ok := response["errors"]; ok {
			if retryCount >= maxRetryCount {
				return nil, fmt.Errorf("batch %d: too many GraphQL errors: %+v", batchNum, val)
			}
			log.Printf("Received error response (batch %d, retrying): %+v", batchNum, val)
			time.Sleep(time.Duration(retryBackoff(retryCount)) * time.Second)
			continue
		}

		dataNode, ok := response["data"].(map[string]interface{})
		if !ok {
			if retryCount >= maxRetryCount {
				return nil, fmt.Errorf("batch %d: too many missing data node errors", batchNum)
			}
			log.Printf("error accessing data element (batch %d)... retrying", batchNum)
			time.Sleep(time.Duration(retryBackoff(retryCount)) * time.Second)
			continue
		}

		return dataNode, nil
	}
}

func retryBackoff(retryCount int) int {
	wait := 10 * (1 << retryCount) // 10, 20, 40, 80, 120...
	if wait > 120 {
		wait = 120
	}
	return wait
}

func isExcluded(login string) bool {
	for _, excluded := range excludeLogins {
		if excluded == login {
			return true
		}
	}
	return false
}

func buildBatchUserQuery(logins []string) string {
	var parts []string
	for _, login := range logins {
		key := sanitizeLogin(login)
		parts = append(parts, fmt.Sprintf(`u_%s: user(login: \"%s\") {
			login
			avatarUrl
			name
			company
			organizations(first: 100) { nodes { login } }
			followers { totalCount }
			contributionsCollection {
				contributionCalendar { totalContributions }
				totalCommitContributions
				totalPullRequestContributions
				restrictedContributionsCount
			}
		}`, key, login))
	}

	queryBody := strings.Join(parts, "\n")
	graphQlString := fmt.Sprintf(`{ "query": "{ %s }" }`, queryBody)

	re := regexp.MustCompile(`\r?\n`)
	graphQlString = re.ReplaceAllString(graphQlString, " ")
	re2 := regexp.MustCompile(`\t+`)
	graphQlString = re2.ReplaceAllString(graphQlString, " ")

	return graphQlString
}

func sanitizeLogin(login string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return re.ReplaceAllString(login, "_")
}

func parseUserNode(userNode map[string]interface{}) (User, error) {
	login := userNode["login"].(string)
	avatarURL := strPropOrEmpty(userNode, "avatarUrl")
	name := strPropOrEmpty(userNode, "name")
	company := strPropOrEmpty(userNode, "company")

	var organizations []string
	if orgData, ok := userNode["organizations"].(map[string]interface{}); ok {
		if nodes, ok := orgData["nodes"].([]interface{}); ok {
			for _, orgNode := range nodes {
				if orgMap, ok := orgNode.(map[string]interface{}); ok {
					organizations = append(organizations, orgMap["login"].(string))
				}
			}
		}
	}

	followerCount := int(userNode["followers"].(map[string]interface{})["totalCount"].(float64))
	contributionsCollection := userNode["contributionsCollection"].(map[string]interface{})
	contributionCount := int(contributionsCollection["contributionCalendar"].(map[string]interface{})["totalContributions"].(float64))
	privateContributionCount := int(contributionsCollection["restrictedContributionsCount"].(float64))
	commitsCount := int(contributionsCollection["totalCommitContributions"].(float64))
	pullRequestsCount := int(contributionsCollection["totalPullRequestContributions"].(float64))

	return User{
		Login:                    login,
		AvatarURL:                avatarURL,
		Name:                     name,
		Company:                  company,
		Organizations:            organizations,
		FollowerCount:            followerCount,
		ContributionCount:        contributionCount,
		PublicContributionCount:  contributionCount - privateContributionCount,
		PrivateContributionCount: privateContributionCount,
		CommitsCount:             commitsCount,
		PullRequestsCount:        pullRequestsCount,
	}, nil
}

func minFollowers(users []User) int {
	if len(users) == 0 {
		return 0
	}
	min := math.MaxInt32
	for _, user := range users {
		if user.FollowerCount < min {
			min = user.FollowerCount
		}
	}
	return min
}

func strPropOrEmpty(obj map[string]interface{}, prop string) string {
	switch t := obj[prop].(type) {
	case string:
		return t
	default:
		return ""
	}
}

func (client HTTPGithubClient) Organizations(login string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/orgs", login)
	body, err := client.Request(url, "")
	if err != nil {
		log.Fatalf("error requesting organizations for user %+v", login)
		return []string{}, err
	}
	orgResp := []OrgResponse{}
	err = json.Unmarshal(body, &orgResp)
	if err != nil {
		log.Fatalf("error parsing organizations JSON for user %+v", login)
		return []string{}, err
	}
	var orgs []string
	for _, org := range orgResp {
		orgs = append(orgs, org.Organization)
	}
	return orgs, err
}

type OrgResponse struct {
	Organization string `json:"login"`
}

func NewGithubClient(wrappers ...net.Wrapper) HTTPGithubClient {
	return HTTPGithubClient{wrappers: wrappers}
}

type User struct {
	Login                    string
	AvatarURL                string
	Name                     string
	Company                  string
	Organizations            []string
	FollowerCount            int
	ContributionCount        int
	PublicContributionCount  int
	PrivateContributionCount int
	CommitsCount             int
	PullRequestsCount        int
}

type UserSearchQuery struct {
	Q            string
	Sort         string
	Order        string
	MaxUsers     int
	MinFollowers int
}

type GithubSearchResults struct {
	Users                []User
	MinimumFollowerCount int
	TotalUserCount       int
}

type restSearchResponse struct {
	TotalCount        int              `json:"total_count"`
	IncompleteResults bool             `json:"incomplete_results"`
	Items             []restSearchItem `json:"items"`
	Message           string           `json:"message"`
}

type restSearchItem struct {
	Login string `json:"login"`
}
