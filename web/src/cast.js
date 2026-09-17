// Google Cast sender.
//
// The SDK only works on a secure origin - browsers dropped the Presentation API
// on plain HTTP - so on an http:// deployment everything here stays inert and the
// UI simply never offers the button, rather than failing when it is pressed.
//
// The Chromecast fetches the media itself, so every URL handed to it has to be
// absolute and reachable from the device, not just from the page.

import { writable } from 'svelte/store';

// The SDK is loaded and initialised. Separate from castAvailable on purpose:
// the button is shown as soon as casting is possible at all, so a viewer with no
// receiver on the network gets Chrome's own "no devices found" dialog instead of
// a button that silently never appears.
export const castApiReady = writable(false);
export const castAvailable = writable(false);
export const castConnected = writable(false);
export const castDeviceName = writable('');
// Set when the receiver would not take the subtitle track and the video was sent
// without it, so the page can say so instead of leaving the viewer guessing.
export const subtitlesDropped = writable(false);

const SDK_SRC = 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1';

let context = null;
let initialised = false;

export function isCastPossible() {
  return typeof window !== 'undefined' && window.isSecureContext;
}

/**
 * Loads the SDK and wires up state. Safe to call more than once, and a no-op on
 * an insecure origin.
 */
export function initCast() {
  if (initialised || !isCastPossible()) return;
  initialised = true;

  window.__onGCastApiAvailable = (available) => {
    if (!available) return;
    try {
      context = cast.framework.CastContext.getInstance();
      context.setOptions({
        // The default receiver needs no registration with Google.
        receiverApplicationId: chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID,
        autoJoinPolicy: chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED
      });

      const applyState = (state) => {
        castAvailable.set(state !== cast.framework.CastState.NO_DEVICES_AVAILABLE);
        castConnected.set(state === cast.framework.CastState.CONNECTED);
        // During CONNECTING a session can exist whose device is not resolved yet;
        // this listener runs outside the try above, so a throw here would be
        // swallowed by the SDK and leave the stores stale.
        const session = context.getCurrentSession();
        castDeviceName.set(session?.getCastDevice()?.friendlyName || '');
      };

      watchPlayer();

      castApiReady.set(true);
      applyState(context.getCastState());
      context.addEventListener(
        cast.framework.CastContextEventType.CAST_STATE_CHANGED,
        (e) => applyState(e.castState)
      );
    } catch (e) {
      console.warn('Cast: initialisation failed', e);
    }
  };

  const s = document.createElement('script');
  s.src = SDK_SRC;
  s.async = true;
  s.onerror = () => console.warn('Cast: SDK could not be loaded');
  document.head.appendChild(s);
}

/**
 * Makes sure a receiver is connected, opening the device picker if needed.
 *
 * Kept separate from loading on purpose: the caller starts a playback session on
 * the server, which counts against the share's play limit, and that must not
 * happen until a device has actually been chosen. Dismissing the picker would
 * otherwise burn a play.
 */
export async function ensureCastSession() {
  if (!context) throw new Error('Cast is not available');
  let session = context.getCurrentSession();
  if (!session) {
    await context.requestSession();
    session = context.getCurrentSession();
    if (!session) throw new Error('No cast device was selected');
  }
  return session;
}

/**
 * Loads media on the already-connected receiver.
 *
 * Relative URLs are resolved against `url`, not against the page: the playback
 * URL comes from the server's configured public base, which is the address the
 * receiver can reach, while the page may have been opened on a different host.
 */
export async function loadOnCast({ url, title, subtitle, posterUrl, subtitleUrl, subtitleLanguage, durationSeconds }) {
  const session = await ensureCastSession();

  const abs = (u) => (u ? new URL(u, url).href : undefined);

  // HLS delivered as mpegts segments; the receiver is told so explicitly rather
  // than left to sniff it from the extension.
  const info = new chrome.cast.media.MediaInfo(abs(url), 'application/x-mpegURL');
  info.streamType = chrome.cast.media.StreamType.BUFFERED;
  if (durationSeconds) info.duration = durationSeconds;

  const meta = new chrome.cast.media.GenericMediaMetadata();
  meta.title = title || 'Shared media';
  if (subtitle) meta.subtitle = subtitle;
  if (posterUrl) meta.images = [new chrome.cast.Image(abs(posterUrl))];
  info.metadata = meta;

  if (subtitleUrl) {
    const track = new chrome.cast.media.Track(1, chrome.cast.media.TrackType.TEXT);
    track.trackContentId = abs(subtitleUrl);
    track.trackContentType = 'text/vtt';
    track.subtype = chrome.cast.media.TextTrackType.SUBTITLES;
    // Required for the SUBTITLES subtype; the receiver rejects the track without
    // it, and it wants a BCP-47 tag. Jellyfin reports ISO 639-2 ("deu"), which
    // the browser canonicalises for us ("de") - no mapping table needed.
    track.language = toBcp47(subtitleLanguage);
    track.name = languageName(track.language) || 'Subtitles';
    info.tracks = [track];
  }

  const load = (withTracks) => {
    const req = new chrome.cast.media.LoadRequest(info);
    req.autoplay = true;
    if (withTracks) req.activeTrackIds = [1];
    return session.loadMedia(req);
  };

  subtitlesDropped.set(false);

  if (info.tracks) {
    try {
      await load(true);
    } catch (e) {
      // Google's Default Media Receiver does not sideload text tracks onto an HLS
      // stream - it expects subtitles inside the manifest - and refuses the whole
      // load rather than just the track. That is a refusal of the video too, so
      // the viewer gets a connected device and no picture.
      //
      // Dropping the track and loading again is the difference between subtitles
      // and nothing at all. The page says so rather than quietly losing them.
      console.warn('Cast refused the media with a subtitle track; retrying without', e);
      delete info.tracks;
      await load(false);
      subtitlesDropped.set(true);
    }
  } else {
    await load(false);
  }

  watchMedia(session);
  return session;
}

// The receiver has no queue here, so the sender is what advances the series: it
// waits for the episode to finish and loads the next one. A real Cast queue would
// need every episode's URL up front, and each of those is a pinned session - ten
// sessions opened the moment playback starts, all counting as viewers. Keeping
// the sender in charge costs nothing but a browser tab that stays open.
//
// Two detectors, because one is not dependable. RemotePlayerController survives
// the media session it is watching, which an update listener bound to a single
// media object does not - that object is torn down at the very moment the episode
// ends, which is the moment we care about. The media listener stays as the second
// route in case the player state arrives without a media session to read the
// reason from. reportEnded settles which of them got there first.
let endedHandler = null;
let lastEndedMediaId = null;

function reportEnded(mediaSessionId) {
  // The end can be reported by both routes, and a state can repeat. Advancing
  // twice would skip an episode, so each media session ends the series once.
  if (mediaSessionId != null && mediaSessionId === lastEndedMediaId) return;
  lastEndedMediaId = mediaSessionId;
  if (endedHandler) endedHandler();
}

function watchPlayer() {
  const player = new cast.framework.RemotePlayer();
  const controller = new cast.framework.RemotePlayerController(player);
  controller.addEventListener(
    cast.framework.RemotePlayerEventType.PLAYER_STATE_CHANGED,
    () => {
      if (player.playerState !== chrome.cast.media.PlayerState.IDLE) return;
      const media = context?.getCurrentSession()?.getMediaSession();
      // FINISHED is what separates an episode that ran out from one the viewer
      // stopped, or a receiver that was disconnected. Only the first advances.
      console.debug('Cast: receiver idle', media?.idleReason);
      if (media?.idleReason !== chrome.cast.media.IdleReason.FINISHED) return;
      reportEnded(media.mediaSessionId);
    }
  );
}

function watchMedia(session) {
  const media = session.getMediaSession();
  if (!media) return;
  const id = media.mediaSessionId;
  media.addUpdateListener(() => {
    if (media.idleReason === chrome.cast.media.IdleReason.FINISHED) reportEnded(id);
  });
}

// Registers what to do when an episode finishes on the receiver. One handler at a
// time, replaced rather than stacked, so a re-registration cannot advance twice.
export function onCastEnded(fn) {
  endedHandler = fn;
}

export function toBcp47(code) {
  if (!code) return 'und';
  try {
    return Intl.getCanonicalLocales(code)[0] || 'und';
  } catch (e) {
    return 'und';
  }
}

// Gives the receiver something readable in its track menu - "German" rather than
// a bare "Subtitles" - falling back when the language is unknown.
function languageName(tag) {
  if (!tag || tag === 'und') return null;
  try {
    return new Intl.DisplayNames([navigator.language || 'en'], { type: 'language' }).of(tag);
  } catch (e) {
    return null;
  }
}

export function stopCast() {
  if (context) context.endCurrentSession(true);
}

/**
 * The SDK rejects with an error code string or an event-like object, not an
 * Error, so `e.message` is usually undefined. Returns null when the viewer simply
 * dismissed the picker, which is not a failure worth showing.
 */
export function describeCastError(e) {
  const code = typeof e === 'string' ? e : (e?.code || e?.message);
  if (code === 'cancel' || code === chrome?.cast?.ErrorCode?.CANCEL) return null;

  // Anything unexpected reaches the console in full. The message below is all the
  // viewer sees, and "could not cast" on its own is impossible to act on - for a
  // failure that only shows up on real hardware, the code is the whole diagnosis.
  console.error('Cast failed', code, e);

  switch (code) {
    case 'timeout': return 'The cast device did not respond';
    case 'receiver_unavailable': return 'No cast device could be reached';
    case 'session_error': return 'The cast device refused the media';
    case 'load_failed': return 'The cast device could not load the stream';
    case 'invalid_parameter': return 'The cast device rejected the request';
    default: return `Could not cast to the device (${code || 'unknown error'})`;
  }
}
