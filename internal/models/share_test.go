package models

import (
	"database/sql"
	"testing"
)

func TestDownloadsAllowed(t *testing.T) {
	cases := []struct {
		share, server, want bool
	}{
		{share: true, server: true, want: true},
		{share: false, server: true, want: false}, // the sharer switched them off
		{share: true, server: false, want: false}, // the server switched them off for every link
		{share: false, server: false, want: false},
	}
	for _, c := range cases {
		s := &Share{AllowDownload: c.share}
		if got := s.DownloadsAllowed(c.server); got != c.want {
			t.Errorf("share=%v server=%v: got %v, want %v", c.share, c.server, got, c.want)
		}
	}
}

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
