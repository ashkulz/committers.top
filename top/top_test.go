package top

import (
	"testing"

	"most-active-github-users-counter/github"
)

// ---------------------------------------------------------------------------
// IsLikelyFakeCommitter tests
// ---------------------------------------------------------------------------

func TestIsLikelyFakeCommitter_Normal(t *testing.T) {
	u := github.User{CommitsCount: 500, PullRequestsCount: 50, FollowerCount: 200}
	if IsLikelyFakeCommitter(u) {
		t.Fatal("normal developer should not be flagged")
	}
}

func TestIsLikelyFakeCommitter_TooManyCommits(t *testing.T) {
	u := github.User{CommitsCount: 80000, PullRequestsCount: 5, FollowerCount: 2}
	if !IsLikelyFakeCommitter(u) {
		t.Fatal("80k commits/year should be flagged as fake")
	}
}

func TestIsLikelyFakeCommitter_HighCommitsNoPRs(t *testing.T) {
	// 8000 commits, 0 PRs — automated push workflow.
	u := github.User{CommitsCount: 8000, PullRequestsCount: 0, FollowerCount: 50}
	if !IsLikelyFakeCommitter(u) {
		t.Fatal("high commits with zero PRs should be flagged")
	}
}

func TestIsLikelyFakeCommitter_HighCommitsNoFollowers(t *testing.T) {
	u := github.User{CommitsCount: 5000, PullRequestsCount: 3, FollowerCount: 1}
	if !IsLikelyFakeCommitter(u) {
		t.Fatal("high commits with near-zero followers should be flagged")
	}
}

func TestIsLikelyFakeCommitter_ActiveDeveloperNotFlagged(t *testing.T) {
	// Very active real developer: many commits, reasonable PRs, followers.
	u := github.User{CommitsCount: 3000, PullRequestsCount: 120, FollowerCount: 500}
	if IsLikelyFakeCommitter(u) {
		t.Fatal("active real developer should not be flagged")
	}
}
