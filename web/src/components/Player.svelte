<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import Hls from 'hls.js';
  import PlayIcon from './PlayIcon.svelte';
  import { fullscreenElement, enterFullscreen, exitFullscreen } from '../fullscreen.js';

  export let playbackData;
  export let title;
  // Names the sidecar track in the player's menu; the viewer chose this language.
  export let subtitleLabel = 'Subtitles';
  export let subtitleLanguage = '';
  // Seconds to start at, when the viewer is resuming.
  export let startPosition = 0;
  // { label, name } of the episode that follows, or null when nothing does.
  export let nextUp = null;
  // Called as (key, position, duration). A callback rather than an event: it
  // also runs while this instance is torn down, when events no longer arrive.
  export let onProgress = null;
  export let progressKey = null;

  const dispatch = createEventDispatcher();

  // Captured once, at creation. Autoplay replaces this component with one for the
  // next episode, and the heartbeat and the finish call must stay aimed at the
  // session this instance was given - reading the prop as it is torn down risks
  // finishing the session that just took its place.
  const sessionId = playbackData?.sessionId;
  // Same reasoning: progress belongs to the item this instance was opened for.
  const itemKey = progressKey;

  // The card offering the next episode appears this long before the end.
  const NEXT_UP_SECONDS = 30;

  let videoElement;
  let hls;
  let heartbeatInterval;
  // error ends playback and offers a way back. recovering is a passing condition
  // hls.js is already working through, and it clears itself once frames flow
  // again - conflating the two is what put "Playback stopped" over a film that
  // was playing perfectly well.
  let error = null;
  let recovering = null;
  let recoveryAttempts = 0;
  let lastRecoveryAt = 0;

  onMount(() => {
    initPlayer();
    startHeartbeat();
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    document.addEventListener('webkitfullscreenchange', handleFullscreenChange);
    // An iPhone's native video fullscreen is not document fullscreen; the video
    // reports it on its own.
    videoElement?.addEventListener('webkitbeginfullscreen', onVideoFullscreen);
    videoElement?.addEventListener('webkitendfullscreen', onVideoFullscreen);
    videoElement?.addEventListener('timeupdate', handleTimeUpdate);
    videoElement?.addEventListener('pause', reportProgress);

    // Fires on the element itself, so this covers both paths: hls.js feeding it
    // through MediaSource, and Safari playing the HLS natively - which is also
    // what AirPlay mirrors, so an episode finishing on an Apple TV lands here too.
    videoElement?.addEventListener('ended', handleEnded);
    videoElement?.addEventListener('loadedmetadata', showSubtitles);
    // Frames are moving again, so whatever hls.js was recovering from is over.
    videoElement?.addEventListener('playing', clearRecovering);
    videoElement?.addEventListener('timeupdate', clearRecovering);

    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
      document.removeEventListener('webkitfullscreenchange', handleFullscreenChange);
      videoElement?.removeEventListener('webkitbeginfullscreen', onVideoFullscreen);
      videoElement?.removeEventListener('webkitendfullscreen', onVideoFullscreen);
      videoElement?.removeEventListener('ended', handleEnded);
      videoElement?.removeEventListener('loadedmetadata', showSubtitles);
      videoElement?.removeEventListener('playing', clearRecovering);
      videoElement?.removeEventListener('timeupdate', clearRecovering);
      videoElement?.removeEventListener('timeupdate', handleTimeUpdate);
      videoElement?.removeEventListener('pause', reportProgress);
    };
  });

  // The `default` attribute is a hint, and Chromium browsers routinely ignore it:
  // the track loads but stays disabled, so a viewer who picked German subtitles
  // still has to switch them on in the player's own menu. Setting the mode is what
  // makes the choice stick. A track only exists here because it was asked for, so
  // turning it on is restoring the selection, not overriding a preference - and
  // the menu still switches it off.
  function showSubtitles() {
    const tracks = videoElement?.textTracks;
    if (!tracks) return;
    for (let i = 0; i < tracks.length; i++) {
      if (tracks[i].kind === 'subtitles') tracks[i].mode = 'showing';
    }
  }

  function handleEnded() {
    finish('ended');
  }

  // ---- Progress ----
  let lastReportAt = 0;

  // Set once the episode is over - played out, or left for the next one from
  // the card. From then on it is reported as watched, whatever second the
  // credits were cut off at, including by the report made on teardown.
  let finished = false;

  function reportProgress() {
    if (!onProgress || !videoElement) return;
    const duration = videoElement.duration;
    // A stream that has not loaded yet reports 0 or NaN; saving that would wipe
    // the position the viewer is resuming from.
    if (!(duration > 0) || !Number.isFinite(duration)) return;
    if (!finished && !(videoElement.currentTime > 0)) return;
    lastReportAt = Date.now();
    onProgress(itemKey, finished ? duration : videoElement.currentTime, duration);
  }

  // ---- Next episode ----
  let remaining = Infinity;
  $: showNextUp = !!nextUp && !error && !finished && remaining <= NEXT_UP_SECONDS && remaining > 0;
  $: countdown = Math.ceil(remaining);
  // How far through the countdown we are, 0..1, for the fill behind the button.
  $: nextUpFill = Math.min(1, Math.max(0, 1 - remaining / NEXT_UP_SECONDS));

  function handleTimeUpdate() {
    const duration = videoElement?.duration;
    // Only the last stretch matters to the card; outside it, leave the
    // reactive statements above alone rather than waking them on every tick.
    if (nextUp && duration > 0 && Number.isFinite(duration)) {
      const left = duration - videoElement.currentTime;
      if (left <= NEXT_UP_SECONDS + 1 || remaining !== Infinity) {
        remaining = left > NEXT_UP_SECONDS + 1 ? Infinity : left;
      }
    }
    if (Date.now() - lastReportAt > 5000) reportProgress();
  }

  function playNext() {
    finish('next');
  }

  // An episode ends once, however it ends. "Play now" in the last seconds would
  // otherwise be followed by the video's own 'ended', and a double click by a
  // second 'next' - each asking the parent for the next episode again, which on
  // a link limited to one viewer can refuse the viewer their own next episode.
  function finish(event) {
    if (finished) return;
    finished = true;
    videoElement?.pause();
    reportProgress();
    dispatch(event);
  }

  function clearRecovering() {
    if (recovering) recovering = null;
  }

  onDestroy(() => {
    clearTimeout(idleTimer);
    cleanup();
  });

  function initPlayer() {
    if (!playbackData?.playbackUrl) {
      error = 'No playback URL provided';
      return;
    }

    // Prefer native HLS where the browser also has AirPlay. hls.js plays through
    // MediaSource, and MediaSource playback cannot be sent to an AirPlay receiver
    // — so on Safari, choosing hls.js would silently remove the only casting route
    // that browser has. The presence of the AirPlay API is the precise test: it is
    // what makes the trade-off matter.
    const hasAirPlay = typeof window !== 'undefined'
      && 'WebKitPlaybackTargetAvailabilityEvent' in window;
    const nativeHls = !!videoElement.canPlayType('application/vnd.apple.mpegurl');

    if (Hls.isSupported() && !(hasAirPlay && nativeHls)) {
      hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
        backBufferLength: 90,
        // Load from the resume point rather than seeking after the first
        // segments, which would fetch and throw away the opening of the file.
        startPosition: startPosition > 0 ? startPosition : -1
      });

      hls.loadSource(playbackData.playbackUrl);
      hls.attachMedia(videoElement);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        videoElement.play().catch(e => {
          console.log('Autoplay prevented:', e);
        });
      });

      hls.on(Hls.Events.ERROR, (event, data) => {
        if (!data.fatal) return;
        console.warn('hls.js fatal error', data.type, data.details);

        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            recovering = 'Connection interrupted, reconnecting';
            hls.startLoad();
            break;

          case Hls.ErrorTypes.MEDIA_ERROR:
            // Common on this pipeline: the video is stream-copied while the audio
            // is re-encoded, and the two can disagree at the first append. It is
            // recoverable, so it must not read as the end of playback.
            //
            // hls.js asks for this escalation rather than a bare retry loop: a
            // plain recover, then a codec swap, then admit defeat. Attempts made
            // more than a few seconds apart are separate incidents, not a spin.
            if (Date.now() - lastRecoveryAt > 5000) recoveryAttempts = 0;
            lastRecoveryAt = Date.now();
            recoveryAttempts++;

            if (recoveryAttempts === 1) {
              recovering = 'Recovering the stream';
              hls.recoverMediaError();
            } else if (recoveryAttempts === 2) {
              recovering = 'Recovering the stream';
              hls.swapAudioCodec();
              hls.recoverMediaError();
            } else {
              recovering = null;
              error = 'This stream could not be decoded in this browser';
              cleanup();
            }
            break;

          default:
            recovering = null;
            error = 'Playback error occurred';
            cleanup();
            break;
        }
      });
    } else if (nativeHls) {
      // Native HLS: Safari, and the path that keeps AirPlay available
      videoElement.src = playbackData.playbackUrl;
      videoElement.addEventListener('loadedmetadata', () => {
        if (startPosition > 0) videoElement.currentTime = startPosition;
        videoElement.play().catch(e => {
          console.log('Autoplay prevented:', e);
        });
      });
      // The hls.js branch reports fatal errors through its own handler; this one
      // had none, so a revoked share or a timed-out session left a blank player
      // with no message. Safari is routed here now, so it needs one.
      videoElement.addEventListener('error', () => {
        error = 'Playback error occurred';
        cleanup();
      });
    } else {
      error = 'HLS playback is not supported in this browser';
    }
  }

  function startHeartbeat() {
    heartbeatInterval = setInterval(async () => {
      if (!sessionId) return;

      try {
        const response = await fetch(`/api/public/sessions/${sessionId}/heartbeat`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            positionSeconds: Math.floor(videoElement?.currentTime || 0)
          }),
          credentials: 'include'
        });

        if (response.ok) {
          const data = await response.json();
          if (data.status !== 'ok') {
            error = data.message || 'Session ended';
            cleanup();
          }
        }
      } catch (e) {
        console.error('Heartbeat failed:', e);
      }
    }, 15000); // Every 15 seconds
  }

  async function cleanup() {
    // First: hls.destroy() below detaches the media, after which the element no
    // longer knows its duration and the position cannot be saved.
    reportProgress();

    if (heartbeatInterval) {
      clearInterval(heartbeatInterval);
      heartbeatInterval = null;
    }

    if (hls) {
      hls.destroy();
      hls = null;
    }

    // Notify server that playback ended
    if (sessionId) {
      try {
        await fetch(`/api/public/sessions/${sessionId}/finish`, {
          method: 'POST',
          credentials: 'include'
        });
      } catch (e) {
        console.error('Failed to notify session end:', e);
      }
    }
  }

  function handleClose() {
    cleanup();
    dispatch('close');
  }

  // Fullscreen goes to the whole page, not the bare <video>: in a fullscreen
  // video element nothing of the page can paint, and the next-episode card would
  // be lost exactly where people watch. Nor to this player: it is rebuilt for
  // every episode, and removing the fullscreen element leaves fullscreen - the
  // viewer would drop out of it at each autoplay. The page survives the swap.
  //
  // That needs a control of our own. The video's control fullscreens the video
  // element, and handing that up to the page afterwards does not work: the
  // browser spends the click on its own request and refuses the second one, so
  // fullscreen opened and closed at once. Where the browser lets the native
  // control be hidden (controlsList), this player's button takes its place;
  // elsewhere - Safari, Firefox, the iPhone - the native one stays, and the
  // video goes fullscreen on its own.
  const ownFullscreenControl = 'controlsList' in HTMLMediaElement.prototype;
  // Read, not assumed: after autoplay this instance starts inside a page that
  // is already fullscreen, and no fullscreenchange will say so.
  let isFullscreen = !!fullscreenElement();
  // The iPhone's native video fullscreen, which Escape must not close the
  // player under.
  let videoFullscreen = false;

  function onVideoFullscreen(event) {
    videoFullscreen = event.type === 'webkitbeginfullscreen';
  }

  function handleFullscreenChange() {
    isFullscreen = !!fullscreenElement();
  }

  function handleDoubleClick() {
    if (ownFullscreenControl) toggleFullscreen();
  }

  // In fullscreen the title bar steps aside once the pointer rests, the way
  // the video's own controls do, and comes back when it moves.
  let idle = false;
  let idleTimer;

  function wake() {
    idle = false;
    clearTimeout(idleTimer);
    idleTimer = setTimeout(() => (idle = true), 2500);
  }

  function toggleFullscreen() {
    if (fullscreenElement()) {
      exitFullscreen();
    } else if (videoFullscreen) {
      videoElement.webkitExitFullscreen?.();
    } else {
      enterFullscreen(videoElement);
    }
  }

  function handleKeydown(event) {
    switch (event.key) {
      case 'Escape':
        if (!fullscreenElement() && !videoFullscreen) {
          handleClose();
        }
        break;
      case ' ':
        // A focused button - Play now, Cancel - is activated by Space itself.
        if (event.target instanceof HTMLElement && event.target.closest('button')) return;
        event.preventDefault();
        if (videoElement.paused) {
          videoElement.play();
        } else {
          videoElement.pause();
        }
        break;
      case 'f':
        toggleFullscreen();
        break;
      case 'ArrowLeft':
        videoElement.currentTime -= 10;
        break;
      case 'ArrowRight':
        videoElement.currentTime += 10;
        break;
    }
  }
</script>

<!-- Reloading or closing the tab tears nothing down, so the regular report
     never runs; pagehide is the last moment the page is still there to save the
     exact position. A tab sent to the background on a phone may never come
     back, which is what the visibility change covers. -->
<svelte:window on:keydown={handleKeydown} on:pagehide={reportProgress}
               on:mousemove={wake} on:touchstart={wake} />
<svelte:document on:visibilitychange={() => document.visibilityState === 'hidden' && reportProgress()} />

<div class="player" class:player--idle={idle}>
  <!-- The video is the page. Chrome floats over it and gets out of the way.
       The fullscreen button only appears where it replaces the video's own. -->
  <header class="player__bar">
    <h2 class="player__title">{title}</h2>
    <div class="player__tools">
      {#if ownFullscreenControl}
        <button class="glyph" on:click={toggleFullscreen}
                title={isFullscreen ? 'Exit fullscreen (F)' : 'Fullscreen (F)'}
                aria-label={isFullscreen ? 'Exit fullscreen' : 'Fullscreen'}>
          {#if isFullscreen}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M8 3v3a2 2 0 01-2 2H3M21 8h-3a2 2 0 01-2-2V3M16 21v-3a2 2 0 012-2h3M3 16h3a2 2 0 012 2v3"/>
            </svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M8 3H5a2 2 0 00-2 2v3M21 8V5a2 2 0 00-2-2h-3M3 16v3a2 2 0 002 2h3M16 21h3a2 2 0 002-2v-3"/>
            </svg>
          {/if}
        </button>
      {/if}
      <button class="glyph" on:click={handleClose} title="Back (Esc)" aria-label="Back to the share">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </div>
  </header>

  <div class="player__stage">
    {#if error}
      <div class="player__error">
        <p class="player__error-head">Playback stopped</p>
        <p class="player__error-body">{error}</p>
        <button class="btn-back" on:click={handleClose}>Back to the share</button>
      </div>
    {:else if recovering}
      <p class="player__recovering">{recovering}…</p>
    {/if}

    <!-- x-webkit-airplay covers Safari, where Google Cast does not exist: Safari
         takes the native HLS path below rather than MediaSource, so the system
         AirPlay button appears in the player controls on its own. -->
    <video
      bind:this={videoElement}
      controls
      controlslist={ownFullscreenControl ? 'nofullscreen' : undefined}
      class:own-fullscreen={ownFullscreenControl}
      on:dblclick={handleDoubleClick}
      playsinline
      autoplay
      x-webkit-airplay="allow"
    >
      {#if playbackData?.subtitleUrl}
        <!-- Text subtitles arrive as a sidecar, so the video needed no re-encode
             and the viewer can switch them off in the player's own menu. -->
        <track
          kind="subtitles"
          label={subtitleLabel}
          srclang={subtitleLanguage || undefined}
          src={playbackData.subtitleUrl}
          on:load={showSubtitles}
          default
        />
      {:else}
        <track kind="captions" />
      {/if}
    </video>

    {#if showNextUp}
      <!-- Sits in the corner the native controls leave free, above their bar. -->
      <aside class="nextup" aria-label="Next episode" aria-live="polite">
        <button class="nextup__x" on:click={handleClose} aria-label="Cancel and go back to the episodes">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
            <path d="M18 6L6 18M6 6l12 12"/>
          </svg>
        </button>
        <p class="nextup__eyebrow">Next episode</p>
        <p class="nextup__title">
          <span class="nextup__no">{nextUp.label}</span>
          <span class="nextup__name">{nextUp.name}</span>
        </p>
        <div class="nextup__actions">
          <button class="nextup__play" on:click={playNext}>
            <span class="nextup__fill" style="transform: scaleX({nextUpFill})" aria-hidden="true"></span>
            <PlayIcon />
            <span>Play now</span>
            <span class="nextup__count">{countdown}s</span>
          </button>
          <button class="nextup__cancel" on:click={handleClose}>Cancel</button>
        </div>
      </aside>
    {/if}
  </div>
</div>

<style>
  /* Hallmark · genre: atmospheric · macrostructure: none — the video is the page
   * design-system: design.md · designed-as-app
   */
  .player {
    position: relative;
    display: grid;
    min-height: 100dvh;
    background: var(--color-paper);
  }

  /* A scrim, not a bar: the chrome dissolves into the picture instead of
     boxing it in. */
  .player__bar {
    position: absolute;
    inset: 0 0 auto 0;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-md);
    padding: var(--space-md) var(--page-gutter) var(--space-2xl);
    padding-top: max(var(--space-md), env(safe-area-inset-top));
    background: linear-gradient(to bottom, var(--scrim-strong), transparent);
    z-index: var(--z-raised);
    pointer-events: none;
  }

  .player__bar > * { pointer-events: auto; }

  /* Fullscreen is for watching: the title bar steps aside, Esc leaves. */
  .player__bar { transition: opacity var(--dur-short) var(--ease-out); }

  :global(:root:fullscreen) .player--idle { cursor: none; }

  :global(:root:fullscreen) .player--idle .player__bar,
  :global(:root:fullscreen) .player--idle .player__bar > * {
    opacity: 0;
    pointer-events: none;
  }

  .player__title {
    font-size: var(--text-md);
    line-height: 1.2;
    color: var(--color-ink);
    overflow-wrap: anywhere;
    min-width: 0;
  }

  .player__tools {
    display: flex;
    gap: var(--space-2xs);
    flex-shrink: 0;
  }

  .glyph {
    position: relative;
    display: grid;
    place-items: center;
    width: 2.5rem;
    height: 2.5rem;
    background: color-mix(in oklch, var(--color-paper-2) 70%, transparent);
    backdrop-filter: blur(10px);
    color: var(--color-ink-2);
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: color var(--dur-micro) var(--ease-out),
                background-color var(--dur-micro) var(--ease-out);
  }

  .glyph::before { content: ""; position: absolute; inset: -0.35rem; }

  .glyph svg { width: 1.1rem; height: 1.1rem; }

  @media (hover: hover) {
    .glyph:hover { color: var(--color-accent); background: var(--color-paper-3); }
  }

  .glyph:active { transform: translateY(1px); }

  .glyph:disabled { opacity: 0.55; cursor: not-allowed; }

  .player__stage {
    position: relative;
    display: grid;
    place-items: center;
    min-height: 100dvh;
  }

  video {
    width: 100%;
    max-height: 100dvh;
    background: var(--color-paper);
  }

  /* A passing condition, not a stopped player: a quiet line that sits over the
     picture without covering it, and leaves on its own once frames return. */
  .player__recovering {
    position: absolute;
    top: var(--space-md);
    left: 50%;
    transform: translateX(-50%);
    padding: var(--space-xs) var(--space-md);
    border-radius: var(--radius-pill);
    background: var(--scrim-strong);
    color: var(--color-ink-2);
    font-size: var(--text-sm);
    z-index: var(--z-dropdown);
    pointer-events: none;
  }

  .player__error {
    position: absolute;
    inset: 0;
    display: grid;
    align-content: center;
    justify-items: start;
    gap: var(--space-sm);
    padding-inline: var(--page-gutter);
    background: var(--scrim-strong);
    z-index: var(--z-dropdown);
  }

  .player__error-head {
    font-family: var(--font-display);
    font-size: var(--text-xl);
    color: var(--color-ink);
  }

  .player__error-body {
    max-width: 48ch;
    color: var(--color-muted);
  }

  .btn-back {
    margin-top: var(--space-md);
    height: var(--control-h);
    padding-inline: var(--space-lg);
    background: var(--color-accent);
    color: var(--color-accent-ink);
    font-weight: 600;
    white-space: nowrap;
    border: none;
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: transform var(--dur-micro) var(--ease-out);
  }

  @media (hover: hover) {
    .btn-back:hover { transform: translateY(-1px); }
  }

  .btn-back:active { transform: translateY(1px); }
  .btn-back:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 3px; }

  /* ── Next episode ──────────────────────────────────────────────────────
     Bottom right, lifted clear of the native control bar. A card, not a
     modal: the credits keep playing underneath and nothing else is blocked. */
  .nextup {
    position: absolute;
    right: var(--page-gutter);
    bottom: calc(var(--space-2xl) + env(safe-area-inset-bottom));
    z-index: var(--z-dropdown);
    display: grid;
    gap: var(--space-xs);
    width: min(22rem, calc(100% - 2 * var(--page-gutter)));
    padding: var(--space-md) var(--space-md) var(--space-md) var(--space-lg);
    background: color-mix(in oklch, var(--color-paper-2) 88%, transparent);
    backdrop-filter: blur(14px);
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius-card);
    animation: nextup-in var(--dur-long) var(--ease-out);
  }

  @keyframes nextup-in {
    from { opacity: 0; transform: translateY(0.75rem); }
  }

  @media (prefers-reduced-motion: reduce) {
    .nextup { animation: none; }
  }

  .nextup__x {
    position: absolute;
    top: var(--space-xs);
    right: var(--space-xs);
    display: grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    padding: 0;
    background: none;
    color: var(--color-muted);
    border: none;
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: color var(--dur-micro) var(--ease-out),
                background-color var(--dur-micro) var(--ease-out);
  }

  .nextup__x svg { width: 1rem; height: 1rem; }

  @media (hover: hover) {
    .nextup__x:hover { color: var(--color-ink); background: var(--color-paper-3); }
  }

  .nextup__eyebrow {
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--color-neutral);
  }

  .nextup__title {
    display: grid;
    gap: var(--space-3xs);
    padding-right: var(--space-xl);
    min-width: 0;
  }

  .nextup__no {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    letter-spacing: 0.06em;
    color: var(--color-accent);
  }

  .nextup__name {
    font-weight: 600;
    line-height: 1.3;
    color: var(--color-ink);
    overflow-wrap: anywhere;
  }

  .nextup__actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-xs);
    margin-top: var(--space-xs);
  }

  .nextup__play, .nextup__cancel {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-xs);
    height: var(--control-h);
    padding-inline: var(--space-lg);
    font-weight: 600;
    white-space: nowrap;
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: transform var(--dur-micro) var(--ease-out),
                background-color var(--dur-micro) var(--ease-out),
                color var(--dur-micro) var(--ease-out);
  }

  /* The countdown fills the button from the left, so the autoplay deadline is
     something seen rather than a number to read. */
  .nextup__play {
    position: relative;
    overflow: hidden;
    isolation: isolate;
    background: var(--color-accent-dim);
    color: var(--color-accent-ink);
    border: none;
  }

  .nextup__fill {
    position: absolute;
    inset: 0;
    z-index: -1;
    background: var(--color-accent);
    transform-origin: left center;
    transition: transform 250ms linear;
  }

  .nextup__play :global(svg) { width: 1rem; height: 1rem; flex-shrink: 0; }

  .nextup__count {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    opacity: 0.8;
  }

  .nextup__cancel {
    background: none;
    color: var(--color-ink-2);
    border: var(--rule-hair) solid var(--color-rule);
  }

  @media (hover: hover) {
    .nextup__play:hover { transform: translateY(-1px); }
    .nextup__cancel:hover { background: var(--color-paper-3); color: var(--color-ink); }
  }

  .nextup__play:active, .nextup__cancel:active { transform: translateY(1px); }
  .nextup__play:focus-visible, .nextup__cancel:focus-visible, .nextup__x:focus-visible {
    outline: 2px solid var(--color-focus);
    outline-offset: 3px;
  }

  /* controlsList only greys the native button out in Chrome; a disabled twin
     beside the player's own would read as broken. */
  video.own-fullscreen::-webkit-media-controls-fullscreen-button { display: none; }

  video::cue {
    background: var(--scrim-strong);
    color: var(--color-ink);
    font-family: var(--font-body);
  }
</style>
