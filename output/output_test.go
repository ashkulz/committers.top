package output

import (
	"bytes"
	"encoding/csv"
	"testing"

	"most-active-github-users-counter/github"
	"most-active-github-users-counter/top"
)

func TestCsvOutput_SortsByContributionsAndRespectsAmount(t *testing.T) {
	results := github.GithubSearchResults{
		Users: []github.User{
			{Login: "a", ContributionCount: 10, FollowerCount: 999},
			{Login: "b", ContributionCount: 20, FollowerCount: 1},
			{Login: "c", ContributionCount: 20, FollowerCount: 5},
		},
	}

	var buf bytes.Buffer
	if err := CsvOutput(results, &buf, top.Options{Amount: 2}); err != nil {
		t.Fatalf("CsvOutput error: %v", err)
	}

	r := csv.NewReader(bytes.NewReader(buf.Bytes()))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse csv: %v", err)
	}

	// header + 2 rows (because Amount=2)
	if got, want := len(rows), 3; got != want {
		t.Fatalf("unexpected number of rows: got %d want %d", got, want)
	}

	if got, want := rows[1][2], "c"; got != want {
		t.Fatalf("rank 1 login mismatch: got %q want %q", got, want)
	}
	if got, want := rows[2][2], "b"; got != want {
		t.Fatalf("rank 2 login mismatch: got %q want %q", got, want)
	}
}

