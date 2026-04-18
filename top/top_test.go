package top

import (
	"testing"

	"most-active-github-users-counter/github"
)

// ---------------------------------------------------------------------------
// locationMatchesThreshold tests
// ---------------------------------------------------------------------------

func TestLocationMatchesThreshold_WorldwidePassthrough(t *testing.T) {
	// Empty keyword list means worldwide query — always pass.
	if !locationMatchesThreshold("uk mx germany", []string{}, 2) {
		t.Fatal("worldwide query should always match")
	}
}

func TestLocationMatchesThreshold_ExactFirstToken(t *testing.T) {
	// "uk" is the very first token — should match.
	if !locationMatchesThreshold("uk mx ksa germany france", []string{"uk", "england"}, 2) {
		t.Fatal("'uk' in first token should match UK keywords")
	}
}

func TestLocationMatchesThreshold_ExactSecondToken(t *testing.T) {
	// "mx" is the second token — should still pass with threshold=2.
	if !locationMatchesThreshold("uk mx ksa germany france", []string{"mx", "mexico"}, 2) {
		t.Fatal("'mx' in second token should pass within threshold=2")
	}
}

func TestLocationMatchesThreshold_ThirdTokenExcluded(t *testing.T) {
	// "ksa" is the third token — should NOT match with threshold=2.
	if locationMatchesThreshold("uk mx ksa germany france", []string{"ksa", "saudi", "riyadh"}, 2) {
		t.Fatal("'ksa' beyond threshold=2 should NOT match")
	}
}

func TestLocationMatchesThreshold_CaseInsensitive(t *testing.T) {
	if !locationMatchesThreshold("UK, Egypt", []string{"uk"}, 2) {
		t.Fatal("match should be case-insensitive")
	}
}

func TestLocationMatchesThreshold_CityBeforeCountry(t *testing.T) {
	// A typical "City, Country" format — city is token 1, country is token 2.
	if !locationMatchesThreshold("Cairo, Egypt", []string{"egypt", "cairo"}, 2) {
		t.Fatal("'Cairo, Egypt' should match Egypt keywords")
	}
}

func TestLocationMatchesThreshold_MultiWordKeyword(t *testing.T) {
	// "hong+kong" keyword normalised to "hong kong" — should match.
	if !locationMatchesThreshold("Hong Kong", []string{"hong+kong"}, 2) {
		t.Fatal("multi-word keyword 'hong+kong' should match 'Hong Kong'")
	}
}

func TestLocationMatchesThreshold_MultiWordKeywordExcluded(t *testing.T) {
	// "hong kong" would need both tokens — ok within threshold 2.
	// But if some other city is in token 1 & 2 it should not match.
	if locationMatchesThreshold("Berlin, Germany", []string{"hong+kong"}, 2) {
		t.Fatal("'Berlin, Germany' should NOT match 'hong+kong'")
	}
}

func TestLocationMatchesThreshold_AbuserWithManyCountries(t *testing.T) {
	abuser := "uk mx ksa drc cog togo cuba peru pk mali oman usa rsa rok uae mk cod macau laos iraq qatar gabon kosovo haiti benin syria chile niger yemen ghana nepal sudan kenya japan china india egypt italy spain france russia ukraine germany norway sweden finland"
	// Only "uk" and "mx" are in first two tokens.
	cases := []struct {
		keywords []string
		want     bool
		label    string
	}{
		{[]string{"uk", "england"}, true, "UK should pass (token 1)"},
		{[]string{"mx", "mexico"}, true, "Mexico should pass (token 2)"},
		{[]string{"germany", "deutschland", "berlin"}, false, "Germany should fail (beyond threshold)"},
		{[]string{"india", "mumbai", "delhi"}, false, "India should fail (beyond threshold)"},
		{[]string{"france", "paris"}, false, "France should fail (beyond threshold)"},
		{[]string{"egypt", "cairo"}, false, "Egypt should fail (beyond threshold)"},
	}
	for _, c := range cases {
		got := locationMatchesThreshold(abuser, c.keywords, 2)
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.label, got, c.want)
		}
	}
}

func TestLocationMatchesThreshold_LondonUK(t *testing.T) {
	// Real location "London, UK" — city is token 1, "uk" is token 2.
	if !locationMatchesThreshold("London, UK", []string{"uk", "england", "london"}, 2) {
		t.Fatal("'London, UK' should match UK keywords")
	}
}

func TestLocationMatchesThreshold_ZeroThresholdDefaultsTo2(t *testing.T) {
	// threshold=0 → defaults to 2. Third token "ksa" should still fail.
	if locationMatchesThreshold("uk mx ksa", []string{"ksa"}, 0) {
		t.Fatal("threshold=0 defaults to 2; 'ksa' at position 3 should fail")
	}
}

// ---------------------------------------------------------------------------
// FilterByLocationThreshold tests
// ---------------------------------------------------------------------------

func makeResults(locations ...string) github.GithubSearchResults {
	users := make([]github.User, len(locations))
	for i, l := range locations {
		users[i] = github.User{Login: l, Location: l}
	}
	return github.GithubSearchResults{Users: users}
}

func TestFilterByLocationThreshold_ReducesResults(t *testing.T) {
	results := makeResults(
		"London, UK",
		"Berlin, Germany",
		"uk mx ksa germany france india",
	)
	keywords := []string{"uk", "england", "london"}
	out := FilterByLocationThreshold(results, keywords, 2)
	if len(out.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(out.Users))
	}
}
