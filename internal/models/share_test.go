package models

import (
	"database/sql"
	"testing"
)

func TestPlayLimitReached(t *testing.T) {
	limit := func(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: true} }
	cases := []struct {
		name  string
		share Share
		want  bool
	}{
		{"no limit", Share{TotalPlays: 100}, false},
		{"plays left", Share{TotalPlays: 2, MaxTotalPlays: limit(3)}, false},
		{"last play used", Share{TotalPlays: 3, MaxTotalPlays: limit(3)}, true},
		{"over the limit", Share{TotalPlays: 4, MaxTotalPlays: limit(3)}, true},
	}
	for _, c := range cases {
		if got := c.share.PlayLimitReached(); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHasEpisodes(t *testing.T) {
	for typ, want := range map[string]bool{"Season": true, "Series": true, "Movie": false, "Episode": false} {
		if got := (&Share{ItemType: typ}).HasEpisodes(); got != want {
			t.Errorf("%s: got %v, want %v", typ, got, want)
		}
	}
}
