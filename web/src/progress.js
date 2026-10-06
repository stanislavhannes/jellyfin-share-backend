// Where the viewer left off, kept in this browser. A share link has no account
// behind it, so the browser is the only place that can remember - and that is
// also the right scope: a link forwarded to someone else should not open on the
// sender's position.
//
// One record per share token: the item last watched, and a position per item.
// The item key is the episode id, or MAIN for a film.

export const MAIN = 'main';

// Under this, the viewer has barely started; offering to resume would only add
// a second button that does the same as the first.
const MIN_RESUME_SECONDS = 10;
// Past this share of the runtime the credits are rolling - the item counts as
// watched, and a series resumes on the next episode instead.
const WATCHED_RATIO = 0.92;

const keyFor = (token) => `jfshare:progress:${token}`;

export function loadProgress(token) {
  try {
    const parsed = JSON.parse(localStorage.getItem(keyFor(token)) || 'null');
    if (parsed && typeof parsed === 'object' && parsed.items) return parsed;
  } catch (e) {
    // Storage blocked (private mode, a strict policy) or a mangled record:
    // the page works the same, it just cannot remember.
  }
  return { last: null, items: {} };
}

// Takes the record the caller already holds - this runs every few seconds
// during playback - and returns a new one so the caller stays reactive.
export function saveProgress(token, current, itemKey, position, duration) {
  if (!itemKey || !(duration > 0)) return current;
  const record = {
    last: itemKey,
    items: {
      ...current.items,
      [itemKey]: { position: Math.max(0, Math.min(position, duration)), duration }
    }
  };
  try {
    localStorage.setItem(keyFor(token), JSON.stringify(record));
  } catch (e) {
    // Full or blocked - nothing to do but keep going.
  }
  return record;
}

function isWatched(entry) {
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
