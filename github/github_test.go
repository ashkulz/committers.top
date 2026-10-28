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

var (
	firstArg     = regexp.MustCompile(`first: (\d+)`)
	afterArg     = regexp.MustCompile(`after: "(\d+)"`)
	followersArg = regexp.MustCompile(`followers:<(\d+)`)
)

// fakeSearch serves the GraphQL user search from memory. It honours
// followers:<N, first, after and the 1000-result cap of GitHub search.
type fakeSearch struct {
	followers []int  // one entry per user, sorted descending
	poison    string // login whose full data never loads
	failEvery int    // when > 0, every failEvery-th request returns a non-JSON body
	requests  int
}

func login(i int) string { return fmt.Sprintf("user%d", i) }

func (f *fakeSearch) wrapper() net.Wrapper {
	return func(net.Requester) net.Requester {
		return func(req *http.Request) ([]byte, error) {
			f.requests++
			if f.failEvery > 0 && f.requests%f.failEvery == 0 {
				return []byte("<html>502 Bad Gateway</html>"), nil
			}
			raw, _ := ioutil.ReadAll(req.Body)
			var body struct{ Query string }
			if err := json.Unmarshal(raw, &body); err != nil {
				return nil, err
			}
			return f.respond(body.Query), nil
		}
	}
}

func (f *fakeSearch) respond(q string) []byte {
	first, _ := strconv.Atoi(firstArg.FindStringSubmatch(q)[1])
	offset := 0
	if m := afterArg.FindStringSubmatch(q); m != nil {
		offset, _ = strconv.Atoi(m[1])
		offset++
	}
	limit := -1
	if m := followersArg.FindStringSubmatch(q); m != nil {
		limit, _ = strconv.Atoi(m[1])
	}
	full := strings.Contains(q, "contributionsCollection")

	matching := []int{}
	for i, followers := range f.followers {
		if limit < 0 || followers < limit {
			matching = append(matching, i)
		}
	}
	if len(matching) > maxPerQuery {
		matching = matching[:maxPerQuery]
	}

	edges := []interface{}{}
	for pos := offset; pos < offset+first && pos < len(matching); pos++ {
		i := matching[pos]
		if full && login(i) == f.poison {
			out, _ := json.Marshal(map[string]interface{}{
				"data":   nil,
				"errors": []interface{}{map[string]interface{}{"type": "RESOURCE_LIMITS_EXCEEDED"}},
			})
			return out
		}
		node := map[string]interface{}{
			"__typename": "User",
			"login":      login(i),
			"followers":  map[string]interface{}{"totalCount": f.followers[i]},
		}
		if full {
			node["avatarUrl"] = "https://example.com/" + login(i)
			node["organizations"] = map[string]interface{}{"nodes": []interface{}{}}
			node["contributionsCollection"] = map[string]interface{}{
				"contributionCalendar":         map[string]interface{}{"totalContributions": 10},
				"totalCommitContributions":     5,
				"restrictedContributionsCount": 1,
			}
		}
		edges = append(edges, map[string]interface{}{"cursor": strconv.Itoa(pos), "node": node})
	}
	out, _ := json.Marshal(map[string]interface{}{
		"data": map[string]interface{}{
			"search": map[string]interface{}{"userCount": len(f.followers), "edges": edges},
		},
	})
	return out
}

func newFake(users int) *fakeSearch {
	followers := make([]int, users)
	for i := range followers {
		followers[i] = 10 * (users - i)
	}
	return &fakeSearch{followers: followers}
}

func search(t *testing.T, f *fakeSearch, maxUsers int) []string {
	t.Helper()
	retryDelay = 0
	client := NewGithubClient(f.wrapper())
	results, err := client.SearchUsers(UserSearchQuery{Q: "type:user location:test", Sort: "followers", Order: "desc", MaxUsers: maxUsers})
	if err != nil {
		t.Fatal(err)
	}
	logins := []string{}
	for _, user := range results.Users {
		logins = append(logins, user.Login)
	}
	return logins
}

func expected(users int, except string) []string {
	logins := []string{}
	for i := 0; i < users; i++ {
		if login(i) != except {
			logins = append(logins, login(i))
		}
	}
	return logins
}

func assertLogins(t *testing.T, got []string, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %d users %v\nwant %d users %v", len(got), got, len(want), want)
	}
}

func TestSearchUsersReturnsEveryUser(t *testing.T) {
	assertLogins(t, search(t, newFake(23), 23), expected(23, ""))
}

// One user whose data GitHub can never return used to fail every retry, and so
// the whole location (issue #145). That user is now skipped.
func TestSearchUsersSkipsUserWhoseDataNeverLoads(t *testing.T) {
	for _, poison := range []string{"user0", "user7", "user12", "user22"} {
		f := newFake(23)
		f.poison = poison
		assertLogins(t, search(t, f, 23), expected(23, poison))
	}
}

// Past the 1000-result cap the search restarts with followers:<N; a skipped
// user must not end it early or drop anyone after them.
func TestSearchUsersSkipsAcrossTheResultCap(t *testing.T) {
	f := newFake(1100)
	f.poison = "user1003"
	assertLogins(t, search(t, f, 1100), expected(1100, "user1003"))
}

// Errors spread across a long search no longer add up to "Too many errors";
// only consecutive errors do. Nobody is skipped for a passing error.
func TestSearchUsersRecoversFromScatteredErrors(t *testing.T) {
	f := newFake(200)
	f.failEvery = 7
	assertLogins(t, search(t, f, 200), expected(200, ""))
}
