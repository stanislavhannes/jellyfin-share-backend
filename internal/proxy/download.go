package proxy

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/download"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/jellyfin"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/models"
)

// ServeDownload streams what a download ticket names. The ticket was issued
// after the share's limits were checked and the play was charged; this only has
// to verify it, and that the share has not been revoked or expired since.
func (p *StreamProxy) ServeDownload(w http.ResponseWriter, r *http.Request) {
	ticket, err := p.downloads.Verify(chi.URLParam(r, "ticket"))
	if err != nil {
		http.Error(w, "this download link has expired", http.StatusForbidden)
		return
	}
	share, err := p.db.GetShareByID(r.Context(), ticket.ShareID)
	if err != nil || share == nil || !share.IsValid() {
		http.Error(w, "share not available", http.StatusForbidden)
		return
	}
	// Checked again, not just at issue: the switch may have gone off since.
	if !share.DownloadsAllowed(p.cfg.AllowDownloads) {
		http.Error(w, "downloads are not available for this link", http.StatusForbidden)
		return
	}

	if ticket.ItemID == download.AllEpisodes {
		p.serveEpisodesZip(w, r, share)
		return
	}
	p.serveFile(w, r, ticket.ItemID)
}

// serveFile passes the original file through, Range included, so a browser can
// resume an interrupted download.
func (p *StreamProxy) serveFile(w http.ResponseWriter, r *http.Request, itemID string) {
	item, err := p.jf.GetItem(r.Context(), itemID)
	if err != nil || item == nil {
		log.Printf("Failed to load item %s for download: %v", itemID, err)
		http.Error(w, "file not available", http.StatusBadGateway)
		return
	}

	resp, err := p.openOriginal(r.Context(), item, r.Header)
	if err != nil {
		log.Printf("Failed to open %s for download: %v", itemID, err)
		http.Error(w, "file not available", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 416 is the browser's own business: it asked to resume past the end, and
	// needs the Content-Range that says how long the file really is.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent &&
		resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		log.Printf("Jellyfin refused download of %s: %d", itemID, resp.StatusCode)
		http.Error(w, "file not available", http.StatusBadGateway)
		return
	}

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "ETag"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.Header().Set("Content-Disposition", download.ContentDisposition(download.FileName(item)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// serveEpisodesZip writes every episode into one ZIP as it streams. The files
// are stored, not deflated: video is already compressed, and deflating it would
// cost CPU for nothing. Writing as it goes means no temporary copy on disk, at the
// price of a response without a length - the browser cannot show progress as a
// percentage, nor resume it.
func (p *StreamProxy) serveEpisodesZip(w http.ResponseWriter, r *http.Request, share *models.Share) {
	ctx := r.Context()
	itemID := share.JellyfinItemID
	listed, err := p.jf.GetEpisodeFilesFor(ctx, share.ItemType, itemID)
	if err != nil {
		log.Printf("Failed to list episodes of %s for download: %v", itemID, err)
		http.Error(w, "episodes not available", http.StatusBadGateway)
		return
	}
	// A library that shows missing episodes lists them with no file behind
	// them. They are left out up front, while an error can still be sent: a
	// season of nothing but missing episodes must not become an empty archive.
	episodes := listed[:0]
	for _, ep := range listed {
		if len(ep.MediaSources) > 0 {
			episodes = append(episodes, ep)
		}
	}
	if len(episodes) == 0 {
		log.Printf("ZIP download of %s: none of its %d episodes has a file", itemID, len(listed))
		http.Error(w, "episodes not available", http.StatusBadGateway)
		return
	}
	if skipped := len(listed) - len(episodes); skipped > 0 {
		log.Printf("ZIP download of %s: leaving out %d episodes without a file", itemID, skipped)
	}

	folder := download.Sanitize(share.Title)
	bySeason := share.ItemType == "Series"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", download.ContentDisposition(folder+".zip"))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)

	zw := zip.NewWriter(w)
	for i := range episodes {
		ep := &episodes[i]
		if ctx.Err() != nil {
			return // the viewer cancelled; nothing is listening any more
		}
		if err := p.addEpisode(ctx, zw, folder, bySeason, ep); err != nil {
			// The status line has gone out, so there is no error to send. Stopping
			// without the central directory leaves a ZIP the viewer's tools reject,
			// rather than one that silently lacks an episode.
			log.Printf("Aborted ZIP download of %s at episode %s: %v", itemID, ep.ID, err)
			return
		}
	}
	if err := zw.Close(); err != nil {
		log.Printf("Failed to finish ZIP download of %s: %v", itemID, err)
	}
}

func (p *StreamProxy) addEpisode(ctx context.Context, zw *zip.Writer, folder string, bySeason bool, item *jellyfin.ItemInfo) error {
	resp, err := p.openOriginal(ctx, item, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jellyfin returned %d", resp.StatusCode)
	}

	// A series is filed by season, the way the library itself would be.
	name := folder + "/"
	if bySeason {
		name += fmt.Sprintf("Season %02d/", item.ParentIndexNumber)
	}
	name += download.FileName(item)

	f, err := zw.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Store,
		Modified: time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	return err
}

// openOriginal requests the source file. From the viewer's request it forwards
// only what resuming needs: Range, and If-Range so a file replaced since the
// first part was saved is sent whole rather than spliced onto the old one.
func (p *StreamProxy) openOriginal(ctx context.Context, item *jellyfin.ItemInfo, viewer http.Header) (*http.Response, error) {
	if len(item.MediaSources) == 0 {
		return nil, fmt.Errorf("item %s has no media source", item.ID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.jf.GetOriginalFileURL(item.ID, item.MediaSources[0].ID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", p.jf.AuthHeader())
	for _, h := range []string{"Range", "If-Range"} {
		if v := viewer.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	return p.httpClient.Do(req)
}
