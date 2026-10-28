package github

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

const (
	maxPerQuery    = 1000 // GitHub search returns at most 1000 results per query
	defaultPerPage = 5
	maxRetryCount  = 10 // consecutive failed requests before giving up
	skipAfter      = 3  // failures of a one-user page before that user is skipped
	maxSkipped     = 10 // users skipped before the whole search is treated as broken
)

// retryDelay is a variable so tests can run without sleeping.
var retryDelay = 10 * time.Second

const userFields = `
  login
  avatarUrl
  name
  company
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
    }
    totalCommitContributions
    restrictedContributionsCount
  }`

// skipFields loads only what is needed to step past a user whose full data
// GitHub cannot return.
const skipFields = `
  login
  followers {
    totalCount
  }`

// searchPage runs one page of the user search and returns its edges.
func (client HTTPGithubClient) searchPage(q string, first int, after string, fields string) ([]interface{}, int, error) {
	afterArg := ""
	if after != "" {
		afterArg = fmt.Sprintf(`, after: "%s"`, after)
	}
	graphQl := fmt.Sprintf(`query {
  search(type: USER, query: "%s", first: %d%s) {
    userCount
    edges {
      cursor
      node {
        __typename
        ... on User {%s
        }
      }
    }
  }
}`, q, first, afterArg, fields)
	request, err := json.Marshal(map[string]string{"query": graphQl})
	if err != nil {
		return nil, 0, err
	}

	body, err := client.Request("https://api.github.com/graphql", string(request))
	if err != nil {
		return nil, 0, fmt.Errorf("error making graphql request: %v", err)
	}

	var response struct {
		Data *struct {
			Search *struct {
				UserCount int
				Edges     []interface{}
			}
		}
		Errors []interface{}
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, 0, fmt.Errorf("error unmarshalling JSON response: %v", err)
	}
	if len(response.Errors) > 0 {
		return nil, 0, fmt.Errorf("received error response: %+v", response.Errors)
	}
	if response.Data == nil || response.Data.Search == nil {
		return nil, 0, fmt.Errorf("error accessing data element: %s", body)
	}
	return response.Data.Search.Edges, response.Data.Search.UserCount, nil
}

func (client HTTPGithubClient) SearchUsers(query UserSearchQuery) (GithubSearchResults, error) {
	users := []User{}
	userLogins := map[string]bool{}

	totalCount := 0
	minFollowerCount := -1
	totalUsersCount := 0

	retryCount := 0
	skipped := []string{}

Pages:
	for totalCount < query.MaxUsers {
		previousCursor := ""
		followerCountQueryStr := ""
		if minFollowerCount >= 0 {
			followerCountQueryStr = fmt.Sprintf(" followers:<%d", minFollowerCount)
		}
		q := fmt.Sprintf("%s%s sort:%s-%s", query.Q, followerCountQueryStr, query.Sort, query.Order)

		perPage := defaultPerPage
		pageFailures := 0
		for consumed := 0; consumed < maxPerQuery; {
			edgeNodes, userCount, err := client.searchPage(q, perPage, previousCursor, userFields)
			if err != nil {
				retryCount++
				if retryCount >= maxRetryCount {
					log.Println(err)
					log.Fatalln("Too many errors received. Quitting.")
				}
				log.Printf("%v (retrying)", err)
				time.Sleep(retryDelay)

				// A page keeps failing when one user's data is too costly for
				// GitHub to return. Retry one user at a time to find that user,
				// then step past them so the rest of the location still loads.
				if perPage > 1 {
					perPage = 1
					continue
				}
				pageFailures++
				if pageFailures < skipAfter {
					continue
				}
				skipEdges, _, skipErr := client.searchPage(q, 1, previousCursor, skipFields)
				if skipErr != nil {
					continue // the search itself is failing, not one user
				}
				if len(skipEdges) == 0 {
					break Pages
				}
				edgeNode := skipEdges[0].(map[string]interface{})
				userNode := edgeNode["node"].(map[string]interface{})
				login := strPropOrEmpty(userNode, "login")
				skipped = append(skipped, login)
				if len(skipped) > maxSkipped {
					log.Fatalf("Skipped too many users %v. Quitting.", skipped)
				}
				log.Printf("Skipping %s: GitHub could not return this user's data", login)
				if followers, ok := userNode["followers"].(map[string]interface{}); ok {
					minFollowerCount = int(followers["totalCount"].(float64))
				}
				previousCursor = edgeNode["cursor"].(string)
				consumed++
				totalCount++
				retryCount = 0
				pageFailures = 0
				perPage = defaultPerPage
				continue
			}
			retryCount = 0
			pageFailures = 0
			perPage = defaultPerPage
			totalUsersCount = userCount

			if len(edgeNodes) == 0 {
				break Pages
			}
			consumed += len(edgeNodes)
			totalCount += len(edgeNodes)

		Edges:
			for _, edge := range edgeNodes {
				edgeNode := edge.(map[string]interface{})
				previousCursor = edgeNode["cursor"].(string)
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

				user := User{
					Login:                    login,
					AvatarURL:                avatarURL,
					Name:                     name,
					Company:                  company,
					Organizations:            organizations,
					FollowerCount:            followerCount,
					ContributionCount:        contributionCount,
					PublicContributionCount:  (contributionCount - privateContributionCount),
					PrivateContributionCount: privateContributionCount,
					CommitsCount:             commitsCount}

				if !userLogins[login] {
					userLogins[login] = true
					users = append(users, user)
				}

				minFollowerCount = int(followerCount)
			}
		}
	}

	return GithubSearchResults{
		Users:                users,
		MinimumFollowerCount: minFollowerCount,
		TotalUserCount:       totalUsersCount}, nil
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
