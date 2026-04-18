package top

import (
	"errors"
	"fmt"

	"most-active-github-users-counter/github"
	"most-active-github-users-counter/net"
)

// IsLikelyFakeCommitter returns true when a user's GitHub activity pattern
// strongly suggests that their commit count is inflated by automation, scripts,
// or bulk-push workflows rather than genuine software development.
//
// The thresholds are intentionally conservative to avoid false-positives on
// extremely productive real developers:
//
//  1. >50 000 commits in a year — no human being ships code at 137 commits/day.
//  2. >5 000 commits with fewer than 10 pull requests — automated pushers
//     almost never open PRs because they're not collaborating with anyone.
//  3. >3 000 commits with <3 followers — bot/throwaway accounts that nobody
//     follows despite allegedly massive output.
func IsLikelyFakeCommitter(user github.User) bool {
	// Rule 1: physically impossible yearly commit rate.
	if user.CommitsCount > 50000 {
		return true
	}

	// Rule 2: high volume, nearly zero pull-request activity.
	if user.CommitsCount > 5000 && user.PullRequestsCount < 10 {
		return true
	}

	// Rule 3: high volume, essentially no followers.
	if user.CommitsCount > 3000 && user.FollowerCount < 3 {
		return true
	}

	return false
}

// FilterFakeCommitters removes users whose commit patterns indicate
// automated or bulk-generated activity.
func FilterFakeCommitters(results github.GithubSearchResults) github.GithubSearchResults {
	filtered := make([]github.User, 0, len(results.Users))
	for _, u := range results.Users {
		if IsLikelyFakeCommitter(u) {
			continue
		}
		filtered = append(filtered, u)
	}
	results.Users = filtered
	return results
}

func GithubTop(options Options) (github.GithubSearchResults, error) {
	var token = options.Token
	if token == "" {
		return github.GithubSearchResults{}, errors.New("Missing GITHUB token")
	}

	query := "type:user"
	for _, location := range options.Locations {
		query = fmt.Sprintf("%s location:%s", query, location)
	}

	for _, location := range options.ExcludeLocations {
		query = fmt.Sprintf("%s -location:%s", query, location)
	}

	var client = github.NewGithubClient(net.TokenAuth(token))
	users, err := client.SearchUsers(github.UserSearchQuery{Q: query, Sort: "followers", Order: "desc", MaxUsers: options.ConsiderNum})
	if err != nil {
		return github.GithubSearchResults{}, err
	}

	// Filter out developers manipulating the leaderboards with bot commits.
	users = FilterFakeCommitters(users)

	return users, nil
}

type Options struct {
	Token            string
	Locations        []string
	ExcludeLocations []string
	Amount           int
	ConsiderNum      int
	PresetTitle      string
	PresetChecksum   string
}

