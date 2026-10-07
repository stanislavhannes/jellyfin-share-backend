package proxy

import (
	"testing"

	"github.com/jellyfin-share/jellyfin-share-backend/internal/jellyfin"
)

func TestWithFilesDropsMissingEpisodes(t *testing.T) {
	file := []jellyfin.MediaSource{{ID: "src"}}
	got := withFiles([]jellyfin.ItemInfo{
		{ID: "e1", MediaSources: file},
		{ID: "missing"}, // listed by the library, no file behind it
		{ID: "e3", MediaSources: file},
	})
	if len(got) != 2 || got[0].ID != "e1" || got[1].ID != "e3" {
		t.Fatalf("got %+v, want e1 and e3 in order", got)
	}
	if len(withFiles([]jellyfin.ItemInfo{{ID: "missing"}})) != 0 {
		t.Fatal("a season of missing episodes must leave nothing to archive")
	}
}
