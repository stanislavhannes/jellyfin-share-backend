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
        const session = context.getCurrentSession();
        castDeviceName.set(session ? session.getCastDevice().friendlyName : '');
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
 * Sends media to the connected receiver, asking for a device first if needed.
 * `subtitleUrl` may be relative; it is resolved against the current page because
 * the receiver has no page context of its own.
 */
export async function castMedia({ url, title, subtitle, posterUrl, subtitleUrl, durationSeconds }) {
  if (!context) throw new Error('Cast is not available');

  let session = context.getCurrentSession();
  if (!session) {
    await context.requestSession();
    session = context.getCurrentSession();
    if (!session) throw new Error('No cast device was selected');
  }

  const abs = (u) => (u ? new URL(u, window.location.href).href : undefined);

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
