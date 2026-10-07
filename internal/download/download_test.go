package download

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/jellyfin"
)

func TestTicketRoundTrip(t *testing.T) {
	s := NewSigner("secret")
	id := uuid.New()
	raw, err := s.Issue(id, "item-1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Verify(raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.ShareID != id || got.ItemID != "item-1" {
		t.Fatalf("got %+v", got)
	}
}

func TestTicketRejectsTamperingAndOtherKeys(t *testing.T) {
	s := NewSigner("secret")
	raw, _ := s.Issue(uuid.New(), "item-1")

	if _, err := NewSigner("other").Verify(raw); err == nil {
		t.Fatal("a ticket signed with another key must not verify")
	}

	// Swap the payload for one naming a different item, keeping the signature.
	other, _ := s.Issue(uuid.New(), AllEpisodes)
	body, _, _ := strings.Cut(other, ".")
	_, sig, _ := strings.Cut(raw, ".")
	if _, err := s.Verify(body + "." + sig); err == nil {
		t.Fatal("a payload with someone else's signature must not verify")
	}

	for _, bad := range []string{"", ".", "abc", raw + "x"} {
		if _, err := s.Verify(bad); err == nil {
			t.Fatalf("%q must not verify", bad)
		}
	}
}

func TestTicketExpires(t *testing.T) {
	s := NewSigner("secret")
	// A correctly signed ticket whose time is up: only the expiry can refuse it.
	payload, _ := json.Marshal(Ticket{ShareID: uuid.New(), ItemID: "item-1", ExpiresAt: time.Now().Add(-time.Minute).Unix()})
	body := base64.RawURLEncoding.EncodeToString(payload)
	raw := body + "." + base64.RawURLEncoding.EncodeToString(s.sign(body))
	if _, err := s.Verify(raw); err == nil {
		t.Fatal("an expired ticket must not verify")
	}
}

func TestFileName(t *testing.T) {
	episode := &jellyfin.ItemInfo{
		Type: "Episode", Name: "Pilot: Part 1", SeriesName: "Show",
		ParentIndexNumber: 1, IndexNumber: 2,
		MediaSources: []jellyfin.MediaSource{{Path: `D:\tv\Show\S01E02.MKV`, Container: "mkv"}},
	}
	if got := FileName(episode); got != "Show - S01E02 - Pilot- Part 1.mkv" {
		t.Errorf("episode: %q", got)
	}

	movie := &jellyfin.ItemInfo{
		Type: "Movie", Name: "Film", ProductionYear: 2020,
		MediaSources: []jellyfin.MediaSource{{Container: "mov,mp4,m4a"}},
	}
	if got := FileName(movie); got != "Film (2020).mov" {
		t.Errorf("movie: %q", got)
	}

	movie.Name = "Film (2020)"
	if got := FileName(movie); got != "Film (2020).mov" {
		t.Errorf("year already in the title: %q", got)
	}

	if got := Sanitize("../\x00.."); got != "-" {
		t.Errorf("sanitize: %q", got)
	}
}
