package top

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"most-active-github-users-counter/github"
	"most-active-github-users-counter/net"
)

// locationSplitter splits a raw location string into individual tokens.
// It splits on whitespace, commas, semicolons, slashes, and pipes.
var locationSplitter = regexp.MustCompile(`[\s,;/|]+`)

// normalizeKeyword converts a preset search keyword (which may use "+" for
// spaces, e.g. "hong+kong") into a plain lowercase string for comparison.
func normalizeKeyword(kw string) string {
	return strings.ToLower(strings.ReplaceAll(kw, "+", " "))
}

// locationMatchesThreshold returns true if any of the first `threshold` tokens
// of the user's raw location string matches one of the provided location
// keywords (case-insensitive). When threshold is <= 0 it defaults to 2.
//
// The purpose is fairness: GitHub's location search is a simple substring
// match over the entire field, so a user who writes every country in their
// bio (e.g. "uk mx ksa … germany france") would appear in every leaderboard.
// By only inspecting the first N tokens we honour up to N legitimate
// locations (primary + one alternate) without penalising users who write a
// formatted address like "London, United Kingdom".
func locationMatchesThreshold(userLocation string, locationKeywords []string, threshold int) bool {
	if len(locationKeywords) == 0 {
		// Worldwide query — no location restriction.
		return true
	}
	if threshold <= 0 {
		threshold = 2
	}

	// Tokenise the user's raw location string.
	rawTokens := locationSplitter.Split(strings.TrimSpace(userLocation), -1)

	// Only consider the first `threshold` non-empty tokens.
	tokens := make([]string, 0, threshold)
	for _, t := range rawTokens {
		if t == "" {
			continue
		}
		tokens = append(tokens, strings.ToLower(t))
		if len(tokens) >= threshold {
			break
		}
	}

	// Join the accepted prefix once for multi-word and substring checks.
	prefix := strings.Join(tokens, " ")

	// Check each keyword against the prefix.
	for _, kw := range locationKeywords {
		normalised := normalizeKeyword(kw)

		// Use substring matching on the joined prefix, which handles:
		//   - Exact single-word matches:  "uk"  in ["uk"]
		//   - Abbreviation matches:       "rsa" in ["rsa"]
		//   - Multi-word phrase matches:  "hong kong" in ["hong", "kong"]
		//   - Phrase-within-token:        "united" ⊃ "united kingdom"? No.
		//     But "united kingdom" as prefix contains "united" via substring.
		//
		// We deliberately do NOT wildcard-expand (e.g. we don't match "uk"
		// inside "ukraine") because we compare whole tokens for single words.
		kwWords := strings.Fields(normalised)
		if len(kwWords) == 1 {
			// Single-word keyword: require exact token equality to avoid
			// "uk" matching inside "ukraine" etc.
			word := kwWords[0]
			for _, tok := range tokens {
				if tok == word {
					return true
				}
			}
			// Also allow substring match of the full prefix against the keyword
			// so that "United Kingdom" (prefix = "united kingdom") matches "uk"
			// via the joined representation — but only when the country writes
			// out the full name. We check the OTHER direction: does the joined
			// prefix *contain* the keyword as a word boundary?
			// Actually: to handle "United Kingdom" → "uk", we check if the
			// normalised keyword appears as a standalone token in the prefix.
			// The safe approach: check if prefix contains " " + word or starts
			// with word (already handled above via tok == word).
			// Additionally check the raw sub-string for known abbreviation cases
			// where the full name is written out (rare, covered by the preset
			// including both "uk" and "england" etc.).
			// Fall through to the substring check below for multi-char keywords.
			if len(word) > 2 && strings.Contains(prefix, word) {
				return true
			}
			continue
		}

		// Multi-word keyword (e.g. "hong kong"): the joined prefix must
		// contain the full phrase as a substring.
		if strings.Contains(prefix, normalised) {
			return true
		}
	}

	return false
}

// FilterByLocationThreshold removes users whose raw location field does not
// match any of the queried location keywords within the first `threshold` tokens.
func FilterByLocationThreshold(results github.GithubSearchResults, locations []string, threshold int) github.GithubSearchResults {
	if len(locations) == 0 {
		return results
	}
	filtered := make([]github.User, 0, len(results.Users))
	for _, u := range results.Users {
		if locationMatchesThreshold(u.Location, locations, threshold) {
			filtered = append(filtered, u)
		}
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

	// Apply fairness threshold: only keep users whose location starts with one
	// of the queried keywords (within the first LocationThreshold tokens).
	users = FilterByLocationThreshold(users, options.Locations, options.LocationThreshold)

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
	// LocationThreshold is the maximum number of tokens (words) to examine at
	// the start of a user's raw location string when deciding whether they
	// belong to a queried country/city. Defaults to 2 when <= 0, allowing a
	// primary location and one alternate (e.g. "London, UK" or "Cairo, Egypt").
	LocationThreshold int
}
