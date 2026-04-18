package top

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed blacklist.txt
var blacklistData string

var (
	blacklistedUsers map[string]struct{}
	blacklistOnce    sync.Once
)

func initBlacklist() {
	blacklistedUsers = make(map[string]struct{})
	lines := strings.Split(blacklistData, "\n")
	for _, line := range lines {
		user := strings.TrimSpace(line)
		if user != "" {
			blacklistedUsers[strings.ToLower(user)] = struct{}{}
		}
	}
}

// IsBlacklisted reports whether the given GitHub login is on the blacklist.
// The check is case-insensitive so casing variations can't bypass it.
func IsBlacklisted(login string) bool {
	blacklistOnce.Do(initBlacklist)
	_, ok := blacklistedUsers[strings.ToLower(login)]
	return ok
}
