package jellyfin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContainsEpisode(t *testing.T) {
	episodes := []EpisodeInfo{{ID: "a"}, {ID: "b"}}
	for id, want := range map[string]bool{"a": true, "b": true, "c": false, "": false} {
		if got := ContainsEpisode(episodes, id); got != want {
			t.Errorf("%q: got %v, want %v", id, got, want)
		}
	}
	if ContainsEpisode(nil, "a") {
		t.Error("an empty list contains nothing")
	}
}

// Jellyfin leaves an item's overview out of a list unless the request names
// it, so the episode list a viewer sees has to ask for it - and the id-only
// checks must not ask for anything.
func TestEpisodeQueriesAskOnlyForTheFieldsTheyNeed(t *testing.T) {
	var fields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Like Jellyfin, only the fields asked for come back.
		fields = r.URL.Query().Get("Fields")
		item := `{"Id":"e1","Name":"Earthfall","Type":"Episode"`
		if strings.Contains(fields, "Overview") {
			item += `,"Overview":"An object from space."`
		}
		if strings.Contains(fields, "MediaSources") {
			item += `,"MediaSources":[{"Size":42}]`
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Items":[` + item + `}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	c.userID = "u"
	episodes, err := c.GetEpisodeListFor(context.Background(), "Season", "s1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Overview", "MediaSources"} {
		if !strings.Contains(fields, want) {
			t.Errorf("Fields=%q should ask for %s", fields, want)
		}
	}
	if len(episodes) != 1 || episodes[0].Overview != "An object from space." || episodes[0].SizeBytes != 42 {
		t.Errorf("got %+v", episodes)
	}

	// The checks that only need the ids ask Jellyfin for no extra fields; media
	// sources alone cost it a lookup per episode.
	if _, err := c.GetEpisodesFor(context.Background(), "Season", "s1"); err != nil {
		t.Fatal(err)
	}
	if fields != "" {
		t.Errorf("GetEpisodesFor asked for Fields=%q, want none", fields)
	}
}
