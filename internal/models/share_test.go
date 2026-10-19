package models

import (
	"database/sql"
	"testing"
	"time"
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

func TestWithholdDetails(t *testing.T) {
	five := int64(5)
	info := SharePublicInfo{
		Title: "Season 1", SeriesName: "Harbour Lights", PosterURL: "/p", BackdropURL: "/b", LogoURL: "/l", ItemType: "Season",
		RequiresPassword: true, MaxTotalPlays: &five, TotalPlays: 2, AllowDownload: true,
		Overview: "plot", Tagline: "line", RuntimeSeconds: 60, Year: 2020, OfficialRating: "PG",
		CommunityRating: 7, CriticRating: 80, Genres: []string{"Drama"}, Studios: []string{"S"},
		Directors: []string{"D"}, Actors: []ActorInfo{{Name: "A"}}, VideoQuality: &VideoQualityInfo{},
		DownloadBytes: 1 << 30, AudioTracks: []AudioTrack{{Index: 1}}, SubtitleTracks: []SubtitleTrack{{Index: 2}},
	}
	info.WithholdDetails()

	if !info.Locked {
		t.Error("withheld info must say it is locked")
	}
	// What the link points at, and its terms, stay visible before the password.
	if info.Title != "Season 1" || info.SeriesName != "Harbour Lights" || info.PosterURL == "" || info.BackdropURL == "" || info.LogoURL == "" ||
		info.MaxTotalPlays == nil || info.TotalPlays != 2 || !info.RequiresPassword || !info.AllowDownload {
		t.Errorf("title, artwork and terms must survive: %+v", info)
	}
	// Everything else waits for the password.
	if info.Overview != "" || info.Tagline != "" || info.RuntimeSeconds != 0 || info.Year != 0 ||
		info.OfficialRating != "" || info.CommunityRating != 0 || info.CriticRating != 0 ||
		info.Genres != nil || info.Studios != nil || info.Directors != nil || info.Actors != nil ||
		info.VideoQuality != nil || info.DownloadBytes != 0 || info.AudioTracks != nil || info.SubtitleTracks != nil {
		t.Errorf("details leaked before the password: %+v", info)
	}
}

func TestGone(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	expiry := func(at time.Time) NullTime { return NullTime{sql.NullTime{Time: at, Valid: true}} }
	revoked := sql.NullTime{Time: past.Add(-time.Minute), Valid: true}
	cases := []struct {
		name       string
		share      Share
		wantReason string
		wantAt     time.Time
	}{
		{"live", Share{ExpiresAt: expiry(future)}, "", time.Time{}},
		{"never expires", Share{}, "", time.Time{}},
		{"expired", Share{ExpiresAt: expiry(past)}, "expired", past},
		{"revoked before it expired", Share{ExpiresAt: expiry(future), RevokedAt: revoked}, "revoked", revoked.Time},
		{"revoked and expired", Share{ExpiresAt: expiry(past), RevokedAt: revoked}, "revoked", revoked.Time},
	}
	for _, c := range cases {
		c.share.Title = "Harbour Lights"
		gone := c.share.Gone()
		if c.wantReason == "" {
			if gone != nil {
				t.Errorf("%s: a live link is not gone: %+v", c.name, gone)
			}
			continue
		}
		if gone == nil || gone.Reason != c.wantReason || !gone.EndedAt.Equal(c.wantAt) || gone.Title != "Harbour Lights" {
			t.Errorf("%s: got %+v, want %s at %v", c.name, gone, c.wantReason, c.wantAt)
		}
	}
}

func TestParsePage(t *testing.T) {
	at := "2026-10-18T15:04:05.123456Z"
	id := "8f0c6a4e-2d1b-4c7a-9e3f-5b6d7c8e9f00"
	cases := []struct {
		name                 string
		limit, before, after string
		wantLimit            int
		wantCursor           bool
	}{
		{"defaults", "", "", "", DefaultPageSize, false},
		{"size and cursor", "20", at, id, 20, true},
		{"nonsense size", "0", "", "", DefaultPageSize, false},
		{"size capped", "5000", "", "", MaxPageSize, false},
		{"unreadable", "abc", "yesterday", "nope", DefaultPageSize, false},
		{"half a cursor", "20", at, "", 20, false},
	}
	for _, c := range cases {
		limit, cursor := ParsePage(c.limit, c.before, c.after)
		if limit != c.wantLimit || (cursor != nil) != c.wantCursor {
			t.Errorf("%s: got (%d, %v), want (%d, cursor %v)", c.name, limit, cursor, c.wantLimit, c.wantCursor)
		}
	}
	if _, cursor := ParsePage("", at, id); cursor.ID.String() != id || cursor.CreatedAt.Format(time.RFC3339Nano) != at {
		t.Errorf("cursor did not survive the round trip: %+v", cursor)
	}
}

func TestHasEpisodes(t *testing.T) {
	for typ, want := range map[string]bool{"Season": true, "Series": true, "Movie": false, "Episode": false} {
		if got := (&Share{ItemType: typ}).HasEpisodes(); got != want {
			t.Errorf("%s: got %v, want %v", typ, got, want)
		}
	}
}
