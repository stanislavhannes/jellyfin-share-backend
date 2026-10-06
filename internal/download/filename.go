package download

import (
	"fmt"
	"mime"
	"path"
	"strings"

	"github.com/jellyfin-share/jellyfin-share-backend/internal/jellyfin"
)

// FileName names a downloaded item the way a media library would file it:
// "Film (2020).mkv", or "Show - S01E02 - Title.mkv" for an episode.
func FileName(item *jellyfin.ItemInfo) string {
	var base string
	switch {
	case item.Type == "Episode" && item.SeriesName != "":
		base = fmt.Sprintf("%s - S%02dE%02d - %s",
			item.SeriesName, item.ParentIndexNumber, item.IndexNumber, item.Name)
	case item.ProductionYear > 0 && !strings.HasSuffix(item.Name, fmt.Sprintf("(%d)", item.ProductionYear)):
		// Libraries named from folders often carry the year in the title already.
		base = fmt.Sprintf("%s (%d)", item.Name, item.ProductionYear)
	default:
		base = item.Name
	}
	return Sanitize(base) + extension(item)
}

// extension is the source file's own extension, which is what the bytes are.
// The container name is the fallback; Jellyfin reports some as a list
// ("mov,mp4,m4a"), of which the first entry is the useful one.
func extension(item *jellyfin.ItemInfo) string {
	if len(item.MediaSources) == 0 {
		return ""
	}
	ms := item.MediaSources[0]
	// The host may be Windows, so both separators have to count.
	if ext := path.Ext(strings.ReplaceAll(ms.Path, `\`, "/")); ext != "" {
		return strings.ToLower(ext)
	}
	if c, _, _ := strings.Cut(ms.Container, ","); c != "" {
		return "." + strings.ToLower(c)
	}
	return ""
}

// Sanitize makes a title safe as a file name on every OS the viewer might save
// it to.
func Sanitize(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r < 0x20, r == 0x7f:
			return -1
		case strings.ContainsRune(`/\:*?"<>|`, r):
			return '-'
		}
		return r
	}, name)
	name = strings.Trim(strings.TrimSpace(name), ".")
	if name == "" {
		return "download"
	}
	return name
}

// ContentDisposition marks a response as an attachment under the given name. A
// non-ASCII name is encoded as RFC 2231 filename*, which every current browser
// reads.
func ContentDisposition(name string) string {
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": name}); v != "" {
		return v
	}
	return "attachment"
}
