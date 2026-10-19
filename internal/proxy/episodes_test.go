package proxy

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfin-share/jellyfin-share-backend/internal/jellyfin"
)

func TestEpisodeCache(t *testing.T) {
	clock := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	c := newEpisodeCache()
	c.now = func() time.Time { return clock }

	share := uuid.New()
	loads := 0
	load := func() ([]jellyfin.EpisodeInfo, error) {
		loads++
		return []jellyfin.EpisodeInfo{{ID: "ep1"}}, nil
	}

	for i := 0; i < 3; i++ {
		if _, err := c.get(share, load); err != nil {
			t.Fatal(err)
		}
	}
	if loads != 1 {
		t.Errorf("a page of stills should list the episodes once, listed %d times", loads)
	}

	clock = clock.Add(episodeListTTL)
	if _, err := c.get(share, load); err != nil {
		t.Fatal(err)
	}
	if loads != 2 {
		t.Errorf("a list older than the TTL should be fetched again, listed %d times", loads)
	}

	// A failure is passed on and not kept, so the next still tries again.
	other := uuid.New()
	if _, err := c.get(other, func() ([]jellyfin.EpisodeInfo, error) { return nil, errors.New("down") }); err == nil {
		t.Error("a failed listing must be reported")
	}
	if _, cached := c.byShare[other]; cached {
		t.Error("a failed listing must not be cached")
	}

	// Loading a list drops the ones that went stale.
	clock = clock.Add(episodeListTTL)
	if _, err := c.get(other, load); err != nil {
		t.Fatal(err)
	}
	if _, kept := c.byShare[share]; kept {
		t.Error("a stale share's list should be dropped")
	}
}

func TestEpisodeCacheSharesALoad(t *testing.T) {
	c := newEpisodeCache()
	share := uuid.New()

	var loads atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	load := func() ([]jellyfin.EpisodeInfo, error) {
		if loads.Add(1) == 1 {
			close(started)
		}
		<-release
		return []jellyfin.EpisodeInfo{{ID: "ep1"}}, nil
	}

	// The first screen of stills: one request starts the listing, the rest
	// arrive while it is still running.
	const stills = 8
	var wg sync.WaitGroup
	results := make(chan []jellyfin.EpisodeInfo, stills)
	get := func() {
		defer wg.Done()
		episodes, err := c.get(share, load)
		if err != nil {
			t.Error(err)
		}
		results <- episodes
	}
	wg.Add(1)
	go get()
	<-started
	for i := 1; i < stills; i++ {
		wg.Add(1)
		go get()
	}
	// Give the latecomers time to find the listing in flight before it ends.
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)

	if n := loads.Load(); n != 1 {
		t.Errorf("stills requested together should share one listing, listed %d times", n)
	}
	for episodes := range results {
		if len(episodes) != 1 || episodes[0].ID != "ep1" {
			t.Errorf("every waiter should get the list, got %+v", episodes)
		}
	}
}

func TestEpisodeCacheSurvivesAPanickingLoad(t *testing.T) {
	c := newEpisodeCache()
	share := uuid.New()

	func() {
		defer func() { recover() }()
		c.get(share, func() ([]jellyfin.EpisodeInfo, error) { panic("jellyfin client bug") })
	}()

	done := make(chan error, 1)
	go func() {
		_, err := c.get(share, func() ([]jellyfin.EpisodeInfo, error) { return []jellyfin.EpisodeInfo{{ID: "ep1"}}, nil })
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the next still should list afresh, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("a panicked load left the share's stills blocked")
	}
}
