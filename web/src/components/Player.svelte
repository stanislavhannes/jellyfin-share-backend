<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import Hls from 'hls.js';

  export let playbackData;
  export let title;
  // Names the sidecar track in the player's menu; the viewer chose this language.
  export let subtitleLabel = 'Subtitles';
  export let subtitleLanguage = '';

  const dispatch = createEventDispatcher();

  // Captured once, at creation. Autoplay replaces this component with one for the
  // next episode, and the heartbeat and the finish call must stay aimed at the
  // session this instance was given - reading the prop as it is torn down risks
  // finishing the session that just took its place.
  const sessionId = playbackData?.sessionId;

  let videoElement;
  let hls;
  let heartbeatInterval;
  // error ends playback and offers a way back. recovering is a passing condition
  // hls.js is already working through, and it clears itself once frames flow
  // again - conflating the two is what put "Playback stopped" over a film that
  // was playing perfectly well.
  let error = null;
  let recovering = null;
  let isFullscreen = false;
  let recoveryAttempts = 0;
  let lastRecoveryAt = 0;

  onMount(() => {
    initPlayer();
    startHeartbeat();
    document.addEventListener('fullscreenchange', handleFullscreenChange);

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
      videoElement?.removeEventListener('ended', handleEnded);
      videoElement?.removeEventListener('loadedmetadata', showSubtitles);
      videoElement?.removeEventListener('playing', clearRecovering);
      videoElement?.removeEventListener('timeupdate', clearRecovering);
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
    dispatch('ended');
  }

  function clearRecovering() {
    if (recovering) recovering = null;
  }

  onDestroy(() => {
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
        backBufferLength: 90
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

  function toggleFullscreen() {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  }

  function handleFullscreenChange() {
    isFullscreen = !!document.fullscreenElement;
  }

  function handleKeydown(event) {
    switch (event.key) {
      case 'Escape':
        if (!isFullscreen) {
          handleClose();
        }
        break;
      case ' ':
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

<svelte:window on:keydown={handleKeydown} />

<div class="player">
  <!-- The video is the page. Chrome floats over it and gets out of the way. -->
  <header class="player__bar">
    <h2 class="player__title">{title}</h2>
    <div class="player__tools">
      <button class="glyph" on:click={toggleFullscreen} title="Fullscreen (F)" aria-label="Toggle fullscreen">
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

  video::cue {
    background: var(--scrim-strong);
    color: var(--color-ink);
    font-family: var(--font-body);
  }
</style>
