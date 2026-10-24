package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"most-active-github-users-counter/net"
)

const errorResponse = `{"errors":[{"type":"%s","message":"error"}]}`

// fakeResponses returns a wrapper that answers requests with the given bodies in order
// (never calling the real requester) and records each request body.
func fakeResponses(responses []string, requests *[]string) net.Wrapper {
	return func(net.Requester) net.Requester {
		return func(req *http.Request) ([]byte, error) {
			body, _ := ioutil.ReadAll(req.Body)
			*requests = append(*requests, string(body))
			if len(*requests) > len(responses) {
				return []byte(`{"data":{"search":{"userCount":0,"edges":[]}}}`), nil
			}
			return []byte(responses[len(*requests)-1]), nil
		}
	}
}

func usersPage(firstID, count int) string {
	edges := []string{}
	for id := firstID; id < firstID+count; id++ {
		edges = append(edges, fmt.Sprintf(`{"cursor":"c%d","node":{"__typename":"User","login":"user%d","avatarUrl":"","organizations":{"nodes":[]},"followers":{"totalCount":%d},"contributionsCollection":{"contributionCalendar":{"totalContributions":1},"totalCommitContributions":1,"totalPullRequestContributions":0,"restrictedContributionsCount":0}}}`, id, id, 1000-id))
	}
	return fmt.Sprintf(`{"data":{"search":{"userCount":100,"edges":[%s]}}}`, strings.Join(edges, ","))
}

func TestMain(m *testing.M) {
	retryDelay = 0
	m.Run()
}

func TestSearchUsersReducesPageSizeOnResourceLimits(t *testing.T) {
	requests := []string{}
	responses := []string{
		fmt.Sprintf(errorResponse, "RESOURCE_LIMITS_EXCEEDED"),
		usersPage(0, 2),
	}
	client := NewGithubClient(fakeResponses(responses, &requests))

	results, err := client.SearchUsers(UserSearchQuery{Q: "location:test", Sort: "followers", Order: "desc", MaxUsers: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(results.Users))
	}
	if !strings.Contains(requests[0], "first: 5") {
		t.Errorf("expected first request to ask for 5 users, got: %s", requests[0])
	}
	if !strings.Contains(requests[1], "first: 2") {
		t.Errorf("expected retry to ask for 2 users, got: %s", requests[1])
	}
}

func TestSearchUsersKeepsPageSizeOnOtherErrors(t *testing.T) {
	requests := []string{}
	responses := []string{
		fmt.Sprintf(errorResponse, "SOMETHING_ELSE"),
		usersPage(0, 5),
	}
	client := NewGithubClient(fakeResponses(responses, &requests))

	if _, err := client.SearchUsers(UserSearchQuery{Q: "location:test", Sort: "followers", Order: "desc", MaxUsers: 100}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requests[1], "first: 5") {
		t.Errorf("expected retry to keep asking for 5 users, got: %s", requests[1])
	}
}

func TestSearchUsersResetsRetriesAfterSuccessfulPage(t *testing.T) {
	// more errors in total than maxRetryCount, but never two in a row: this used to abort the whole search
	requests := []string{}
	responses := []string{}
	for i := 0; i < 15; i++ {
		responses = append(responses, fmt.Sprintf(errorResponse, "SOMETHING_ELSE"), usersPage(i*5, 5))
	}
	client := NewGithubClient(fakeResponses(responses, &requests))

	results, err := client.SearchUsers(UserSearchQuery{Q: "location:test", Sort: "followers", Order: "desc", MaxUsers: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Users) != 75 {
		t.Fatalf("expected 75 users, got %d", len(results.Users))
	}
}

const resourceLimitsResponse = `{"data":null,"errors":[{"type":"RESOURCE_LIMITS_EXCEEDED","message":"Resource limits for this query exceeded."}]}`
const timeoutResponse = `{"data":null,"errors":[{"message":"Something went wrong while executing your query. This may be the result of a timeout, or it could be a GitHub bug."}]}`

var (
	firstPattern     = regexp.MustCompile(`first: (\d+)`)
	afterPattern     = regexp.MustCompile(`after: \\"c(\d+)\\"`)
	followersPattern = regexp.MustCompile(`followers:<(\d+)`)
)

// fakeSearchAPI simulates GitHub's user search over a fixed list of users sorted by followers. It
// honours followers:<N, first and after, stops at 1000 results like GitHub does, and can make the
// full-data query fail for chosen users or return them as null nodes.
type fakeSearchAPI struct {
	logins       []string // sorted by followers, descending
	failing      map[string]*fakeFailure
	nullNodes    map[string]bool
	requests     []string
	furthestSeen int // furthest result position any request asked for within its search
}

type fakeFailure struct {
	response  string
	remaining int // failures left before the user's data loads; negative means it never loads
}

func newFakeSearchAPI(userCount int) *fakeSearchAPI {
	api := &fakeSearchAPI{failing: map[string]*fakeFailure{}, nullNodes: map[string]bool{}}
	for i := 0; i < userCount; i++ {
		api.logins = append(api.logins, fmt.Sprintf("user%d", i))
	}
	return api
}

// fakeFollowers is the follower count of the user at index i: distinct and descending.
func fakeFollowers(i int) int {
	return 100000 - i
}

func (api *fakeSearchAPI) wrapper() net.Wrapper {
	return func(net.Requester) net.Requester {
		return func(req *http.Request) ([]byte, error) {
			body, _ := ioutil.ReadAll(req.Body)
			api.requests = append(api.requests, string(body))
			return []byte(api.respond(string(body))), nil
		}
	}
}

func (api *fakeSearchAPI) respond(request string) string {
	followerLimit := -1
	if m := followersPattern.FindStringSubmatch(request); m != nil {
		followerLimit, _ = strconv.Atoi(m[1])
	}
	matches := []int{}
	for i := range api.logins {
		if followerLimit < 0 || fakeFollowers(i) < followerLimit {
			matches = append(matches, i)
		}
	}

	first, _ := strconv.Atoi(firstPattern.FindStringSubmatch(request)[1])
	start := 0
	if m := afterPattern.FindStringSubmatch(request); m != nil {
		after, _ := strconv.Atoi(m[1])
		start = after + 1
	}
	end := start + first
	if end > api.furthestSeen {
		api.furthestSeen = end
	}
	if end > 1000 {
		end = 1000
	}
	if end > len(matches) {
		end = len(matches)
	}

	fullData := strings.Contains(request, "contributionsCollection")
	edges := []string{}
	for position := start; position < end; position++ {
		i := matches[position]
		login := api.logins[i]
		if failure, ok := api.failing[login]; ok && fullData && failure.remaining != 0 {
			if failure.remaining > 0 {
				failure.remaining--
			}
			return failure.response
		}
		node := "null"
		if !api.nullNodes[login] {
			if fullData {
				node = fmt.Sprintf(`{"__typename":"User","login":"%s","avatarUrl":"","organizations":{"nodes":[]},"followers":{"totalCount":%d},"contributionsCollection":{"contributionCalendar":{"totalContributions":1},"totalCommitContributions":1,"totalPullRequestContributions":0,"restrictedContributionsCount":0}}`, login, fakeFollowers(i))
			} else {
				node = fmt.Sprintf(`{"__typename":"User","login":"%s","followers":{"totalCount":%d}}`, login, fakeFollowers(i))
			}
		}
		edges = append(edges, fmt.Sprintf(`{"cursor":"c%d","node":%s}`, position, node))
	}
	return fmt.Sprintf(`{"data":{"search":{"userCount":%d,"edges":[%s]}}}`, len(api.logins), strings.Join(edges, ","))
}

// searchFakeAPI runs a search against api and returns the logins found.
func searchFakeAPI(t *testing.T, api *fakeSearchAPI, maxUsers int) map[string]bool {
	t.Helper()
	results, err := NewGithubClient(api.wrapper()).SearchUsers(UserSearchQuery{Q: "type:user location:test", Sort: "followers", Order: "desc", MaxUsers: maxUsers})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, user := range results.Users {
		found[user.Login] = true
	}
	return found
}

func TestSearchUsersSkipsUserWhoseDataNeverLoads(t *testing.T) {
	// one user whose data GitHub can never return used to abort the search for the whole location
	for name, response := range map[string]string{"resource limits": resourceLimitsResponse, "timeout": timeoutResponse} {
		t.Run(name, func(t *testing.T) {
			api := newFakeSearchAPI(30)
			api.failing["user3"] = &fakeFailure{response: response, remaining: -1}

			found := searchFakeAPI(t, api, 1000)

			if found["user3"] {
				t.Error("expected user3 to be skipped")
			}
			if len(found) != 29 {
				t.Errorf("expected the other 29 users, got %d", len(found))
			}
		})
	}
}

func TestSearchUsersKeepsUserAfterTransientFailures(t *testing.T) {
	api := newFakeSearchAPI(30)
	api.failing["user3"] = &fakeFailure{response: timeoutResponse, remaining: 2}

	found := searchFakeAPI(t, api, 1000)

	if len(found) != 30 {
		t.Errorf("expected all 30 users, got %d", len(found))
	}
	for _, request := range api.requests {
		if !strings.Contains(request, "contributionsCollection") {
			t.Errorf("expected no result to be skipped, but saw a query to identify one: %s", request)
		}
	}
}

func TestSearchUsersSkipsNullNodes(t *testing.T) {
	api := newFakeSearchAPI(30)
	api.nullNodes["user7"] = true

	found := searchFakeAPI(t, api, 1000)

	if found["user7"] || len(found) != 29 {
		t.Errorf("expected the 29 users other than user7, got %d users", len(found))
	}
}

func TestSearchUsersRetriesMalformedResponse(t *testing.T) {
	// a missing search node used to panic
	requests := []string{}
	responses := []string{`{"data":{"search":null}}`, usersPage(0, 5)}
	client := NewGithubClient(fakeResponses(responses, &requests))

	results, err := client.SearchUsers(UserSearchQuery{Q: "location:test", Sort: "followers", Order: "desc", MaxUsers: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Users) != 5 {
		t.Fatalf("expected 5 users, got %d", len(results.Users))
	}
}

func TestSearchUsersStaysWithinSearchResultLimit(t *testing.T) {
	// skipping a result changes the page size partway through a search; GitHub returns nothing past
	// result 1000, so requesting beyond it would end the search early
	api := newFakeSearchAPI(1200)
	api.failing["user3"] = &fakeFailure{response: resourceLimitsResponse, remaining: -1}

	found := searchFakeAPI(t, api, 1000)

	if api.furthestSeen > 1000 {
		t.Errorf("requested results up to position %d, past GitHub's limit of 1000", api.furthestSeen)
	}
	for i := 0; i < 1000; i++ {
		if login := fmt.Sprintf("user%d", i); i != 3 && !found[login] {
			t.Errorf("expected %s in results", login)
		}
	}
}

func TestSearchUsersAdvancesPastNonUserResults(t *testing.T) {
	requests := []string{}
	organization := `{"cursor":"c4","node":{"__typename":"Organization"}}`
	page := strings.TrimSuffix(usersPage(0, 4), "]}}}") + "," + organization + "]}}}"
	client := NewGithubClient(fakeResponses([]string{page}, &requests))

	if _, err := client.SearchUsers(UserSearchQuery{Q: "location:test", Sort: "followers", Order: "desc", MaxUsers: 100}); err != nil {
		t.Fatal(err)
	}
	if len(requests) < 2 || !strings.Contains(requests[1], `after: \"c4\"`) {
		t.Errorf("expected the next page to start after the organization, got: %v", requests)
	}
}

func TestUserSearchRequestIsValidJSON(t *testing.T) {
	for _, fields := range []string{userFields, identityFields} {
		request := userSearchRequest("type:user location:test followers:<10 sort:followers-desc", 5, "Y3Vyc29yOjU=", fields)
		var body map[string]string
		if err := json.Unmarshal([]byte(request), &body); err != nil {
			t.Fatalf("request is not valid JSON: %v\n%s", err, request)
		}
		if !strings.Contains(body["query"], `query:"type:user location:test followers:<10 sort:followers-desc", first: 5, after: "Y3Vyc29yOjU="`) {
			t.Errorf("unexpected GraphQL query: %s", body["query"])
		}
	}
}
