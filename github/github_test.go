package github

import "testing"

func buildExcludeSet(repos ...string) map[string]bool {
	set := map[string]bool{}
	for _, r := range repos {
		set[r] = true
	}
	return set
}

func TestRepoExclusionsEmptySet(t *testing.T) {
	repos := []RepoCommitContribution{
		{NameWithOwner: "owner/repo", IsPrivate: false, Commits: 100},
	}
	pub, priv := repoExclusions(repos, map[string]bool{})
	if pub != 0 || priv != 0 {
		t.Fatalf("expected no exclusions, got public=%d private=%d", pub, priv)
	}
}

func TestRepoExclusionsPublicAndPrivate(t *testing.T) {
	repos := []RepoCommitContribution{
		{NameWithOwner: "domovinatv/dataset.domovina.tv", IsPrivate: false, Commits: 27604},
		{NameWithOwner: "owner/private-archive", IsPrivate: true, Commits: 500},
		{NameWithOwner: "owner/real-project", IsPrivate: false, Commits: 120},
	}
	exclude := buildExcludeSet("domovinatv/dataset.domovina.tv", "owner/private-archive")

	pub, priv := repoExclusions(repos, exclude)
	if pub != 27604 {
		t.Errorf("expected excludedPublic=27604, got %d", pub)
	}
	if priv != 500 {
		t.Errorf("expected excludedPrivate=500, got %d", priv)
	}
}

func TestRepoExclusionsCaseInsensitive(t *testing.T) {
	repos := []RepoCommitContribution{
		{NameWithOwner: "DomovinaTV/Dataset.Domovina.TV", IsPrivate: false, Commits: 42},
	}
	// The exclude set is lower-cased by the caller; repoExclusions lower-cases the
	// repository name before matching.
	exclude := buildExcludeSet("domovinatv/dataset.domovina.tv")

	pub, priv := repoExclusions(repos, exclude)
	if pub != 42 || priv != 0 {
		t.Errorf("expected case-insensitive match (public=42), got public=%d private=%d", pub, priv)
	}
}

func TestParseRepoCommitContributions(t *testing.T) {
	collection := map[string]interface{}{
		"commitContributionsByRepository": []interface{}{
			map[string]interface{}{
				"repository": map[string]interface{}{
					"nameWithOwner": "owner/repo",
					"isPrivate":     true,
				},
				"contributions": map[string]interface{}{
					"totalCount": float64(13),
				},
			},
			// malformed entry should be skipped, not panic
			map[string]interface{}{"unexpected": "shape"},
		},
	}

	parsed := parseRepoCommitContributions(collection)
	if len(parsed) != 1 {
		t.Fatalf("expected 1 parsed repo, got %d", len(parsed))
	}
	got := parsed[0]
	if got.NameWithOwner != "owner/repo" || !got.IsPrivate || got.Commits != 13 {
		t.Errorf("unexpected parsed contribution: %+v", got)
	}
}

func TestParseRepoCommitContributionsMissingKey(t *testing.T) {
	parsed := parseRepoCommitContributions(map[string]interface{}{})
	if len(parsed) != 0 {
		t.Fatalf("expected no contributions for missing key, got %d", len(parsed))
	}
}

func TestClampZero(t *testing.T) {
	if got := clampZero(-5); got != 0 {
		t.Errorf("expected clampZero(-5)=0, got %d", got)
	}
	if got := clampZero(7); got != 7 {
		t.Errorf("expected clampZero(7)=7, got %d", got)
	}
}
