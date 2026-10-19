package proxy

import (
	"net/url"
	"testing"
)

func TestImageSizing(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"maxWidth=320", "maxWidth=320"},
		{"maxWidth=99999&quality=500", "maxWidth=1920&quality=100"},
		{"maxHeight=-4&quality=abc&maxWidth=", ""},
		{"maxWidth=400&other=1", "maxWidth=400"},
	}
	for _, c := range cases {
		query, _ := url.ParseQuery(c.query)
		if got := imageSizing(query).Encode(); got != c.want {
			t.Errorf("%q: got %q, want %q", c.query, got, c.want)
		}
	}
}
