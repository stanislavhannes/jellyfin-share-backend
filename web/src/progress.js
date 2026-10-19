// Where the link's viewers left off. The server keeps it per link, not per
// browser, so a film started on a phone resumes on the laptop: the playing page
// reports its position with every heartbeat and once more when it stops, and
// casting reports the receiver's position the same way.
//
// One record per link: the item last watched, and a position per item. The item
// key is the episode id, or MAIN for a film.

export const MAIN = 'main';

// Under this, the viewer has barely started; offering to resume would only add
// a second button that does the same as the first.
const MIN_RESUME_SECONDS = 10;
// Past this share of the runtime the credits are rolling - the item counts as
// watched, and a series resumes on the next episode instead.
const WATCHED_RATIO = 0.92;

export const emptyProgress = () => ({ last: null, items: {} });

// The link's record from the server, or null when it could not be read. A
// failure is not "nothing watched": treating it so would start the next play
// at 0, and its heartbeats would overwrite where the link was really left.
export async function loadProgress(token) {
  // Where earlier versions kept it, in this browser. Nothing reads it now.
  try {
    localStorage.removeItem(`jfshare:progress:${token}`);
  } catch (e) {
    // Storage blocked - nothing was kept there either.
  }
  try {
    const response = await fetch(`/api/public/shares/${token}/progress`, { credentials: 'include' });
    if (response.ok) {
      const record = await response.json();
      if (record && typeof record === 'object' && record.items) {
        return { last: record.last || null, items: record.items };
      }
    }
  } catch (e) {
    // Offline or refused - fall through.
  }
  return null;
}

// The page's own copy, updated while it plays so the list and the Resume button
// are right the moment the player closes, without waiting for a reload. The
// server is told separately, by the player's heartbeats. Returns a new record
// so the caller stays reactive.
// A brief start does not replace a real position - the server keeps the same
// rule, so the page shows what everyone else will see.
export function noteProgress(current, itemKey, position, duration) {
  if (!itemKey || !(duration > 0)) return current;
  const stored = current.items[itemKey];
  if (stored && position < MIN_RESUME_SECONDS && stored.position >= MIN_RESUME_SECONDS) return current;
  return {
    last: itemKey,
    items: {
      ...current.items,
      [itemKey]: { position: Math.max(0, Math.min(position, duration)), duration }
    }
  };
}

export function isWatched(entry) {
  return !!entry && entry.duration > 0 && entry.position / entry.duration >= WATCHED_RATIO;
}

// Seconds to resume an item at, or 0 when it should start from the top.
export function resumePosition(entry) {
  if (!entry || isWatched(entry) || entry.position < MIN_RESUME_SECONDS) return 0;
  return entry.position;
}

// Share of the item already seen, 0..1, for the bar under an episode row.
export function watchedFraction(entry) {
  if (!entry || !(entry.duration > 0)) return 0;
  return isWatched(entry) ? 1 : entry.position / entry.duration;
}

// Where a series picks up: the episode last watched, or the one after it when
// that was finished. Null when nothing has been watched, or the last episode of
// the share was seen to the end.
export function seriesResumeTarget(record, episodes) {
  if (!record?.last || !episodes?.length) return null;
  const i = episodes.findIndex((e) => e.id === record.last);
  if (i < 0) return null;
  const entry = record.items[record.last];
  if (isWatched(entry)) {
    const next = episodes[i + 1];
    return next ? { episode: next, position: 0, isNext: true } : null;
  }
  return { episode: episodes[i], position: resumePosition(entry), isNext: false };
}

export function formatClock(seconds) {
  const s = Math.max(0, Math.floor(seconds || 0));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}
