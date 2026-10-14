package github

import (
	"fmt"
	"io/ioutil"
	"net/http"
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
