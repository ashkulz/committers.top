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

	// Repositories whose commit contributions should be ignored when ranking
	// users (e.g. dataset/archive repos that would otherwise inflate counts).
	// Matching is case-insensitive on the "owner/name" form.
	excludeRepos := map[string]bool{}
	for _, repo := range query.ExcludeRepos {
		excludeRepos[strings.ToLower(repo)] = true
	}

	totalCount := 0
	minFollowerCount := -1
	maxPerQuery := 1000
	perPage := 5
	totalUsersCount := 0

	retryCount := 0
	maxRetryCount := 10

Pages:
	for totalCount < query.MaxUsers {
		previousCursor := ""
		followerCountQueryStr := ""
		if minFollowerCount >= 0 {
			followerCountQueryStr = fmt.Sprintf(" followers:<%d", minFollowerCount)
		}
		for currentPage := 1; currentPage <= (maxPerQuery / perPage); currentPage++ {
			cursorQueryStr := ""
			if previousCursor != "" {
				cursorQueryStr = fmt.Sprintf(", after: \\\"%s\\\"", previousCursor)
			}
			graphQlString := fmt.Sprintf(`{ "query": "query {
        search(type: USER, query:\"%s%s sort:%s-%s\", first: %d%s) {
          userCount
          edges {
            node {
              __typename
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
                  restrictedContributionsCount,
                  commitContributionsByRepository(maxRepositories: 100) {
                    repository {
                      nameWithOwner,
                      isPrivate
                    },
                    contributions {
                      totalCount
                    }
                  }
                }
              }
            },
            cursor
          }
        }
      }" }`, query.Q, followerCountQueryStr, query.Sort, query.Order, perPage, cursorQueryStr)

			re := regexp.MustCompile(`\r?\n`)
			graphQlString = re.ReplaceAllString(graphQlString, " ")

			body, err := client.Request("https://api.github.com/graphql", graphQlString)
			if err != nil {
				retryCount++
				if retryCount < maxRetryCount {
					log.Println("error making graphql request... retrying")
					time.Sleep(10 * time.Second)
					continue Pages
				} else {
					log.Fatalln("Too many errors received. Quitting.")
				}
			}

			var response interface{}
			if err := json.Unmarshal(body, &response); err != nil {
				retryCount++
				if retryCount < maxRetryCount {
					log.Println("error unmarshalling JSON response... retrying")
					time.Sleep(10 * time.Second)
					continue Pages
				} else {
					log.Fatalln("Too many errors received. Quitting.")
				}
			}
			rootNode := response.(map[string]interface{})
			if val, ok := rootNode["errors"]; ok {
				retryCount++
				if retryCount < maxRetryCount {
					log.Printf("Received error response (retrying): %+v", val)
					time.Sleep(10 * time.Second)
					continue Pages
				} else {
					log.Fatalln("Too many errors received. Quitting.")
				}
			}
			dataNode, ok := rootNode["data"].(map[string]interface{})
			if !ok {
				retryCount++
				if retryCount < maxRetryCount {
					log.Println("Error accessing data element")
					time.Sleep(10 * time.Second)
					continue Pages
				} else {
					log.Fatalln("Too many errors received. Quitting.")
				}
			}

			searchNode := dataNode["search"].(map[string]interface{})
			totalUsersCount = int(searchNode["userCount"].(float64))
			edgeNodes := searchNode["edges"].([]interface{})

			if len(edgeNodes) == 0 {
				break Pages
			}
			totalCount += len(edgeNodes)

		Edges:
			for _, edge := range edgeNodes {
				edgeNode := edge.(map[string]interface{})
				userNode := edgeNode["node"].(map[string]interface{})
				typename := userNode["__typename"].(string)
				if typename != "User" {
					continue Edges
				}
				login := userNode["login"].(string)
				avatarURL := userNode["avatarUrl"].(string)
				name := strPropOrEmpty(userNode, "name")
				company := strPropOrEmpty(userNode, "company")
				organizations := []string{}

				orgNodes := userNode["organizations"].(map[string]interface{})["nodes"].([]interface{})
				for _, orgNode := range orgNodes {

					organizations = append(organizations, orgNode.(map[string]interface{})["login"].(string))
				}

				followerCount := int(userNode["followers"].(map[string]interface{})["totalCount"].(float64))
				contributionsCollection := userNode["contributionsCollection"].(map[string]interface{})
				contributionCount := int(contributionsCollection["contributionCalendar"].(map[string]interface{})["totalContributions"].(float64))
				privateContributionCount := int(contributionsCollection["restrictedContributionsCount"].(float64))
				commitsCount := int(contributionsCollection["totalCommitContributions"].(float64))
				pullRequestsCount := int(contributionsCollection["totalPullRequestContributions"].(float64))

				repoContributions := parseRepoCommitContributions(contributionsCollection)
				excludedPublic, excludedPrivate := repoExclusions(repoContributions, excludeRepos)
				excludedTotal := excludedPublic + excludedPrivate

				contributionCount = clampZero(contributionCount - excludedTotal)
				privateContributionCount = clampZero(privateContributionCount - excludedPrivate)
				commitsCount = clampZero(commitsCount - excludedTotal)

				user := User{
					Login:                     login,
					AvatarURL:                 avatarURL,
					Name:                      name,
					Company:                   company,
					Organizations:             organizations,
					FollowerCount:             followerCount,
					ContributionCount:         contributionCount,
					PublicContributionCount:   clampZero(contributionCount - privateContributionCount),
					PrivateContributionCount:  privateContributionCount,
					CommitsCount:              commitsCount,
					PullRequestsCount:         pullRequestsCount,
					ExcludedContributionCount: excludedTotal}

				if !userLogins[login] {
					userLogins[login] = true
					users = append(users, user)
				}

				previousCursor = edgeNode["cursor"].(string)
				minFollowerCount = int(followerCount)
			}
		}
	}

	return GithubSearchResults{
		Users:                users,
		MinimumFollowerCount: minFollowerCount,
		TotalUserCount:       totalUsersCount}, nil
}

// RepoCommitContribution holds the number of commit contributions a user made
// to a single repository within the queried time window.
type RepoCommitContribution struct {
	NameWithOwner string
	IsPrivate     bool
	Commits       int
}

// parseRepoCommitContributions extracts the per-repository commit breakdown from
// a parsed contributionsCollection node. Missing/malformed entries are skipped.
func parseRepoCommitContributions(contributionsCollection map[string]interface{}) []RepoCommitContribution {
	result := []RepoCommitContribution{}
	rawRepos, ok := contributionsCollection["commitContributionsByRepository"].([]interface{})
	if !ok {
		return result
	}
	for _, raw := range rawRepos {
		node, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		repo, ok := node["repository"].(map[string]interface{})
		if !ok {
			continue
		}
		nameWithOwner, _ := repo["nameWithOwner"].(string)
		isPrivate, _ := repo["isPrivate"].(bool)
		commits := 0
		if contribs, ok := node["contributions"].(map[string]interface{}); ok {
			if total, ok := contribs["totalCount"].(float64); ok {
				commits = int(total)
			}
		}
		result = append(result, RepoCommitContribution{
			NameWithOwner: nameWithOwner,
			IsPrivate:     isPrivate,
			Commits:       commits,
		})
	}
	return result
}

// repoExclusions sums the commit contributions belonging to excluded repositories,
// split by visibility so they can be subtracted from the right totals. The exclude
// set keys are expected to be lower-cased "owner/name" strings.
func repoExclusions(repos []RepoCommitContribution, exclude map[string]bool) (excludedPublic int, excludedPrivate int) {
	if len(exclude) == 0 {
		return 0, 0
	}
	for _, repo := range repos {
		if exclude[strings.ToLower(repo.NameWithOwner)] {
			if repo.IsPrivate {
				excludedPrivate += repo.Commits
			} else {
				excludedPublic += repo.Commits
			}
		}
	}
	return excludedPublic, excludedPrivate
}

func clampZero(n int) int {
	if n < 0 {
		return 0
	}
	return n
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
	Login                     string
	AvatarURL                 string
	Name                      string
	Company                   string
	Organizations             []string
	FollowerCount             int
	ContributionCount         int
	PublicContributionCount   int
	PrivateContributionCount  int
	CommitsCount              int
	PullRequestsCount         int
	ExcludedContributionCount int
}

type UserSearchQuery struct {
	Q            string
	Sort         string
	Order        string
	MaxUsers     int
	ExcludeRepos []string
}

type GithubSearchResults struct {
	Users                []User
	MinimumFollowerCount int
	TotalUserCount       int
}
