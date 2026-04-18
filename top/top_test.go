package top

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Blacklist tests
// ---------------------------------------------------------------------------

func TestIsBlacklisted_KnownAbuser(t *testing.T) {
	for _, name := range []string{
		"themiralay", "komutan234", "buraksocial",
		"Zer0-Bug", "wajahat-ali-mir-dev", "haroonrashidzadran",
	} {
		if !IsBlacklisted(name) {
			t.Errorf("'%s' should be blacklisted", name)
		}
	}
}

func TestIsBlacklisted_NormalUser(t *testing.T) {
	if IsBlacklisted("torvalds") {
		t.Fatal("torvalds should not be blacklisted")
	}
}

func TestIsBlacklisted_CaseInsensitive(t *testing.T) {
	if !IsBlacklisted("THEMIRALAY") {
		t.Fatal("blacklist check should be case-insensitive")
	}
}
