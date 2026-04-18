package top

import (
	"errors"
	"fmt"

	"most-active-github-users-counter/github"
	"most-active-github-users-counter/net"
)

// FilterBlacklistedUsers removes any users who are marked as known graph abusers.
func FilterBlacklistedUsers(results github.GithubSearchResults) github.GithubSearchResults {
	filtered := make([]github.User, 0, len(results.Users))
	for _, u := range results.Users {
		if IsBlacklisted(u.Login) {
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

	// Filter out explicitly blacklisted abusive accounts.
	users = FilterBlacklistedUsers(users)

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

