package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewShareProgress(t *testing.T) {
	rows := []ItemProgress{ // newest first, as the query returns them
		{ItemID: "ep2", PositionSecs: 300, DurationSecs: 2700},
		{ItemID: "film", PositionSecs: 7000, DurationSecs: 7200},
	}
	p := NewShareProgress("film", rows)

	if p.Last != "ep2" {
		t.Errorf("last: got %q, want the newest row", p.Last)
	}
	// The share's own item answers to "main", which is what the page asks for.
	if got := p.Items[MainItemKey]; got != (ProgressEntry{Position: 7000, Duration: 7200}) {
		t.Errorf("main entry: got %+v", got)
	}
	if got := p.Items["ep2"]; got != (ProgressEntry{Position: 300, Duration: 2700}) {
		t.Errorf("episode entry: got %+v", got)
	}

	if empty := NewShareProgress("film", nil); empty.Last != "" || empty.Items == nil || len(empty.Items) != 0 {
		t.Errorf("no rows must give an empty record with a usable map: %+v", empty)
	}
	if p := NewShareProgress("film", rows[1:]); p.Last != MainItemKey {
		t.Errorf("last on the share's own item must read main, got %q", p.Last)
	}
}

func TestMayRecordProgress(t *testing.T) {
	share := &Share{ID: uuid.New()}
	pinned := sql.NullString{String: "film", Valid: true}
	live := &ShareSession{ShareID: share.ID, JellyfinItemID: pinned}
	revoked := &Share{ID: share.ID, RevokedAt: sql.NullTime{Time: time.Now(), Valid: true}}

	cases := []struct {
		name    string
		session *ShareSession
		share   *Share
		want    bool
	}{
		{"live session on a valid link", live, share, true},
		{"session already finished", &ShareSession{ShareID: share.ID, JellyfinItemID: pinned,
			FinishedAt: sql.NullTime{Time: time.Now(), Valid: true}}, share, false},
		{"link revoked", live, revoked, false},
		{"link gone", live, nil, false},
		{"session of another link", &ShareSession{ShareID: uuid.New(), JellyfinItemID: pinned}, share, false},
		{"session pinned to nothing", &ShareSession{ShareID: share.ID}, share, false},
		{"session pinned to an empty id", &ShareSession{ShareID: share.ID,
			JellyfinItemID: sql.NullString{Valid: true}}, share, false},
	}
	for _, c := range cases {
		if got := MayRecordProgress(c.session, c.share); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReplacesProgress(t *testing.T) {
	at := func(pos int64) *ItemProgress { return &ItemProgress{PositionSecs: pos, DurationSecs: 3600} }
	cases := []struct {
		name   string
		stored *ItemProgress
		newPos int64
		want   bool
	}{
		{"nothing stored yet", nil, 2, true},
		{"moving on", at(600), 700, true},
		{"going back past the floor", at(600), 30, true},
		{"a brief start from 0", at(600), 2, false},
		{"a brief start over a watched item", at(3600), 4, false},
		{"brief over brief", at(3), 6, true},
		{"exactly at the floor", at(600), MinResumeSecs, true},
	}
	for _, c := range cases {
		if got := ReplacesProgress(c.stored, c.newPos); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReportedProgress(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	cases := []struct {
		name               string
		position, duration *int64
		wantPos, wantDur   int64
		wantOK             bool
	}{
		{"normal", n(120), n(3600), 120, 3600, true},
		{"no duration yet", n(5), nil, 0, 0, false},
		{"zero duration", n(5), n(0), 0, 0, false},
		{"no position", nil, n(3600), 0, 0, false},
		{"past the end", n(4000), n(3600), 3600, 3600, true},
		{"negative", n(-3), n(3600), 0, 3600, true},
		{"longer than any item", n(5), n(1 << 40), 0, 0, false},
	}
	for _, c := range cases {
		pos, dur, ok := ReportedProgress(c.position, c.duration)
		if ok != c.wantOK || pos != c.wantPos || dur != c.wantDur {
			t.Errorf("%s: got (%d, %d, %v), want (%d, %d, %v)", c.name, pos, dur, ok, c.wantPos, c.wantDur, c.wantOK)
		}
	}
}
