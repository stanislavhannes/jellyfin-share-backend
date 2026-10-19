package proxy

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/jellyfin"
)

// episodeListTTL is how long a share's episode list answers still requests
// before Jellyfin is asked again. A page of stills arrives within seconds, so
// this turns one listing per still into one per page; an episode added to the
// season shows after a minute.
const episodeListTTL = time.Minute

// episodeCache holds each share's episode list briefly, for ServeEpisodeImage.
// The first screen of stills is requested all at once, so a list being fetched
// is shared too: the requests that arrive meanwhile wait for it rather than each
// listing the series again.
type episodeCache struct {
	mu      sync.Mutex
	now     func() time.Time
	byShare map[uuid.UUID]*episodeEntry
}

// episodeEntry is one share's list. done closes once episodes and err are set;
// at is when the list arrived, and stays zero until then.
type episodeEntry struct {
	done     chan struct{}
	episodes []jellyfin.EpisodeInfo
	err      error
	at       time.Time
}

func newEpisodeCache() *episodeCache {
	return &episodeCache{now: time.Now, byShare: map[uuid.UUID]*episodeEntry{}}
}

// get returns the share's list, calling load when there is none younger than
// episodeListTTL and none on its way. load runs for every caller waiting on it,
// so it must not depend on any one request's lifetime. A failed load is handed
// to its waiters and then forgotten. Stale entries are dropped whenever a load
// starts, so shares nobody looks at again do not stay in memory.
func (c *episodeCache) get(shareID uuid.UUID, load func() ([]jellyfin.EpisodeInfo, error)) ([]jellyfin.EpisodeInfo, error) {
	c.mu.Lock()
	now := c.now()
	if entry, ok := c.byShare[shareID]; ok && (entry.at.IsZero() || now.Sub(entry.at) < episodeListTTL) {
		c.mu.Unlock()
		<-entry.done
		return entry.episodes, entry.err
	}
	for id, e := range c.byShare {
		if !e.at.IsZero() && now.Sub(e.at) >= episodeListTTL {
			delete(c.byShare, id)
		}
	}
	entry := &episodeEntry{done: make(chan struct{})}
	c.byShare[shareID] = entry
	c.mu.Unlock()

	// Settled in a defer so that a panicking load still releases its waiters
	// with an error, rather than leaving every later still for the share
	// blocked on an entry that never finishes.
	entry.err = errLoadAbandoned
	defer func() {
		c.mu.Lock()
		if entry.err != nil {
			delete(c.byShare, shareID)
		} else {
			entry.at = c.now()
		}
		c.mu.Unlock()
		close(entry.done)
	}()
	entry.episodes, entry.err = load()
	return entry.episodes, entry.err
}

// errLoadAbandoned is what waiters get when the load they waited on panicked.
var errLoadAbandoned = errors.New("episode listing did not finish")
