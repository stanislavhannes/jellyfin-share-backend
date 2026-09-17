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
    // Required for the SUBTITLES subtype; the receiver rejects the track without it.
    track.language = subtitleLanguage || 'und';
    track.name = 'Subtitles';
    info.tracks = [track];
  }

  const request = new chrome.cast.media.LoadRequest(info);
  request.autoplay = true;
  if (subtitleUrl) request.activeTrackIds = [1];

  await session.loadMedia(request);
  return session;
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
  switch (code) {
    case 'timeout': return 'The cast device did not respond';
    case 'receiver_unavailable': return 'No cast device could be reached';
    case 'session_error': return 'The cast device refused the media';
    default: return 'Could not cast to the device';
  }
}
