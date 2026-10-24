package github

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"most-active-github-users-counter/net"
)

const root string = "https://api.github.com/"

// retryDelay is the pause before retrying a failed search page (a variable so tests can shorten it)
var retryDelay = 10 * time.Second

// skipAfterFailures is how many times in a row loading a single search result may fail before that
// result is skipped, so that one account GitHub can't return data for doesn't abort the whole search.
const skipAfterFailures = 3

// userFields are the fields fetched for each user in the search results.
const userFields = `__typename
              ... on User {
                login,
                avatarUrl,
                name,
                company,
                organizations(first: 100) {
                  nodes {
                    login
                  }
                }
                followers {
                  totalCount
                }
                contributionsCollection {
                  contributionCalendar {
                    totalContributions
                  },
                  totalCommitContributions,
                  totalPullRequestContributions,
                  restrictedContributionsCount
                }
              }`

// identityFields are just enough to identify a search result whose full data can't be loaded.
const identityFields = `__typename
              ... on User {
                login,
                followers {
                  totalCount
                }
              }`

var newlines = regexp.MustCompile(`\r?\n`)

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

func (client HTTPGithubClient) SearchUsers(query UserSearchQuery) (GithubSearchResults, error) {
	users := []User{}
	userLogins := map[string]bool{}

	totalCount := 0
	minFollowerCount := -1
	maxPerQuery := 1000
	defaultPerPage := 5
	perPage := defaultPerPage
	totalUsersCount := 0

	retryCount := 0
	maxRetryCount := 10
	singleResultFailures := 0

Pages:
	for totalCount < query.MaxUsers {
		previousCursor := ""
		followerCountQueryStr := ""
		if minFollowerCount >= 0 {
			followerCountQueryStr = fmt.Sprintf(" followers:<%d", minFollowerCount)
		}
		searchQuery := fmt.Sprintf("%s%s sort:%s-%s", query.Q, followerCountQueryStr, query.Sort, query.Order)

		// GitHub returns at most maxPerQuery results for a search; count the results consumed rather
		// than pages, because the page size can change while paging through them.
		for consumed := 0; consumed < maxPerQuery; {
			pageSize := perPage
			if remaining := maxPerQuery - consumed; pageSize > remaining {
				pageSize = remaining
			}

			body, err := client.Request("https://api.github.com/graphql", userSearchRequest(searchQuery, pageSize, previousCursor, userFields))
			page, graphQlErrors, failure := parseSearchPage(body, err)
			if failure != "" {
				retryCount++
				if pageSize == 1 {
					singleResultFailures++
				}
				if retryCount >= maxRetryCount {
					log.Println(failure)
					log.Fatalln("Too many errors received. Quitting.")
				}
				if pageSize == 1 && singleResultFailures >= skipAfterFailures {
					// the same single result keeps failing: skip it instead of giving up on the whole search
					if skipped, ok := client.searchResultAfter(searchQuery, previousCursor); ok {
						log.Printf("Skipping %s: its data could not be loaded after %d attempts (%s)", skipped.label(), singleResultFailures, failure)
						previousCursor = skipped.cursor
						if skipped.followers >= 0 {
							minFollowerCount = skipped.followers
						}
						totalCount++
						consumed++
						retryCount, singleResultFailures, perPage = 0, 0, defaultPerPage
						continue
					}
				} else if perPage > 1 && (onlyResourceLimitErrors(graphQlErrors) || retryCount >= 2) {
					perPage = perPage / 2
					log.Printf("Reducing page size to %d", perPage)
				}
				log.Printf("%s (retrying)", failure)
				time.Sleep(retryDelay)
				continue Pages
			}

			if page.userCount >= 0 {
				totalUsersCount = page.userCount
			}
			if len(page.edges) == 0 {
				break Pages
			}
			totalCount += len(page.edges)
			consumed += len(page.edges)

			for _, edge := range page.edges {
				edgeNode, _ := edge.(map[string]interface{})
				if cursor, ok := edgeNode["cursor"].(string); ok {
					previousCursor = cursor
				}
				userNode, _ := edgeNode["node"].(map[string]interface{})
				if userNode == nil {
					log.Println("Skipping a search result without data")
					continue
				}
				if typename, _ := userNode["__typename"].(string); typename != "User" {
					continue
				}
				user, ok := parseUser(userNode)
				if !ok {
					log.Printf("Skipping %q: incomplete data in search results", strPropOrEmpty(userNode, "login"))
					continue
				}

				if !userLogins[user.Login] {
					userLogins[user.Login] = true
					users = append(users, user)
				}
				minFollowerCount = user.FollowerCount
			}
			retryCount, singleResultFailures = 0, 0
		}
	}

	return GithubSearchResults{
		Users:                users,
		MinimumFollowerCount: minFollowerCount,
		TotalUserCount:       totalUsersCount}, nil
}

// userSearchRequest builds the GraphQL request for one page of a user search.
func userSearchRequest(searchQuery string, first int, cursor string, nodeFields string) string {
	cursorQueryStr := ""
	if cursor != "" {
		cursorQueryStr = fmt.Sprintf(`, after: \"%s\"`, cursor)
	}
	graphQlString := fmt.Sprintf(`{ "query": "query {
        search(type: USER, query:\"%s\", first: %d%s) {
          userCount
          edges {
            node {
              %s
            },
            cursor
          }
        }
      }" }`, searchQuery, first, cursorQueryStr, nodeFields)
	return newlines.ReplaceAllString(graphQlString, " ")
}

// searchPage is one page of search results.
type searchPage struct {
	userCount int // -1 if the response didn't include it
	edges     []interface{}
}

// parseSearchPage reads a page of search results from a GraphQL response. If the response can't be
// used it returns a description of the problem, along with GitHub's GraphQL errors if there were any.
func parseSearchPage(body []byte, requestErr error) (searchPage, interface{}, string) {
	if requestErr != nil {
		return searchPage{}, nil, fmt.Sprintf("error making graphql request: %v", requestErr)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return searchPage{}, nil, "error unmarshalling JSON response"
	}
	if graphQlErrors, ok := response["errors"]; ok {
		return searchPage{}, graphQlErrors, fmt.Sprintf("received error response: %+v", graphQlErrors)
	}
	dataNode, _ := response["data"].(map[string]interface{})
	searchNode, _ := dataNode["search"].(map[string]interface{})
	edges, ok := searchNode["edges"].([]interface{})
	if !ok {
		return searchPage{}, nil, "error accessing search results in response"
	}
	page := searchPage{userCount: -1, edges: edges}
	if userCount, ok := searchNode["userCount"].(float64); ok {
		page.userCount = int(userCount)
	}
	return page, nil, ""
}

// parseUser reads a User node from the search results; ok is false if expected fields are missing.
func parseUser(node map[string]interface{}) (User, bool) {
	login, _ := node["login"].(string)
	followerCount, hasFollowers := intAt(node, "followers", "totalCount")
	contributionCount, hasContributions := intAt(node, "contributionsCollection", "contributionCalendar", "totalContributions")
	privateContributionCount, hasPrivate := intAt(node, "contributionsCollection", "restrictedContributionsCount")
	commitsCount, hasCommits := intAt(node, "contributionsCollection", "totalCommitContributions")
	pullRequestsCount, hasPullRequests := intAt(node, "contributionsCollection", "totalPullRequestContributions")
	if login == "" || !hasFollowers || !hasContributions || !hasPrivate || !hasCommits || !hasPullRequests {
		return User{}, false
	}

	organizations := []string{}
	orgsNode, _ := node["organizations"].(map[string]interface{})
	orgNodes, _ := orgsNode["nodes"].([]interface{})
	for _, orgNode := range orgNodes {
		org, _ := orgNode.(map[string]interface{})
		if orgLogin, ok := org["login"].(string); ok {
			organizations = append(organizations, orgLogin)
		}
	}

	return User{
		Login:                    login,
		AvatarURL:                strPropOrEmpty(node, "avatarUrl"),
		Name:                     strPropOrEmpty(node, "name"),
		Company:                  strPropOrEmpty(node, "company"),
		Organizations:            organizations,
		FollowerCount:            followerCount,
		ContributionCount:        contributionCount,
		PublicContributionCount:  (contributionCount - privateContributionCount),
		PrivateContributionCount: privateContributionCount,
		CommitsCount:             commitsCount,
		PullRequestsCount:        pullRequestsCount}, true
}

// intAt follows path through nested JSON objects and returns the number found at its end.
func intAt(obj map[string]interface{}, path ...string) (int, bool) {
	var current interface{} = obj
	for _, key := range path {
		node, ok := current.(map[string]interface{})
		if !ok {
			return 0, false
		}
		current = node[key]
	}
	number, ok := current.(float64)
	return int(number), ok
}

// skippedResult identifies a search result that is skipped because its data can't be loaded.
type skippedResult struct {
	login     string
	followers int // -1 if unknown
	cursor    string
}

func (result skippedResult) label() string {
	if result.login == "" {
		return "a search result"
	}
	return result.login
}

// searchResultAfter identifies the search result right after cursor with a minimal query, which can
// still succeed when loading that result's full data keeps failing.
func (client HTTPGithubClient) searchResultAfter(searchQuery string, cursor string) (skippedResult, bool) {
	body, err := client.Request("https://api.github.com/graphql", userSearchRequest(searchQuery, 1, cursor, identityFields))
	page, _, failure := parseSearchPage(body, err)
	if failure != "" || len(page.edges) == 0 {
		return skippedResult{}, false
	}
	edgeNode, _ := page.edges[0].(map[string]interface{})
	resultCursor, ok := edgeNode["cursor"].(string)
	if !ok {
		return skippedResult{}, false
	}
	result := skippedResult{followers: -1, cursor: resultCursor}
	if userNode, ok := edgeNode["node"].(map[string]interface{}); ok {
		result.login = strPropOrEmpty(userNode, "login")
		if followers, ok := intAt(userNode, "followers", "totalCount"); ok {
			result.followers = followers
		}
	}
	return result, true
}

// onlyResourceLimitErrors reports whether every GraphQL error is RESOURCE_LIMITS_EXCEEDED,
// which GitHub returns when a page of results is too expensive to compute.
func onlyResourceLimitErrors(errs interface{}) bool {
	list, ok := errs.([]interface{})
	if !ok || len(list) == 0 {
		return false
	}
	for _, e := range list {
		errNode, ok := e.(map[string]interface{})
		if !ok || errNode["type"] != "RESOURCE_LIMITS_EXCEEDED" {
			return false
		}
	}
	return true
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
	orgs := []string{}

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
	Q        string
	Sort     string
	Order    string
	MaxUsers int
}

type GithubSearchResults struct {
	Users                []User
	MinimumFollowerCount int
	TotalUserCount       int
}
