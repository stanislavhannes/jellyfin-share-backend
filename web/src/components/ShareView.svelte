<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import Player from './Player.svelte';
  import CastIcon from './CastIcon.svelte';
  import { initCast, ensureCastSession, loadOnCast, stopCast, describeCastError, onCastEnded,
           toBcp47, subtitlesDropped, castApiReady, castAvailable, castConnected,
           castDeviceName } from '../cast.js';

  export let shareInfo;
  export let token;

  const dispatch = createEventDispatcher();

  let needsPassword = shareInfo.requiresPassword;
  let passwordInput = '';
  let passwordError = '';
  let passwordLoading = false;

  $: showPasswordForm = needsPassword;
  $: isSeasonOrSeries = shareInfo.itemType === 'Season' || shareInfo.itemType === 'Series';

  let isPlaying = false;
  let playbackData = null;
  let playError = '';
  let imageLoaded = !shareInfo.posterUrl;
  let showFullCast = false;
  let currentPlayingTitle = '';
  // Which episode the browser player is on, so autoplay knows where it is.
  let currentEpisodeId = null;

  // The track the viewer picked, so the player can name it and switch it on.
  $: chosenSubtitle = (shareInfo.subtitleTracks || []).find((t) => t.index === selectedSubtitleIndex);

  // Episode list for Season/Series
  let episodes = [];
  let episodesLoading = false;
  let episodesError = '';

  onMount(async () => {
    initCast();
    onCastEnded(advanceCast);
    const timeout = setTimeout(() => {
      imageLoaded = true;
    }, 500);

    // Load episodes for Season/Series
    if (isSeasonOrSeries && !needsPassword) {
      await loadEpisodes();
    }

    return () => clearTimeout(timeout);
  });

  // Load episodes when password is validated
  $: if (isSeasonOrSeries && !needsPassword && episodes.length === 0 && !episodesLoading) {
    loadEpisodes();
  }

  async function loadEpisodes() {
    if (!isSeasonOrSeries) return;
    episodesLoading = true;
    episodesError = '';
    try {
      const response = await fetch(`/api/public/shares/${token}/episodes`, {
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        episodesError = data.error || 'Failed to load episodes';
        return;
      }
      const data = await response.json();
      episodes = data.episodes || [];
    } catch (e) {
      episodesError = 'Failed to load episodes';
    } finally {
      episodesLoading = false;
    }
  }

  function formatDuration(seconds) {
    if (!seconds) return '';
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
  }

  function formatExpiry(expiresAt) {
    if (!expiresAt) return null; // never expires
    const expiry = new Date(expiresAt);
    const now = new Date();
    const diff = expiry - now;
    if (diff <= 0) return 'Expired';
    const hours = Math.floor(diff / (1000 * 60 * 60));
    const days = Math.floor(hours / 24);
    if (days > 0) return `${days}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h`;
    return `${Math.floor(diff / (1000 * 60))}m`;
  }

  function formatRating(rating) {
    return rating.toFixed(1);
  }

  function getPlaysRemaining() {
    if (!shareInfo.maxTotalPlays) return null;
    return shareInfo.maxTotalPlays - shareInfo.totalPlays;
  }

  function getPlaysPercentage() {
    if (!shareInfo.maxTotalPlays) return 100;
    return ((shareInfo.maxTotalPlays - shareInfo.totalPlays) / shareInfo.maxTotalPlays) * 100;
  }

  async function submitPassword() {
    if (!passwordInput.trim()) {
      passwordError = 'Please enter a password';
      return;
    }
    passwordLoading = true;
    passwordError = '';
    try {
      const response = await fetch(`/api/public/shares/${token}/password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: passwordInput }),
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        passwordError = data.error || 'Incorrect password';
      } else {
        needsPassword = false;
        passwordInput = '';
      }
    } catch (e) {
      passwordError = 'Failed to validate password';
    } finally {
      passwordLoading = false;
    }
  }

  let selectedAudioIndex = null;
  let selectedSubtitleIndex = null;

  function trackLabel(t, fallback) {
    return t.displayTitle || t.language || `${fallback} ${t.index}`;
  }

  // Report what this browser can actually decode so the server can stream-copy a
  // matching source instead of re-encoding it. AV1 is never claimed: segments are
  // mpegts here, and AV1 in mpegts does not decode even where AV1 itself does.
  function supportedVideoCodecs() {
    const codecs = ['h264']; // baseline; every target browser decodes it
    try {
      const el = document.createElement('video');
      const supports = (type) =>
        (el.canPlayType && el.canPlayType(type) !== '') ||
        (window.MediaSource && MediaSource.isTypeSupported(type));
      if (supports('video/mp4; codecs="hvc1.1.6.L93.B0"') ||
          supports('video/mp4; codecs="hev1.1.6.L93.B0"')) {
        codecs.push('hevc');
      }
    } catch (e) {
      // Probing failed - h264 alone is always a safe answer
    }
    return codecs;
  }

  // The server validates and pins these; sending them is a request, not a command.
  // videoCodecs is overridden when casting: the receiver decides what it can
  // decode, not this browser. Everything else - notably the language hints, which
  // matter for an episode whose track list was probed on a different one - has to
  // travel on both paths alike.
  function trackQuery(videoCodecs) {
    const p = ['videoCodecs=' + (videoCodecs || supportedVideoCodecs().join(','))];
    if (selectedAudioIndex != null) {
      p.push('audioStreamIndex=' + selectedAudioIndex);
      // Send the language too: for a season or series the list was probed on one
      // episode, and stream N elsewhere is often a different language.
      const t = (shareInfo.audioTracks || []).find((x) => x.index === selectedAudioIndex);
      if (t?.language) p.push('audioLanguage=' + encodeURIComponent(t.language));
    }
    if (selectedSubtitleIndex != null) {
      p.push('subtitleStreamIndex=' + selectedSubtitleIndex);
      const t = (shareInfo.subtitleTracks || []).find((x) => x.index === selectedSubtitleIndex);
      if (t?.language) p.push('subtitleLanguage=' + encodeURIComponent(t.language));
    }
    return '?' + p.join('&');
  }

  let casting = false;
  let castHeartbeat = null;
  let castSessionId = null;
  // Which episode is on the receiver, so the list can mark it.
  let castingEpisodeId = null;

  // Releases the session server-side rather than leaving it to the stale-session
  // reaper, which would hold a concurrent-viewer slot for the timeout's duration.
  async function endCastSession() {
    stopCast();
    castingEpisodeId = null;
    await finishCastSession();
  }

  // Releases the session but leaves the device connected, so the next episode can
  // start without opening the picker again.
  async function finishCastSession() {
    stopCastHeartbeat();
    const id = castSessionId;
    castSessionId = null;
    if (!id) return;
    try {
      await fetch(`/api/public/sessions/${id}/finish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
        credentials: 'include'
      });
    } catch (e) {
      console.warn('Could not finish cast session', e);
    }
  }

  // While casting, the receiver plays and the Player component is never mounted,
  // so nothing is sending heartbeats. Without them the session goes stale after
  // JFSHARE_SESSION_HEARTBEAT_TIMEOUT_SECONDS - two minutes by default - and the
  // stream proxy starts answering 403, killing playback mid-show.
  function startCastHeartbeat(sessionId) {
    stopCastHeartbeat();
    castHeartbeat = setInterval(async () => {
      try {
        const response = await fetch(`/api/public/sessions/${sessionId}/heartbeat`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
          credentials: 'include'
        });
        if (!response.ok) stopCastHeartbeat();
      } catch (e) {
        // A transient network blip should not end the session; the next tick retries.
        console.warn('Cast heartbeat failed', e);
      }
    }, 15000);
  }

  function stopCastHeartbeat() {
    if (castHeartbeat) {
      clearInterval(castHeartbeat);
      castHeartbeat = null;
    }
  }

  onDestroy(stopCastHeartbeat);

  // Stop once the receiver is gone, so a finished cast does not keep a session
  // alive and occupying a concurrent-viewer slot.
  $: if (!$castConnected) stopCastHeartbeat();

  // h264 is requested outright rather than negotiated: every Cast device decodes
  // it, where HEVC depends on the model. Going through /play once keeps this to a
  // single play against the share's limit.
  async function castItem({ playPath, title, subtitle, durationSeconds, episodeId = null }) {
    playError = '';
    casting = true;
    try {
      // Pick the device first. /play counts against the share's play limit, so it
      // must not run if the viewer dismisses the picker.
      await ensureCastSession();

      // A receiver plays one thing at a time. Switching episodes without releasing
      // the previous session would leave it holding a concurrent-viewer slot until
      // the stale-session reaper gets to it.
      await finishCastSession();

      const response = await fetch(playPath, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = data.error || 'Failed to start playback';
        return;
      }
      const data = await response.json();
      const chosen = (shareInfo.subtitleTracks || []).find((t) => t.index === selectedSubtitleIndex);
      await loadOnCast({
        url: data.playbackUrl,
        subtitleUrl: data.subtitleUrl,
        subtitleLanguage: chosen?.language,
        title,
        subtitle,
        posterUrl: shareInfo.posterUrl,
        durationSeconds
      });
      castSessionId = data.sessionId;
      castingEpisodeId = episodeId;
      startCastHeartbeat(data.sessionId);
    } catch (e) {
      // A dismissed picker is not a failure.
      playError = describeCastError(e) || '';
    } finally {
      casting = false;
    }
  }

  function startCast() {
    return castItem({
      playPath: `/api/public/shares/${token}/play${trackQuery('h264')}`,
      title: shareInfo.title,
      subtitle: shareInfo.year ? String(shareInfo.year) : '',
      durationSeconds: shareInfo.runtimeSeconds
    });
  }

  function castEpisode(episode, { continues } = {}) {
    return castItem({
      playPath: episodePlayPath(episode.id, { codec: 'h264', continues }),
      title: episode.name,
      subtitle: `${episodeLabel(episode)} \u00b7 ${shareInfo.title}`,
      durationSeconds: episode.runtimeSeconds,
      episodeId: episode.id
    });
  }

  // A season or series has no single Play button to sit beside, so connecting is
  // its own step. It costs nothing: /play only runs once an episode is picked.
  async function connectCast() {
    playError = '';
    casting = true;
    try {
      await ensureCastSession();
    } catch (e) {
      playError = describeCastError(e) || '';
    } finally {
      casting = false;
    }
  }

  function episodeLabel(episode) {
    return episode.seasonNumber
      ? `S${episode.seasonNumber}E${episode.indexNumber || '?'}`
      : `Episode ${episode.indexNumber || '?'}`;
  }

  // The list is already in running order, so "next" is simply the row below.
  function nextEpisode(afterId) {
    const i = episodes.findIndex((e) => e.id === afterId);
    return i >= 0 && i + 1 < episodes.length ? episodes[i + 1] : null;
  }

  function episodePlayPath(episodeId, { codec, continues } = {}) {
    let query = trackQuery(codec);
    // Tells the backend this carries on a viewing it already charged for, so a
    // series link offered as "three plays" means three viewings rather than
    // three episodes. The backend decides whether to honour it.
    if (continues) query += '&continues=' + continues;
    return `/api/public/shares/${token}/episodes/${episodeId}/play${query}`;
  }

  async function finishSession(sessionId) {
    if (!sessionId) return;
    try {
      await fetch(`/api/public/sessions/${sessionId}/finish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
        credentials: 'include'
      });
    } catch (e) {
      console.warn('Could not finish session', e);
    }
  }

  // Autoplay in the browser - and with it AirPlay, which mirrors the same element,
  // so an episode finishing on an Apple TV arrives here as well.
  async function handlePlaybackEnded() {
    // A single item has nothing to advance to; leave the player as it was.
    if (!isSeasonOrSeries) return;

    const next = nextEpisode(currentEpisodeId);
    if (!next) {
      handlePlayerClose();
      return;
    }

    // Release the finished session before asking for the next one. A share
    // limited to one concurrent viewer would otherwise refuse its own sequel.
    const previous = playbackData?.sessionId;
    await finishSession(previous);

    try {
      const response = await fetch(episodePlayPath(next.id, { continues: previous }), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = data.error || 'Failed to start the next episode';
        handlePlayerClose();
        return;
      }
      playbackData = await response.json();
      currentEpisodeId = next.id;
      currentPlayingTitle = `E${next.indexNumber}: ${next.name}`;
    } catch (e) {
      playError = 'Failed to connect to server';
      handlePlayerClose();
    }
  }

  // Autoplay on the receiver. Reading castSessionId before castEpisode runs
  // matters: it releases the old session and clears the field on the way.
  async function advanceCast() {
    const next = nextEpisode(castingEpisodeId);
    console.debug('Cast: episode finished', castingEpisodeId, '-> next', next?.id || 'none');
    if (!next) {
      await endCastSession();
      return;
    }
    await castEpisode(next, { continues: castSessionId });
  }

  async function startPlayback() {
    playError = '';
    try {
      const response = await fetch(`/api/public/shares/${token}/play${trackQuery()}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = data.error || 'Failed to start playback';
        return;
      }
      playbackData = await response.json();
      currentPlayingTitle = shareInfo.title;
      currentEpisodeId = null;
      isPlaying = true;
    } catch (e) {
      playError = 'Failed to connect to server';
    }
  }

  async function startEpisodePlayback(episode) {
    playError = '';
    try {
      const response = await fetch(`/api/public/shares/${token}/episodes/${episode.id}/play${trackQuery()}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = data.error || 'Failed to start playback';
        return;
      }
      playbackData = await response.json();
      currentPlayingTitle = `E${episode.indexNumber}: ${episode.name}`;
      currentEpisodeId = episode.id;
      isPlaying = true;
    } catch (e) {
      playError = 'Failed to connect to server';
    }
  }

  function handlePlayerClose() {
    isPlaying = false;
    playbackData = null;
    currentPlayingTitle = '';
    currentEpisodeId = null;
  }

  function handleImageLoad() {
    imageLoaded = true;
  }

  // A poster that 404s should leave a gap, not paint its alt text across the
  // layout. The figure is dropped and the stage reflows to one column.
  let posterFailed = false;

  function handleImageError() {
    posterFailed = true;
    imageLoaded = true;
  }

  const absoluteTime = new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZoneName: 'short'
  });

  // "in 2 d 3 h" answers "is this still worth opening"; the exact local moment
  // answers "when do I lose it". The page shows both, in different places.
  function formatAbsolute(dateStr) {
    const d = new Date(dateStr);
    return Number.isNaN(d.getTime()) ? '' : absoluteTime.format(d);
  }

  // ---- Presentation: the fold's caption is a list of specifics, not badges. ----
  $: metaBits = [
    shareInfo.year,
    shareInfo.runtimeSeconds ? formatDuration(shareInfo.runtimeSeconds) : null,
    shareInfo.officialRating,
    shareInfo.videoQuality?.resolution,
    shareInfo.communityRating > 0 ? `${formatRating(shareInfo.communityRating)} / 10 users` : null,
    shareInfo.criticRating > 0 ? `${shareInfo.criticRating}% critics` : null
  ].filter(Boolean);

  $: techBits = shareInfo.videoQuality
    ? [
        `${shareInfo.videoQuality.width}\u00d7${shareInfo.videoQuality.height}`,
        shareInfo.videoQuality.codec?.toUpperCase(),
        shareInfo.videoQuality.audioCodec?.toUpperCase()
      ].filter(Boolean)
    : [];

  // Long titles step down a size rung rather than crowding themselves.
  $: titleIsLong = (shareInfo.title || '').length > 32;

  $: foldImage = shareInfo.backdropUrl || shareInfo.posterUrl || '';
</script>
<div class="share">
  {#if isPlaying && playbackData}
    <!-- Keyed on the session: autoplay swaps in the next episode, and the player
         has to be rebuilt around it rather than handed a new URL mid-flight. -->
    {#key playbackData.sessionId}
      <Player {playbackData} title={currentPlayingTitle || shareInfo.title}
              subtitleLabel={chosenSubtitle ? trackLabel(chosenSubtitle, 'Subtitle') : 'Subtitles'}
              subtitleLanguage={toBcp47(chosenSubtitle?.language)}
              on:close={handlePlayerClose} on:ended={handlePlaybackEnded} />
    {/key}
  {:else}
    <!-- ── The photographic fold. The artwork fills it; the type is annotation. ── -->
    <section class="fold" class:fold--bare={!foldImage}>
      {#if foldImage}
        <img class="fold__image" src={foldImage} alt="" fetchpriority="high" />
        <div class="fold__scrim"></div>
      {/if}

      <!-- N9 · Edge-aligned minimal. Wordmark left, the one status a recipient
           needs right, nothing in between. -->
      <header class="nav">
        <span class="wordmark">Jellyfin Share</span>
        <span class="nav__expiry">
          {#if shareInfo.expiresAt}
            <time datetime={shareInfo.expiresAt} title={formatAbsolute(shareInfo.expiresAt)}>
              Expires in {formatExpiry(shareInfo.expiresAt)}
            </time>
          {:else}
            No expiry
          {/if}
        </span>
      </header>

      <div class="plate">
        {#if shareInfo.logoUrl}
          <img class="plate__logo" src="{shareInfo.logoUrl}?maxWidth=500" alt={shareInfo.title} />
        {:else}
          <h1 class="plate__title" class:plate__title--long={titleIsLong}>{shareInfo.title}</h1>
        {/if}

        {#if metaBits.length}
          <p class="caption">{metaBits.join(' · ')}</p>
        {/if}
        {#if shareInfo.genres && shareInfo.genres.length > 0}
          <p class="caption caption--quiet">{shareInfo.genres.slice(0, 4).join(' · ')}</p>
        {/if}
      </div>
    </section>

    <!-- ── The print, laid over the photograph's edge, and the one action. ── -->
    <section class="stage" class:stage--noprint={!shareInfo.posterUrl || posterFailed}>
      {#if shareInfo.posterUrl && !posterFailed}
        <figure class="print">
          <img
            src="{shareInfo.posterUrl}?maxWidth=400"
            alt={shareInfo.title}
            class:is-loaded={imageLoaded}
            on:load={handleImageLoad}
            on:error={handleImageError}
          />
          {#if shareInfo.videoQuality}
            <figcaption>{shareInfo.videoQuality.resolution}</figcaption>
          {/if}
        </figure>
      {/if}

      <div class="act">
        {#if shareInfo.overview}
          <p class="lede">{shareInfo.overview}</p>
        {/if}

        {#if (shareInfo.audioTracks && shareInfo.audioTracks.length > 1) || (shareInfo.subtitleTracks && shareInfo.subtitleTracks.length > 0)}
          <div class="tracks">
            {#if shareInfo.audioTracks && shareInfo.audioTracks.length > 1}
              <div class="field">
                <label for="audio-select">Audio</label>
                <select id="audio-select" bind:value={selectedAudioIndex}>
                  <option value={null}>Default</option>
                  {#each shareInfo.audioTracks as track}
                    <option value={track.index}>{trackLabel(track, 'Track')}</option>
                  {/each}
                </select>
              </div>
            {/if}
            {#if shareInfo.subtitleTracks && shareInfo.subtitleTracks.length > 0}
              <div class="field">
                <label for="subtitle-select">Subtitles</label>
                <select id="subtitle-select" bind:value={selectedSubtitleIndex}>
                  <option value={null}>None</option>
                  {#each shareInfo.subtitleTracks as track}
                    <option value={track.index}>{trackLabel(track, 'Subtitle')}</option>
                  {/each}
                </select>
              </div>
            {/if}
          </div>
        {/if}

        {#if showPasswordForm}
          <form class="gate" on:submit|preventDefault={submitPassword} novalidate>
            <div class="field">
              <label for="share-password">Password</label>
              <div class="gate__row">
                <input
                  id="share-password"
                  type="password"
                  bind:value={passwordInput}
                  placeholder="Set by whoever shared this"
                  disabled={passwordLoading}
                  aria-invalid={passwordError ? 'true' : 'false'}
                  aria-describedby="share-password-help"
                />
                <button type="submit" class="btn-primary" disabled={passwordLoading}>
                  {#if passwordLoading}
                    <span class="meter meter--inline" aria-hidden="true"></span>
                    <span>Checking</span>
                  {:else}
                    <span>Unlock</span>
                  {/if}
                </button>
              </div>
              <p class="field__help" id="share-password-help" class:field__help--error={passwordError}>
                {passwordError || 'This link is password protected.'}
              </p>
            </div>
          </form>
        {:else if isSeasonOrSeries}
          <div class="episodes">
            <h2 class="episodes__head">
              Episodes
              {#if episodes.length > 0}<span class="episodes__n">{episodes.length}</span>{/if}
            </h2>

            {#if $castApiReady && episodes.length > 0}
              <div class="castbar">
                {#if $castConnected}
                  <p class="castbar__state">
                    Connected to <strong>{$castDeviceName}</strong> — pick an episode to play it there.
                  </p>
                  <button class="btn-ghost" on:click={endCastSession}>
                    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                      <rect x="6" y="6" width="12" height="12" rx="1.5" />
                    </svg>
                    <span>Stop casting</span>
                  </button>
                {:else}
                  <button class="btn-ghost" on:click={connectCast} disabled={casting}>
                    <CastIcon />
                    <span>{$castAvailable ? 'Cast to a TV' : 'Looking for devices'}</span>
                  </button>
                {/if}
              </div>
            {/if}

            {#if episodesLoading}
              <div class="state"><div class="meter"></div><p>Loading episodes</p></div>
            {:else if episodesError}
              <p class="msg msg--error">{episodesError}</p>
            {:else if episodes.length === 0}
              <p class="msg">No episodes in this share.</p>
            {:else}
              <ul class="eplist">
                {#each episodes as episode}
                  <li>
                    <button class="ep"
                            class:ep--casting={$castConnected && castingEpisodeId === episode.id}
                            disabled={casting}
                            on:click={() => ($castConnected ? castEpisode(episode) : startEpisodePlayback(episode))}>
                      <span class="ep__no">
                        {#if episode.seasonNumber}S{episode.seasonNumber}E{episode.indexNumber || '?'}{:else}{episode.indexNumber || '?'}{/if}
                      </span>
                      <span class="ep__name">{episode.name}</span>
                      <span class="ep__end">
                        {#if episode.runtimeSeconds}
                          <span class="ep__len">{formatDuration(episode.runtimeSeconds)}</span>
                        {/if}
                        {#if $castConnected}
                          <CastIcon title="Plays on {$castDeviceName}" />
                        {/if}
                      </span>
                    </button>
                  </li>
                {/each}
              </ul>
            {/if}

            {#if playError}<p class="msg msg--error">{playError}</p>{/if}
            {#if $subtitlesDropped}
              <p class="msg">Casting without subtitles — the receiver would not take the track.</p>
            {/if}
          </div>
        {:else}
          <div class="play">
            <button class="btn-primary btn-primary--lg" on:click={startPlayback}>
              <svg viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.75" stroke-linejoin="round" aria-hidden="true"><polygon points="6 3 20 12 6 21 6 3"/></svg>
              <span>Play</span>
            </button>

            {#if $castApiReady}
              <button class="btn-ghost" on:click={startCast} disabled={casting}>
                <CastIcon />
                <span>{casting ? 'Casting' : ($castConnected ? $castDeviceName : ($castAvailable ? 'Cast to a TV' : 'Looking for devices'))}</span>
              </button>
              {#if $castConnected}
                <button class="btn-ghost" on:click={endCastSession}>
                  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                    <rect x="6" y="6" width="12" height="12" rx="1.5" />
                  </svg>
                  <span>Stop</span>
                </button>
              {/if}
            {/if}
          </div>

          {#if shareInfo.maxTotalPlays}
            <!-- The play budget as a line under the button, not a widget. -->
            <div class="fuel">
              <div class="fuel__track">
                <div class="fuel__fill"
                     class:fuel__fill--low={getPlaysPercentage() <= 33}
                     style="transform: scaleX({getPlaysPercentage() / 100})"></div>
              </div>
              <p class="fuel__label">{getPlaysRemaining()} of {shareInfo.maxTotalPlays} plays left</p>
            </div>
          {/if}

          {#if playError}<p class="msg msg--error">{playError}</p>{/if}
          {#if $subtitlesDropped}
            <p class="msg">Casting without subtitles — the receiver would not take the track.</p>
          {/if}
        {/if}
      </div>
    </section>

    <!-- ── Credits. A spec list, not a card. ── -->
    {#if (shareInfo.directors && shareInfo.directors.length) || (shareInfo.studios && shareInfo.studios.length) || (shareInfo.actors && shareInfo.actors.length) || techBits.length}
      <section class="credits">
        <dl>
          {#if shareInfo.directors && shareInfo.directors.length > 0}
            <div>
              <dt>Direction</dt>
              <dd>{shareInfo.directors.join(', ')}</dd>
            </div>
          {/if}
          {#if shareInfo.studios && shareInfo.studios.length > 0}
            <div>
              <dt>Studio</dt>
              <dd>{shareInfo.studios[0]}</dd>
            </div>
          {/if}
          {#if shareInfo.actors && shareInfo.actors.length > 0}
            <div>
              <dt>Cast</dt>
              <dd>
                <ul class="cast">
                  {#each (showFullCast ? shareInfo.actors : shareInfo.actors.slice(0, 6)) as actor}
                    <li>
                      <span class="cast__name">{actor.name}</span>
                      {#if actor.role}<span class="cast__role">{actor.role}</span>{/if}
                    </li>
                  {/each}
                </ul>
                {#if shareInfo.actors.length > 6}
                  <button class="link" on:click={() => (showFullCast = !showFullCast)}>
                    {showFullCast ? 'Show fewer' : `Show all ${shareInfo.actors.length}`}
                  </button>
                {/if}
              </dd>
            </div>
          {/if}
          {#if techBits.length}
            <div>
              <dt>File</dt>
              <dd class="mono">{techBits.join(' · ')}</dd>
            </div>
          {/if}
        </dl>
      </section>
    {/if}

    <!-- Ft2 · Inline single line -->
    <footer class="foot">
      <p>
        Shared via <a class="foot-link" href="https://github.com/stanislavhannes/jellyfin-share-backend" target="_blank" rel="noopener noreferrer">Jellyfin Share</a>{#if shareInfo.maxTotalPlays}{' · '}{getPlaysRemaining()} of {shareInfo.maxTotalPlays} plays left{/if}{' · '}
        {#if shareInfo.expiresAt}link expires <time datetime={shareInfo.expiresAt}>{formatAbsolute(shareInfo.expiresAt)}</time>{:else}link does not expire{/if}
      </p>
    </footer>
  {/if}
</div>

<style>
  /* Hallmark · genre: atmospheric · macrostructure: 08 Photographic
   * H6 photographic fold · nav: N9 edge-aligned · footer: Ft2 inline
   * design-system: design.md · designed-as-app
   */

  .share { min-height: 100dvh; }

  /* ── The fold ──────────────────────────────────────────────────────────
     The artwork is the page above the break. The type is annotation laid in
     the bottom-left corner, and the ground fades into paper so the poster
     below can lap over the edge without a seam. */
  .fold {
    position: relative;
    display: grid;
    grid-template-rows: auto 1fr;
    min-height: clamp(19rem, 56dvh, 30rem);
    overflow: hidden;
  }

  .fold--bare {
    min-height: 0;
    background: var(--color-paper-2);
  }

  .fold__image {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    object-position: center 28%;
  }

  .fold__scrim {
    position: absolute;
    inset: 0;
    /* The top band holds at full strength past the nav row before it fades: a
       backdrop can be bright (or white), and the wordmark and expiry have to
       stay legible over any frame Jellyfin hands us, not just a dark one. */
    background:
      linear-gradient(to bottom, var(--scrim-strong) 0%, var(--scrim-strong) 9%, transparent 34%),
      linear-gradient(to top, var(--color-paper) 2%, var(--scrim-strong) 34%, transparent 72%);
  }

  .fold > .nav,
  .fold > .plate { position: relative; z-index: var(--z-base); }

  /* N9 · Edge-aligned minimal */
  .nav {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    padding: var(--space-lg) var(--page-gutter);
    padding-top: max(var(--space-lg), env(safe-area-inset-top));
  }

  .wordmark {
    font-family: var(--font-display);
    font-size: var(--text-md);
    letter-spacing: -0.01em;
    color: var(--color-ink);
    white-space: nowrap;
  }

  .nav__expiry,
  .nav__expiry time {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    color: var(--color-ink-2);
    white-space: nowrap;
  }

  .plate {
    align-self: end;
    max-width: min(40rem, 100%);
    padding: var(--space-2xl) var(--page-gutter);
  }

  .plate__logo {
    width: auto;
    max-width: min(22rem, 78%);
    max-height: 6rem;
    object-fit: contain;
    display: block;
    margin-bottom: var(--space-sm);
  }

  .plate__title {
    font-size: var(--text-display);
    overflow-wrap: anywhere;
    min-width: 0;
    margin-bottom: var(--space-sm);
  }

  .plate__title--long { font-size: var(--text-display-s); }

  .caption {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    line-height: 1.6;
    color: var(--color-ink-2);
  }

  .caption--quiet {
    font-family: var(--font-body);
    font-size: var(--text-sm);
    letter-spacing: 0.04em;
    color: var(--color-muted);
  }

  /* ── The stage: the print and the one action ───────────────────────────── */
  .stage {
    /* The fold's image and scrim are absolutely positioned, so they paint above
       a later, un-positioned sibling. The stage laps up over the fold's edge —
       it has to be positioned too, or the overlap swallows its first rows. */
    position: relative;
    z-index: var(--z-raised);
    display: grid;
    gap: var(--space-xl);
    padding: var(--space-lg) var(--page-gutter) var(--space-xl);
  }

  .print {
    display: none;
    position: relative;
    margin: 0;
  }

  .print img {
    display: block;
    width: 100%;
    aspect-ratio: 2 / 3;
    object-fit: cover;
    border-radius: var(--radius-card);
    opacity: 0;
    transition: opacity var(--dur-long) var(--ease-out);
  }

  .print img.is-loaded { opacity: 1; }

  .print figcaption {
    position: absolute;
    inset: auto auto var(--space-xs) var(--space-xs);
    padding: var(--space-3xs) var(--space-xs);
    background: var(--color-paper);
    color: var(--color-ink-2);
    font-family: var(--font-outlier);
    font-size: var(--text-xs);
    letter-spacing: 0.08em;
    border-radius: var(--radius-pill);
  }

  .stage--noprint { grid-template-columns: minmax(0, 1fr); }

  .act {
    display: grid;
    gap: var(--space-lg);
    justify-items: start;
    min-width: 0;
  }

  .lede {
    max-width: var(--measure);
    line-height: 1.65;
    color: var(--color-ink-2);
  }

  /* ── Controls ──────────────────────────────────────────────────────────── */
  .tracks {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-md);
    width: 100%;
  }

  .field {
    display: grid;
    gap: var(--space-2xs);
    min-width: 0;
  }

  label {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--color-ink-2);
  }

  select, input {
    height: var(--control-h);
    max-width: 100%;
    padding-inline: var(--space-sm);
    background: var(--color-paper-2);
    color: var(--color-ink);
    /* 1px in every state — the box never shifts on focus or error. */
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius-input);
    outline: 2px solid transparent;
    outline-offset: 1px;
    transition: background-color var(--dur-micro) var(--ease-out),
                border-color var(--dur-micro) var(--ease-out);
  }

  select { min-width: 11rem; cursor: pointer; }
  input { font-family: var(--font-outlier); min-width: 0; flex: 1 1 11rem; }
  input::placeholder { color: var(--color-neutral); }

  @media (hover: hover) {
    select:hover, input:hover { background: var(--color-paper-3); }
  }

  select:focus-visible, input:focus-visible {
    outline-color: var(--color-focus);
    border-color: var(--color-ink-2);
  }

  input[aria-invalid="true"] { border-color: var(--color-danger); }
  select:disabled, input:disabled { opacity: 0.55; cursor: not-allowed; }

  .gate { width: 100%; max-width: 30rem; }

  .gate__row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-xs);
  }

  .field__help {
    min-height: 1lh;
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .field__help--error { color: var(--color-danger); }

  /* ── Buttons ───────────────────────────────────────────────────────────── */
  .btn-primary, .btn-ghost {
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

  .btn-primary {
    background: var(--color-accent);
    color: var(--color-accent-ink);
    border: none;
  }

  .btn-primary--lg {
    height: 3.25rem;
    padding-inline: var(--space-xl);
    font-size: var(--text-md);
  }

  .btn-primary svg { width: 1.15rem; height: 1.15rem; }

  /* Icons never shrink the label or get squeezed by it. */
  .btn-ghost :global(svg),
  .btn-ghost svg { width: 1.05rem; height: 1.05rem; flex-shrink: 0; }

  .btn-ghost {
    background: none;
    color: var(--color-ink-2);
    border: var(--rule-hair) solid var(--color-rule);
  }

  @media (hover: hover) {
    .btn-primary:hover:not(:disabled) { transform: translateY(-1px); }
    .btn-ghost:hover:not(:disabled) { background: var(--color-paper-2); color: var(--color-ink); }
  }

  .btn-primary:active:not(:disabled), .btn-ghost:active:not(:disabled) { transform: translateY(1px); }

  /* Ring clears the accent fill so it has paper to contrast against. */
  .btn-primary:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 3px; }

  .btn-primary:disabled, .btn-ghost:disabled { opacity: 0.55; cursor: not-allowed; transform: none; }

  .play {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-sm);
  }

  .link {
    margin-top: var(--space-xs);
    white-space: nowrap;
    padding: 0;
    background: none;
    color: var(--color-accent);
    font-size: var(--text-sm);
    font-weight: 600;
    border: none;
    border-bottom: var(--rule-hair) solid var(--color-accent-dim);
    cursor: pointer;
  }

  @media (hover: hover) {
    .link:hover { border-bottom-color: var(--color-accent); }
  }

  .link:active { color: var(--color-ink); }

  .link:disabled { opacity: 0.55; cursor: not-allowed; }

  /* ── The play budget: a line under the button, not a widget ────────────── */
  .fuel {
    display: grid;
    gap: var(--space-xs);
    width: 100%;
    max-width: 18rem;
  }

  .fuel__track { height: 2px; background: var(--color-rule); }

  .fuel__fill {
    width: 100%;
    height: 100%;
    background: var(--color-accent);
    transform-origin: left center;
    transition: transform var(--dur-long) var(--ease-out);
  }

  .fuel__fill--low { background: var(--color-danger); }

  .fuel__label {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    color: var(--color-muted);
  }

  /* ── Episodes: an index, same voice as the admin index ─────────────────── */
  .episodes { width: 100%; }

  .episodes__head {
    overflow-wrap: anywhere;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: var(--space-xs);
    font-size: var(--text-lg);
    margin-bottom: var(--space-md);
  }

  .episodes__n {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .castbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-sm);
    margin-bottom: var(--space-md);
  }

  .castbar__state {
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .castbar__state strong { color: var(--color-accent); font-weight: 600; }

  .eplist { list-style: none; margin: 0; padding: 0; width: 100%; }

  .ep {
    display: grid;
    grid-template-columns: 4.25rem minmax(0, 1fr) auto;
    gap: var(--space-md);
    align-items: baseline;
    width: 100%;
    padding: var(--space-md) 0;
    text-align: start;
    background: none;
    border: none;
    border-bottom: var(--rule-hair) solid var(--color-rule-2);
    cursor: pointer;
    transition: transform var(--dur-micro) var(--ease-out);
  }

  @media (hover: hover) {
    .ep:hover:not(:disabled) { transform: translateY(-1px); }
    .ep:hover:not(:disabled) .ep__name { color: var(--color-accent); }
  }

  .ep:active:not(:disabled) { transform: translateY(1px); }

  .ep:disabled { opacity: 0.55; cursor: not-allowed; }

  .ep--casting .ep__no { color: var(--color-accent); }

  .ep__no {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    letter-spacing: 0.06em;
    color: var(--color-muted);
  }

  .ep__name {
    font-weight: 600;
    line-height: 1.35;
    color: var(--color-ink);
    overflow-wrap: anywhere;
    transition: color var(--dur-micro) var(--ease-out);
  }

  .ep__end {
    display: inline-flex;
    align-items: center;
    gap: var(--space-xs);
    color: var(--color-neutral);
  }

  .ep__end :global(svg) { width: 1rem; height: 1rem; flex-shrink: 0; }

  /* While a receiver is connected the glyph is the row's promise, so it takes
     the accent rather than sitting in the quiet grey. */
  .ep--casting .ep__end,
  .eplist .ep:not(:disabled) .ep__end :global(svg) { color: var(--color-accent); }

  .ep__len {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    color: var(--color-neutral);
    white-space: nowrap;
  }

  /* ── Credits ───────────────────────────────────────────────────────────── */
  .credits {
    padding: var(--space-xl) var(--page-gutter) var(--space-xl);
  }

  .credits dl { display: grid; gap: var(--space-lg); margin: 0; }

  .credits dl > div {
    display: grid;
    gap: var(--space-2xs);
    padding-bottom: var(--space-md);
    border-bottom: var(--rule-hair) solid var(--color-rule-2);
  }

  dt {
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--color-neutral);
  }

  dd { margin: 0; min-width: 0; color: var(--color-ink-2); overflow-wrap: anywhere; }

  .mono { font-family: var(--font-outlier); font-variant-numeric: tabular-nums; }

  .cast {
    list-style: none;
    display: grid;
    gap: var(--space-xs);
    margin: 0;
    padding: 0;
  }

  .cast__name { display: block; color: var(--color-ink); }

  .cast__role {
    display: block;
    font-size: var(--text-sm);
    color: var(--color-neutral);
  }

  /* ── Messages and loading ──────────────────────────────────────────────── */
  .msg { font-size: var(--text-sm); color: var(--color-muted); }
  .msg--error { color: var(--color-danger); }

  .state {
    display: grid;
    gap: var(--space-sm);
    justify-items: start;
    padding-block: var(--space-lg);
    color: var(--color-muted);
    font-size: var(--text-sm);
  }

  .meter {
    width: 7.5rem;
    height: 2px;
    background: var(--color-rule);
    overflow: hidden;
  }

  .meter::after {
    content: "";
    display: block;
    width: 40%;
    height: 100%;
    background: var(--color-accent);
    animation: sweep 1.1s var(--ease-in-out) infinite;
  }

  .meter--inline {
    width: 1.5rem;
    background: var(--color-accent-ink);
  }

  .meter--inline::after { background: var(--color-accent-ink); }

  @keyframes sweep {
    0%   { transform: translateX(-110%); }
    100% { transform: translateX(260%); }
  }

  @media (prefers-reduced-motion: reduce) {
    .meter::after { width: 100%; }
  }

  /* ── Ft2 · Inline single line ──────────────────────────────────────────── */
  .foot {
    padding: var(--space-md) var(--page-gutter) var(--space-lg);
    padding-bottom: max(var(--space-lg), env(safe-area-inset-bottom));
    border-top: var(--rule-hair) solid var(--color-rule-2);
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    line-height: 1.6;
    color: var(--color-neutral);
  }

  /* ── Wider: the print laps over the photograph's edge ──────────────────── */
  @media (min-width: 40rem) {
    .plate { max-width: min(40rem, 68%); }

    .stage {
      grid-template-columns: minmax(0, 1fr) 11rem;
      align-items: start;
      padding-top: var(--space-lg);
    }

    .print {
      display: block;
      grid-column: 2;
      grid-row: 1;
      width: 11rem;
      margin-top: -7rem;   /* the print alone laps the fold's edge */
    }
    .act { grid-column: 1; grid-row: 1; }

    .credits dl > div {
      grid-template-columns: 8.5rem minmax(0, 1fr);
      gap: var(--space-md);
      align-items: baseline;
    }

    .cast { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-sm) var(--space-md); }
  }

  @media (min-width: 60rem) {
    .stage {
      grid-template-columns: minmax(0, 1fr) 13.5rem;
      gap: var(--space-2xl);
      padding-bottom: var(--space-xl);
    }

    .print { width: 13.5rem; margin-top: -9rem; }

    .credits { padding-block: var(--space-2xl); }

    .cast { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  }
</style>
