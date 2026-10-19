package models

import "time"

// MainItemKey is what the share page calls the item a link points at directly -
// the film - as opposed to the episodes inside a season or series.
const MainItemKey = "main"

// ItemProgress is where a link's viewers got to in one item. It belongs to the
// link, not to a browser: whoever holds the link picks up where it was left,
// on any device.
type ItemProgress struct {
	ItemID       string    `db:"item_id"`
	PositionSecs int64     `db:"position_secs"`
	DurationSecs int64     `db:"duration_secs"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type ProgressEntry struct {
	Position int64 `json:"position"`
	Duration int64 `json:"duration"`
}

// ShareProgress is the record the share page reads: the item watched last, and
// a position for every item watched at all.
type ShareProgress struct {
	Last  string                   `json:"last"`
	Items map[string]ProgressEntry `json:"items"`
}

// NewShareProgress builds the page's record from stored rows, newest first. The
// share's own item is keyed MainItemKey, episodes by their id.
func NewShareProgress(shareItemID string, rows []ItemProgress) ShareProgress {
	key := func(itemID string) string {
		if itemID == shareItemID {
			return MainItemKey
		}
		return itemID
	}
	p := ShareProgress{Items: make(map[string]ProgressEntry, len(rows))}
	for i, r := range rows {
		if i == 0 {
			p.Last = key(r.ItemID)
		}
		p.Items[key(r.ItemID)] = ProgressEntry{Position: r.PositionSecs, Duration: r.DurationSecs}
	}
	return p
}

// MinResumeSecs is where a position starts to count. Under it the viewer has
// barely started - web/src/progress.js uses the same floor before it offers to
// resume - and a report that low must not replace a real one.
const MinResumeSecs = 10

// ReplacesProgress says whether a new position should take the place of the
// stored one. A brief start - a mis-tap, a play from the top that was stopped
// at once - would otherwise wipe where everyone holding the link had got to,
// or that they had watched it to the end. Past the floor, the latest report
// wins as usual, so starting over really does start over. database.SaveProgress
// applies the same rule inside its upsert; change both together.
func ReplacesProgress(stored *ItemProgress, newPos int64) bool {
	return stored == nil || newPos >= MinResumeSecs || stored.PositionSecs < MinResumeSecs
}

// maxDurationSecs bounds what a page may claim an item lasts. The value comes
// from the client; a week is longer than anything a library holds, and keeps
// the number inside the database's INTEGER columns.
const maxDurationSecs = 7 * 24 * 3600

// MayRecordProgress says whether a session may still move its link's progress:
// it is running, it belongs to this link, the link can still be watched, and
// the session is pinned to an item. A session ended by its viewer, by the
// reaper or by revocation writes nothing, whatever it sends afterwards.
func MayRecordProgress(session *ShareSession, share *Share) bool {
	return session != nil && share != nil &&
		!session.FinishedAt.Valid &&
		session.ShareID == share.ID &&
		share.IsValid() &&
		session.JellyfinItemID.Valid && session.JellyfinItemID.String != ""
}

// ReportedProgress checks what a heartbeat or finish call reported. Without a
// known duration nothing can be said about how much was seen, so it is not
// stored; a position outside the item is pulled back inside it.
func ReportedProgress(position, duration *int64) (pos, dur int64, ok bool) {
	if position == nil || duration == nil || *duration <= 0 || *duration > maxDurationSecs {
		return 0, 0, false
	}
	return min(max(*position, 0), *duration), *duration, true
}
