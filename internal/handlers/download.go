package handlers

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/database"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/download"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/middleware"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/models"
)

// RequestDownload authorises a download of the shared film or episode itself.
func (h *PublicHandler) RequestDownload(w http.ResponseWriter, r *http.Request) {
	share, ok := h.downloadableShare(w, r)
	if !ok {
		return
	}
	if share.HasEpisodes() {
		writeError(w, http.StatusBadRequest, "pick an episode, or download them all")
		return
	}
	h.issueDownload(w, r, share, share.JellyfinItemID)
}

// RequestEpisodeDownload authorises a download of one episode of a season or
// series share. The episode is checked against the share here, once; the ticket
// then carries it, so the GET cannot be pointed anywhere else.
func (h *PublicHandler) RequestEpisodeDownload(w http.ResponseWriter, r *http.Request) {
	share, ok := h.downloadableShare(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "episodeId")
	episodes, err := h.episodesForShare(r.Context(), share)
	if err != nil {
		log.Printf("Failed to get episodes: %v", err)
		writeError(w, http.StatusBadGateway, "could not reach the media server")
		return
	}
	if !containsEpisode(episodes, episodeID) {
		writeError(w, http.StatusForbidden, "episode not part of this share")
		return
	}
	h.issueDownload(w, r, share, episodeID)
}

// RequestAllEpisodesDownload authorises one ZIP of every episode in the share.
// It costs a single play: it is one request for the whole share, which is what
// a play limit on a series link is counting.
func (h *PublicHandler) RequestAllEpisodesDownload(w http.ResponseWriter, r *http.Request) {
	share, ok := h.downloadableShare(w, r)
	if !ok {
		return
	}
	if !share.HasEpisodes() {
		writeError(w, http.StatusBadRequest, "this share does not contain episodes")
		return
	}
	h.issueDownload(w, r, share, download.AllEpisodes)
}

// downloadableShare is publicShare for a download, which the backend may have
// switched off altogether.
func (h *PublicHandler) downloadableShare(w http.ResponseWriter, r *http.Request) (*models.Share, bool) {
	if !h.cfg.AllowDownloads {
		writeError(w, http.StatusForbidden, "downloads are disabled")
		return nil, false
	}
	return h.publicShare(w, r)
}

// issueDownload charges the download against the play limit and hands back a
// ticket for it. Only the total is checked: a file being saved does not occupy a
// concurrent-viewer slot - nobody is watching it yet.
func (h *PublicHandler) issueDownload(w http.ResponseWriter, r *http.Request, share *models.Share, itemID string) {
	ipHash := middleware.GetIPHash(r.Context())
	if share.PlayLimitReached() {
		h.db.LogAuditEvent(r.Context(), database.AuditEventPlaybackDenied, &share.ID, nil, nil, &ipHash, map[string]interface{}{
			"reason": "limit_reached",
			"itemId": itemID,
			"kind":   "download",
		})
		writeError(w, http.StatusForbidden, "maximum plays reached")
		return
	}

	ticket, err := h.downloads.Issue(share.ID, itemID)
	if err != nil {
		log.Printf("Failed to issue download ticket: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to start download")
		return
	}

	if err := h.db.IncrementTotalPlays(r.Context(), share.ID); err != nil {
		log.Printf("Failed to increment play count: %v", err)
	}
	h.db.LogAuditEvent(r.Context(), database.AuditEventDownloadStarted, &share.ID, nil, nil, &ipHash, map[string]interface{}{
		"itemId": itemID,
	})

	writeJSON(w, http.StatusOK, models.DownloadResponse{
		DownloadURL: "/api/public/downloads/" + ticket,
	})
}
