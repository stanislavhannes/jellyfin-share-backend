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
  let error = null;
  let isFullscreen = false;

  onMount(() => {
    initPlayer();
    startHeartbeat();
    document.addEventListener('fullscreenchange', handleFullscreenChange);

    // Fires on the element itself, so this covers both paths: hls.js feeding it
    // through MediaSource, and Safari playing the HLS natively - which is also
    // what AirPlay mirrors, so an episode finishing on an Apple TV lands here too.
    videoElement?.addEventListener('ended', handleEnded);
    videoElement?.addEventListener('loadedmetadata', showSubtitles);

    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
      videoElement?.removeEventListener('ended', handleEnded);
      videoElement?.removeEventListener('loadedmetadata', showSubtitles);
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
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              error = 'Network error - trying to recover...';
              hls.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              error = 'Media error - trying to recover...';
              hls.recoverMediaError();
              break;
            default:
              error = 'Playback error occurred';
              cleanup();
              break;
          }
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

<div class="player-container">
  <div class="player-header">
    <h2>{title}</h2>
    <button class="close-button" on:click={handleClose}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M18 6L6 18M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <div class="video-wrapper">
    {#if error}
      <div class="error-overlay">
        <p>{error}</p>
        <button on:click={handleClose}>Go Back</button>
      </div>
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

  <div class="player-controls">
    <button on:click={toggleFullscreen} title="Toggle fullscreen (F)">
      {#if isFullscreen}
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z"/>
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="currentColor">
          <path d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"/>
        </svg>
      {/if}
    </button>
  </div>
</div>

<style>
  .player-container {
    position: fixed;
    inset: 0;
    background: #000;
    z-index: 1000;
    display: flex;
    flex-direction: column;
  }

  .player-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    background: linear-gradient(to bottom, rgba(0,0,0,0.8) 0%, transparent 100%);
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 10;
  }

  .player-header h2 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0;
    color: #fff;
    text-shadow: 0 2px 4px rgba(0,0,0,0.5);
  }

  .close-button {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    border: none;
    background: rgba(255,255,255,0.1);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s;
  }

  .close-button:hover {
    background: rgba(255,255,255,0.2);
  }

  .close-button svg {
    width: 24px;
    height: 24px;
  }

  .video-wrapper {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
  }

  video {
    width: 100%;
    height: 100%;
    max-height: 100vh;
    background: #000;
  }

  .error-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: rgba(0,0,0,0.9);
    color: #ff6b6b;
    gap: 1rem;
  }

  .error-overlay button {
    padding: 0.75rem 1.5rem;
    background: #333;
    color: #fff;
    border: none;
    border-radius: 8px;
    cursor: pointer;
  }

  .player-controls {
    position: absolute;
    bottom: 80px;
    right: 1rem;
    z-index: 10;
  }

  .player-controls button {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    border: none;
    background: rgba(255,255,255,0.1);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s;
  }

  .player-controls button:hover {
    background: rgba(255,255,255,0.2);
  }

  .player-controls svg {
    width: 24px;
    height: 24px;
  }
</style>
