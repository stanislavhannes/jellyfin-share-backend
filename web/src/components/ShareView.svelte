<script>
  import { onMount, onDestroy, tick, createEventDispatcher } from 'svelte';
  import Player from './Player.svelte';
  import CastIcon from './CastIcon.svelte';
  import PlayIcon from './PlayIcon.svelte';
  import DownloadIcon from './DownloadIcon.svelte';
  import CheckIcon from './CheckIcon.svelte';
  import BrandMark from './BrandMark.svelte';
  import { initCast, ensureCastSession, loadOnCast, stopCast, describeCastError, onCastEnded,
           toBcp47, languageName, subtitlesDropped, castApiReady, castConnected,
           castDeviceName, castProgress, tvHint } from '../cast.js';
  import { MAIN, emptyProgress, loadProgress, noteProgress, resumePosition, seriesResumeTarget,
           watchedFraction, isWatched, formatClock } from '../progress.js';
  import { fullscreenElement, exitFullscreen } from '../fullscreen.js';

  export let shareInfo;
  export let token;

  const dispatch = createEventDispatcher();

  // `locked` is the server's answer for this browser: a password link stays locked
  // until its password has been entered here, and until then carries no details.
  let needsPassword = shareInfo.locked ?? shareInfo.requiresPassword;
  let passwordInput = '';
  let passwordError = '';
  let passwordLoading = false;
  let passwordField;
  // A phone keyboard makes a hidden password easy to mistype.
  let showPassword = false;
  // Where focus goes once the link opens: the button that starts watching.
  let playButton;
  // Where it goes instead when the player closes on a link with nothing left.
  let spentNote;
  let termsHead;
  let focusPlayWhenReady = false;

  $: showPasswordForm = needsPassword;
  $: isSeasonOrSeries = shareInfo.itemType === 'Season' || shareInfo.itemType === 'Series';

  let isPlaying = false;
  let playbackData = null;
  // The player replaces the page, footer included, while this holds.
  $: playerOpen = Boolean(isPlaying && playbackData);
  let playError = '';
  let imageLoaded = !shareInfo.posterUrl;
  let showFullCast = false;
  let currentPlayingTitle = '';
  // Which episode the browser player is on, so autoplay knows where it is.
  let currentEpisodeId = null;
  // Where the player starts, when resuming.
  let startPosition = 0;

  // ---- Where the link's viewers left off (kept on the server; see progress.js) ----
  let progress = emptyProgress();
  let progressRequested = false;
  // Bumped by every update the page makes itself, so a server reply that set
  // off before one of them cannot set the page back when it lands.
  let progressVersion = 0;
  // The first read. A start waits for it: starting before it would begin at 0,
  // and the next heartbeat would overwrite the position the link was left at.
  let firstProgress = null;
  // Whether a reply from the server has been taken in yet. Until then the page's
  // own copy holds only what was played here, which is newer than anything the
  // server could say about those items.
  let progressLoaded = false;

  function recordProgress(key, position, duration) {
    progressVersion += 1;
    progress = noteProgress(progress, key, position, duration);
  }

  // Read on open, or once the password is in - a locked link answers nothing.
  $: if (!needsPassword && !progressRequested) {
    progressRequested = true;
    firstProgress = refreshProgress();
  }

  // Never while the player is open: the page's own copy is ahead of the
  // server's there.
  async function refreshProgress() {
    const version = progressVersion;
    const record = await loadProgress(token);
    // Keep what the page has when the read failed. Before the first reply is in,
    // a local update in the meantime is no reason to drop it: the merge below
    // already puts the page's own items over the server's.
    if (!record || isPlaying || (progressLoaded && version !== progressVersion)) return;
    progress = progressLoaded
      ? record
      : { last: progress.last || record.last, items: { ...record.items, ...progress.items } };
    progressLoaded = true;
  }

  // Set while a start waits for the first read, so every control that spends a
  // play is held meanwhile - a second tap would otherwise start a second play.
  let awaitingProgress = false;

  // Runs a start once the first read is in, but waits no longer than a few
  // seconds: a slow server must not hold the Play button hostage.
  async function afterProgress(start) {
    if (busy) return;
    awaitingProgress = true;
    try {
      await Promise.race([firstProgress, new Promise((resolve) => setTimeout(resolve, 3000))]);
    } finally {
      awaitingProgress = false;
    }
    return start();
  }

  // Back on the tab after watching elsewhere - another device, or the same link
  // on a TV - the page picks up what was watched meanwhile.
  function handleVisibility() {
    if (document.visibilityState === 'visible' && !needsPassword && !isPlaying) refreshProgress();
  }

  $: movieResumeAt = resumePosition(progress.items[MAIN]);
  $: movieWatched = isWatched(progress.items[MAIN]);
  $: seriesResume = seriesResumeTarget(progress, episodes);

  // The episode after the one playing, for the card near its end.
  $: upNext = isPlaying && currentEpisodeId ? nextEpisode(currentEpisodeId) : null;
  $: nextUpCard = upNext ? { label: episodeLabel(upNext, multiSeason), name: upNext.name } : null;

  // The track the viewer picked, so the player can name it and switch it on.
  $: chosenSubtitle = (shareInfo.subtitleTracks || []).find((t) => t.index === selectedSubtitleIndex);

  // Episode list for Season/Series
  let episodes = [];
  let episodesLoading = false;
  let episodesError = '';
  // Asked for once per unlock. Keying the load on an empty list instead would
  // re-request forever on a season that is empty or whose request failed.
  let episodesRequested = false;
  // Stills that failed to load leave the row without one, not a broken image.
  let thumbFailed = {};

  onMount(() => {
    initCast();
    onCastEnded(advanceCast);
    const timeout = setTimeout(() => {
      imageLoaded = true;
    }, 500);

    return () => clearTimeout(timeout);
  });

  // On open, or once the password has been entered.
  $: if (isSeasonOrSeries && !needsPassword && !episodesRequested) {
    episodesRequested = true;
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
        episodesError = refusal(response.status, data.error, 'episodes');
        return;
      }
      const data = await response.json();
      episodes = data.episodes || [];
    } catch (e) {
      episodesError = OFFLINE;
    } finally {
      episodesLoading = false;
    }
    if (focusPlayWhenReady) {
      focusPlayWhenReady = false;
      await tick();
      playButton?.focus();
    }
  }

  // Re-reads the share: once the password is in, the details it withheld.
  async function refreshShare() {
    try {
      const response = await fetch(`/api/public/shares/${token}`, { credentials: 'include' });
      if (response.ok) shareInfo = await response.json();
    } catch (e) {
      // The page keeps what it has; the next play asks the server anyway.
    }
  }

  // ---- The link's terms, said before they bite ----
  $: playLimit = shareInfo.maxTotalPlays || null;
  $: playsLeft = playLimit ? Math.max(0, playLimit - shareInfo.totalPlays) : null;
  $: lastPlay = playsLeft === 1;
  $: playsUsedUp = playsLeft === 0;
  // The page turns into the spent view only once nothing is still playing: the
  // last play is spent the moment it starts, and a TV playing it still needs
  // its Stop button.
  $: spent = playsUsedUp && !castSessionId && !$castConnected;
  $: viewerLimit = shareInfo.maxConcurrentViewers || null;
  // A snapshot from when the page loaded, and it may count a viewer who closed
  // the tab without stopping. The server clears those when Play is pressed, so
  // this warns rather than blocks.
  $: viewersFull = viewerLimit !== null && shareInfo.currentConcurrentViewers >= viewerLimit;

  $: playsCount = playLimit ? `${playsLeft} of ${playLimit} ${playLimit === 1 ? 'play' : 'plays'} left` : '';

  // The headline of the terms: what is left and until when, in one line at
  // reading size above the button that spends it. The finer print sits below.
  $: playsHead = !playLimit
    ? ''
    : playsUsedUp
      ? 'No plays left'
      : lastPlay
        ? (playLimit === 1 ? 'One play' : `Last of ${playLimit} plays`)
        : playsCount;
  $: expiryHead = shareInfo.expiresAt ? `expires ${expiryWords(shareInfo.expiresAt)}` : 'no expiry';
  // The headline is the one place the terms are stated, so the exact moment
  // rides along under it rather than in a second and third spot on the page.
  $: expiryExact = shareInfo.expiresAt ? formatAbsolute(shareInfo.expiresAt) : '';
  const capitalised = (text) => text.charAt(0).toUpperCase() + text.slice(1);

  // Read before the button, not after it. A spent link says its piece in place
  // of the controls instead (see .spent), except while a TV is still connected
  // and the controls stay on screen.
  $: playsWarning = playsUsedUp
    ? (spent ? '' : 'Every play on this link has been used.')
    : lastPlay
      ? (playLimit === 1 ? 'This link has a single play. ' : 'This is the last play. ') +
        (shareInfo.allowDownload
          ? 'Watching here or on a TV, or downloading, uses it.'
          : 'Watching here or on a TV uses it.')
      : '';

  // Resume comes from the link's shared record, so the place may be someone
  // else's, or the viewer's own on another device. It is said right at the
  // button that offers it, which may be the first thing a new viewer sees.
  $: resumedMidway = isSeasonOrSeries ? seriesResume?.position > 0 : movieResumeAt > 0;
  $: resumeNote = resumedMidway
    ? 'Where anyone with this link last stopped, on any device.'
    : isSeasonOrSeries && seriesResume?.isNext
      ? 'Next after the last episode anyone with this link finished.'
      : '';

  $: viewersLine = !viewerLimit
    ? ''
    : viewersFull
      ? (viewerLimit === 1
          ? 'Someone is watching right now, and only one person can watch at a time. Playing may not start until they stop.'
          : `${viewerLimit} people are watching right now, the most this link allows. Playing may not start until one of them stops.`)
      : (viewerLimit === 1 ? 'One person can watch at a time.' : `Up to ${viewerLimit} people can watch at once.`);

  // How a series spends plays, which is not obvious from "3 plays": running on
  // into the next episode, by autoplay or Play now on the card, carries the
  // play along (see continuationOf on the server); an episode the viewer picks
  // is a new one.
  $: countingNote = playLimit && isSeasonOrSeries
    ? 'When an episode runs on into the next, or you take Play now at its end, that is still one play. Starting an episode yourself uses one.' +
      (shareInfo.allowDownload ? ' So does each download.' : '')
    : '';

  $: tvNote = tvHint();

  // ---- Refusals, in the viewer's words ----
  // The server's error strings are written for logs. The viewer needs what
  // happened, whether it cost a play, and what to do; every refusal below comes
  // back before the server charges anything, so "no play was used" is true.
  const OFFLINE = 'Couldn’t reach the server. Check your connection and try again.';

  function refusal(status, serverError = '', action = 'play') {
    const what = {
      play: 'Playback didn’t start.',
      next: 'The next episode didn’t start.',
      download: 'The download didn’t start.',
      episodes: 'The episodes couldn’t be loaded.'
    }[action];
    const free = action === 'episodes' ? '' : ' No play was used.';
    const said = String(serverError).toLowerCase();

    if (status === 401) {
      needsPassword = true;
      return 'This link needs its password again.';
    }
    if (status === 410) return 'This link has expired, or whoever shared it switched it off.';
    if (status === 429) return `${what} Too many tries in a short time — wait a minute, then try again.`;
    if (status === 403) {
      if (said.includes('maximum plays')) {
        // The page turns into the spent view, whose statement says this - unless
        // a TV is still connected, which keeps the controls on screen, or the page
        // never knew of a limit to count down.
        shareInfo = { ...shareInfo, totalPlays: Math.max(shareInfo.totalPlays, playLimit || 0) };
        return castSessionId || $castConnected || !playLimit
          ? 'All the plays on this link have been used. Ask whoever shared it for a new link.'
          : '';
      }
      if (said.includes('concurrent')) {
        return `${what} Every place on this link is taken right now. Try again once someone stops watching.${free}`;
      }
      if (said.includes('downloads')) return 'Downloads are switched off for this link.';
    }
    if (status === 400 && said.includes('track')) {
      return `${what} The audio or subtitle track you picked isn’t available here. Pick another.${free}`;
    }
    if (status === 502) return `${what} The media server didn’t answer. Try again in a minute.${free}`;
    return `${what} Try again, or let whoever shared the link know.${free}`;
  }

  function formatDuration(seconds) {
    if (!seconds) return '';
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
  }

  const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;

  // "in 6 days" while there is time to spare; under two days the hours count.
  function expiryWords(expiresAt) {
    const diff = new Date(expiresAt) - Date.now();
    if (!(diff > 0)) return 'now';
    const hours = Math.floor(diff / (1000 * 60 * 60));
    if (hours >= 48) return `in ${plural(Math.floor(hours / 24), 'day')}`;
    if (hours >= 1) return `in ${plural(hours, 'hour')}`;
    return `in ${plural(Math.max(1, Math.floor(diff / (1000 * 60))), 'minute')}`;
  }

  // A download's size, so a tap on mobile data is a choice. Decimal units, as
  // phones and file managers show them.
  function formatBytes(bytes) {
    if (!(bytes > 0)) return '';
    const units = ['KB', 'MB', 'GB', 'TB'];
    let n = bytes / 1000;
    let i = 0;
    while (n >= 1000 && i < units.length - 1) { n /= 1000; i += 1; }
    return `${n >= 10 ? Math.round(n) : n.toFixed(1)} ${units[i]}`;
  }

  function formatRating(rating) {
    return rating.toFixed(1);
  }

  async function submitPassword() {
    if (passwordLoading) return;
    if (!passwordInput.trim()) {
      passwordError = 'Enter the password to open this link.';
      passwordField?.focus();
      return;
    }
    passwordLoading = true;
    passwordError = '';
    let opened = false;
    try {
      const response = await fetch(`/api/public/shares/${token}/password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: passwordInput }),
        credentials: 'include'
      });
      if (response.ok) {
        opened = true;
      } else if (response.status === 401) {
        passwordError = 'That password doesn’t match. Check it with whoever shared the link.';
      } else if (response.status === 429) {
        passwordError = 'Too many tries in a short time. Wait a minute, then try again.';
      } else {
        passwordError = 'The password couldn’t be checked. Try again in a moment.';
      }
    } catch (e) {
      passwordError = OFFLINE;
    } finally {
      passwordLoading = false;
    }

    if (!opened) {
      passwordField?.select();
      return;
    }
    // The details were withheld until now; fetch them before showing the page.
    await refreshShare();
    passwordInput = '';
    needsPassword = false;
    episodesRequested = false;
    progressRequested = false;
    focusPlayWhenReady = isSeasonOrSeries;
    await tick();
    if (!isSeasonOrSeries) playButton?.focus();
  }

  let selectedAudioIndex = null;
  let selectedSubtitleIndex = null;
  let tracksOpen = false;

  // Jellyfin's own titles name codecs ("Deutsch - PGSSUB"), which mean nothing to
  // a viewer. A track is named by its language and, for audio, its layout; only
  // tracks that would then share a name - a commentary in the same language -
  // keep Jellyfin's title, since that is what tells them apart.
  const LAYOUTS = { 1: 'Mono', 2: 'Stereo', 6: '5.1', 8: '7.1' };

  function plainLabel(t, fallback) {
    const name = languageName(toBcp47(t.language)) || t.displayTitle || `${fallback} ${t.index}`;
    const extra = [];
    if (t.channels) extra.push(LAYOUTS[t.channels] || `${t.channels} channels`);
    if (t.isForced) extra.push('forced');
    return extra.length ? `${name} (${extra.join(', ')})` : name;
  }

  function labelTracks(tracks = [], fallback) {
    const plain = tracks.map((t) => plainLabel(t, fallback));
    return new Map(tracks.map((t, i) => [
      t.index,
      plain.indexOf(plain[i]) !== plain.lastIndexOf(plain[i]) && t.displayTitle ? t.displayTitle : plain[i]
    ]));
  }

  $: audioLabels = labelTracks(shareInfo.audioTracks, 'Track');
  // Left to itself Jellyfin may pick audio by its own account's language
  // preference, which the page cannot see. So the track marked default - or the
  // first - is chosen outright and sent like any other choice: what the menu
  // shows is what plays, and every track is listed once.
  $: defaultAudio = (shareInfo.audioTracks || []).find((t) => t.isDefault) ?? shareInfo.audioTracks?.[0];
  $: if (selectedAudioIndex == null && shareInfo.audioTracks?.length > 1 && defaultAudio) {
    selectedAudioIndex = defaultAudio.index;
  }
  $: subtitleLabels = labelTracks(shareInfo.subtitleTracks, 'Subtitle');
  $: hasTracks = (shareInfo.audioTracks?.length > 1) || (shareInfo.subtitleTracks?.length > 0);
  $: tracksSummary = [
    shareInfo.audioTracks?.length > 1
      ? `Audio: ${audioLabels.get(selectedAudioIndex)}`
      : null,
    shareInfo.subtitleTracks?.length > 0
      ? `Subtitles: ${selectedSubtitleIndex == null ? 'None' : subtitleLabels.get(selectedSubtitleIndex)}`
      : null
  ].filter(Boolean).join(' · ');

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
  // What the receiver is playing: its progress key, and the runtime to fall back
  // on until the receiver reports one.
  let castPlaying = null;

  // Where the receiver stands, or null while it has nothing worth keeping - an
  // idle or reset receiver must not wind the link back to 0. `ended` counts the
  // item as watched to the end. noteCast acts on what this returns.
  function castSnapshot({ ended = false } = {}) {
    if (!castPlaying) return null;
    const remote = castProgress();
    if (!ended && !remote) return null;
    const duration = remote?.duration || castPlaying.duration;
    if (!(duration > 0)) return null;
    return { key: castPlaying.key, position: ended ? duration : remote.position, duration };
  }

  // Moves the page's own copy to the receiver's position, so the episode list
  // follows the TV, and returns it as the server takes it.
  function noteCast(snapshot) {
    if (!snapshot) return {};
    recordProgress(snapshot.key, snapshot.position, snapshot.duration);
    return { positionSeconds: Math.floor(snapshot.position), durationSeconds: Math.round(snapshot.duration) };
  }
  // Which episode is on the receiver, so the list can mark it.
  let castingEpisodeId = null;

  // Releases the session server-side rather than leaving it to the stale-session
  // reaper, which would hold a concurrent-viewer slot for the timeout's duration.
  async function endCastSession() {
    // Read before stopping: once the receiver lets go it no longer knows.
    const last = noteCast(castSnapshot());
    stopCast();
    castingEpisodeId = null;
    await finishCastSession(last);
  }

  // Releases the session but leaves the device connected, so the next episode can
  // start without opening the picker again.
  async function finishCastSession(last = {}) {
    stopCastHeartbeat();
    const id = castSessionId;
    castSessionId = null;
    castPlaying = null;
    if (!id) return;
    try {
      await fetch(`/api/public/sessions/${id}/finish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(last),
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
          body: JSON.stringify(noteCast(castSnapshot())),
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
  async function castItem({ playPath, title, subtitle, durationSeconds, episodeId = null, startTime = 0, charged = true }) {
    // One at a time: each run ends in a /play, and a second one is a second play.
    if (casting) return;
    playError = '';
    casting = true;
    // Set once the server has counted this start: a TV that then fails to load
    // it has still cost a play, and the message has to say so.
    let playCharged = false;
    try {
      // Pick the device first. /play counts against the share's play limit, so it
      // must not run if the viewer dismisses the picker.
      await ensureCastSession();

      // A receiver plays one thing at a time. Switching episodes without releasing
      // the previous session would leave it holding a concurrent-viewer slot until
      // the stale-session reaper gets to it.
      await finishCastSession(noteCast(castSnapshot()));

      const response = await fetch(playPath, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = refusal(response.status, data.error, 'play');
        return;
      }
      const data = await response.json();
      if (charged) {
        spendPlay();
        playCharged = true;
      }
      const chosen = (shareInfo.subtitleTracks || []).find((t) => t.index === selectedSubtitleIndex);
      await loadOnCast({
        url: data.playbackUrl,
        subtitleUrl: data.subtitleUrl,
        subtitleLanguage: chosen?.language,
        title,
        subtitle,
        posterUrl: shareInfo.posterUrl,
        durationSeconds,
        startTime
      });
      castSessionId = data.sessionId;
      castingEpisodeId = episodeId;
      castPlaying = { key: episodeId || MAIN, duration: durationSeconds || 0 };
      startCastHeartbeat(data.sessionId);
    } catch (e) {
      // A dismissed picker is not a failure.
      const said = describeCastError(e);
      playError = !said ? ''
        : !playCharged || !playLimit ? `${said.cause} ${said.next}`
        : playsUsedUp ? `${said.cause} That was this link’s last play, so it can’t be started again. Ask whoever shared it for another.`
        : `${said.cause} ${said.next} The play was counted when it started, so starting again uses another.`;
    } finally {
      casting = false;
    }
  }

  function startCast() {
    return afterProgress(() => castItem({
      playPath: `/api/public/shares/${token}/play${trackQuery('h264')}`,
      title: shareInfo.title,
      subtitle: shareInfo.year ? String(shareInfo.year) : '',
      durationSeconds: shareInfo.runtimeSeconds,
      startTime: resumePosition(progress.items[MAIN])
    }));
  }

  function castEpisode(episode, { continues, startTime = 0 } = {}) {
    return castItem({
      playPath: episodePlayPath(episode.id, { codec: 'h264', continues }),
      title: episode.name,
      subtitle: `${episodeLabel(episode, multiSeason)} \u00b7 ${displayTitle}`,
      durationSeconds: episode.runtimeSeconds,
      episodeId: episode.id,
      startTime,
      charged: !continues
    });
  }

  // The server counts a play the moment it starts one; the line under the
  // button follows without waiting for a reload.
  function spendPlay() {
    shareInfo = { ...shareInfo, totalPlays: shareInfo.totalPlays + 1 };
  }

  // A season or series has no single Play button to sit beside, so connecting is
  // its own step. It costs nothing: /play only runs once an episode is picked.
  async function connectCast() {
    playError = '';
    casting = true;
    try {
      await ensureCastSession();
    } catch (e) {
      const said = describeCastError(e);
      playError = said ? `${said.cause} ${said.next}` : '';
    } finally {
      casting = false;
    }
  }

  // In words wherever it is read as a sentence or a button. The season is
  // named only when the link spans more than one.
  $: multiSeason = shareInfo.itemType === 'Series';

  // multiSeason is passed in, not read, so that every reactive statement and
  // template expression that names an episode lists it and follows it.
  function episodeLabel(episode, spansSeasons) {
    const n = `Episode ${episode.indexNumber || '?'}`;
    return spansSeasons && episode.seasonNumber ? `Season ${episode.seasonNumber}, ${n.toLowerCase()}` : n;
  }

  // What a series' Play does, in its own words.
  function seriesPlayText(resume, list, spansSeasons) {
    if (resume?.position > 0) return `Resume ${episodeLabel(resume.episode, spansSeasons)}`;
    if (resume) return `${resume.isNext ? 'Continue with' : 'Play'} ${episodeLabel(resume.episode, spansSeasons)}`;
    return list.length ? `Play ${episodeLabel(list[0], spansSeasons)}` : '';
  }
  $: seriesPlayLabel = seriesPlayText(seriesResume, episodes, multiSeason);

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

  async function finishSession(sessionId, last = {}) {
    if (!sessionId) return;
    try {
      await fetch(`/api/public/sessions/${sessionId}/finish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(last),
        credentials: 'include'
      });
    } catch (e) {
      console.warn('Could not finish session', e);
    }
  }

  // Autoplay in the browser - and with it AirPlay, which mirrors the same element,
  // so an episode finishing on an Apple TV arrives here as well.
  async function handlePlaybackEnded() {
    // A film, like the last episode of a series, has nothing to follow it:
    // back to the share page, where it now shows as watched.
    if (!isSeasonOrSeries) {
      handlePlayerClose();
      return;
    }

    const next = nextEpisode(currentEpisodeId);
    if (!next) {
      handlePlayerClose();
      return;
    }

    // Release the finished session before asking for the next one. A share
    // limited to one concurrent viewer would otherwise refuse its own sequel.
    // The player marked the episode watched as it ended; its own finish call
    // would arrive at a session already closed here, so this one carries it.
    const previous = playbackData?.sessionId;
    const ended = progress.items[currentEpisodeId];
    await finishSession(previous, ended
      ? { positionSeconds: Math.floor(ended.position), durationSeconds: Math.round(ended.duration) }
      : {});

    try {
      const response = await fetch(episodePlayPath(next.id, { continues: previous }), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        nextFailed(refusal(response.status, data.error, 'next'), previous, next);
        return;
      }
      startPosition = 0;
      const data = await response.json();
      // The viewer may have left the player while this was loading - and maybe
      // started another episode already. Either way this session is no longer
      // wanted: with no player to heartbeat or finish it, it would hold a
      // viewer slot until the stale-session reaper found it.
      if (playbackData?.sessionId !== previous) {
        await finishSession(data.sessionId);
        return;
      }
      playbackData = data;
      currentEpisodeId = next.id;
      currentPlayingTitle = `${episodeLabel(next, multiSeason)}: ${next.name}`;
    } catch (e) {
      nextFailed(OFFLINE, previous, next);
    }
  }

  // The next episode did not start. A player still open on the episode that
  // ended closes and says so, with focus on the episode that did not start:
  // the one to retry. If the viewer already left the player, it is still said,
  // unless they have started something else meanwhile - another player, one
  // still starting, a download, or the TV - which is what the page is about now.
  function nextFailed(message, previous, next) {
    if (playbackData) {
      if (playbackData.sessionId !== previous) return;
      playError = message;
      handlePlayerClose(next.id);
      return;
    }
    if (busy || castSessionId) return;
    playError = message;
  }

  // Autoplay on the receiver. Reading castSessionId before castEpisode runs
  // matters: it releases the old session and clears the field on the way.
  // It ran out on the TV, so it counts as watched to the end.
  async function advanceCast() {
    const next = nextEpisode(castingEpisodeId);
    console.debug('Cast: episode finished', castingEpisodeId, '-> next', next?.id || 'none');
    const previous = castSessionId;
    await finishCastSession(noteCast(castSnapshot({ ended: true })));
    if (!next) {
      stopCast();
      castingEpisodeId = null;
      return;
    }
    await castEpisode(next, { continues: previous });
  }

  // What is being started (MAIN or an episode id). Every way of starting is
  // disabled meanwhile: a second tap would be a second play on the server.
  let starting = null;
  // Starting, casting or preparing a download: each spends from the same budget,
  // so while one is in flight every control that spends is held.
  $: busy = starting !== null || casting || preparingDownload !== null || awaitingProgress;

  async function startPlayback(startAt = 0) {
    if (starting) return;
    playError = '';
    starting = MAIN;
    try {
      const response = await fetch(`/api/public/shares/${token}/play${trackQuery()}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = refusal(response.status, data.error, 'play');
        return;
      }
      startPosition = startAt;
      playbackData = await response.json();
      spendPlay();
      currentPlayingTitle = shareInfo.title;
      currentEpisodeId = null;
      isPlaying = true;
    } catch (e) {
      playError = OFFLINE;
    } finally {
      starting = null;
    }
  }

  async function startEpisodePlayback(episode, startAt = 0) {
    if (starting) return;
    playError = '';
    starting = episode.id;
    try {
      const response = await fetch(`/api/public/shares/${token}/episodes/${episode.id}/play${trackQuery()}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = refusal(response.status, data.error, 'play');
        return;
      }
      startPosition = startAt;
      playbackData = await response.json();
      spendPlay();
      currentPlayingTitle = `${episodeLabel(episode, multiSeason)}: ${episode.name}`;
      currentEpisodeId = episode.id;
      isPlaying = true;
    } catch (e) {
      playError = OFFLINE;
    } finally {
      starting = null;
    }
  }

  // Every way out of the player ends here - Back, an episode ending after
  // autoplay was cancelled on the card, the last episode ending, a refused
  // next episode - and fullscreen ends with it: it
  // belongs to the page, and the episode list has no business filling the
  // screen. Autoplay swaps the player without coming through here, so
  // fullscreen survives the move to the next episode.
  async function handlePlayerClose(focusEpisodeId = currentEpisodeId) {
    if (fullscreenElement()) exitFullscreen();
    isPlaying = false;
    playbackData = null;
    currentPlayingTitle = '';
    currentEpisodeId = null;
    startPosition = 0;
    // The first read may have landed while the player was open and been set
    // aside; without it the other episodes would show nothing.
    if (!progressLoaded) refreshProgress();
    // The page is rebuilt as the player goes, and the control that had focus
    // goes with the player, so focus would fall to the top of the document.
    // It goes back to what was playing: the episode's row, or Play - or, once
    // that was the last play, the statement saying so (or, with a TV still
    // holding the controls, the terms saying it).
    await tick();
    // A disabled control cannot take focus, so the first that can is used.
    [focusEpisodeId && document.getElementById(`ep-${focusEpisodeId}`), playButton, spentNote, termsHead]
      .find((el) => el && !el.disabled)?.focus();
  }

  // An episode row, or the series' main button: on the receiver when one is
  // connected, here otherwise.
  function playEpisode(episode, startAt = 0) {
    return $castConnected
      ? castEpisode(episode, { startTime: startAt })
      : startEpisodePlayback(episode, startAt);
  }

  // The series' main button: resume where the viewer was, or start at the top.
  // Each reads the record itself once the wait is over, rather than a reactive
  // value that may not have caught up with it yet.
  function playSeries() {
    return afterProgress(() => {
      const target = seriesResumeTarget(progress, episodes);
      if (target) return playEpisode(target.episode, target.position);
      if (episodes.length) return playEpisode(episodes[0]);
    });
  }

  // An episode row: where that episode was left.
  function resumeEpisode(episode) {
    return afterProgress(() => playEpisode(episode, resumePosition(progress.items[episode.id])));
  }

  function playMovie() {
    return afterProgress(() => startPlayback(resumePosition(progress.items[MAIN])));
  }

  // ---- Downloads ----
  // Which download is being prepared ('main', an episode id, or 'all'), so only
  // that button shows it.
  let preparingDownload = null;
  // The download waiting on its confirm (same keys as above).
  let confirmingDownload = null;
  // Once the last play is spent elsewhere - another row, another device - there
  // is nothing left for an open confirm to guard.
  $: if (playsUsedUp) confirmingDownload = null;

  // On a link with a play limit a download asks first, in place: it spends a
  // play, and a stray tap on a phone would spend it on a large file over mobile
  // data. Play itself does not ask - it is what the link is for.
  async function requestDownload(path, key) {
    if (playLimit && confirmingDownload !== key) {
      confirmingDownload = key;
      return;
    }
    confirmingDownload = null;
    await startDownload(path, key);
  }

  // The row's own button opens the confirm and closes it again; only "Download
  // it" inside the confirm goes ahead, so a double click on the row cannot
  // spend a play.
  function rowDownload(key, download) {
    if (confirmingDownload === key) return keepPlay(key);
    return download();
  }

  async function keepPlay(key) {
    confirmingDownload = null;
    await tick();
    document.getElementById(`dl-${key}`)?.focus();
  }

  // The POST is what checks the share and charges the play; the file itself
  // then comes from a plain GET the browser handles as a download, so the page
  // stays where it is.
  async function startDownload(path, key) {
    playError = '';
    preparingDownload = key;
    try {
      const response = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include'
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        playError = refusal(response.status, data.error, 'download');
        return;
      }
      const { downloadUrl } = await response.json();
      const a = document.createElement('a');
      a.href = downloadUrl;
      a.download = '';
      document.body.appendChild(a);
      a.click();
      a.remove();
      spendPlay();
    } catch (e) {
      playError = OFFLINE;
    } finally {
      preparingDownload = null;
    }
  }

  const downloadMovie = () => requestDownload(`/api/public/shares/${token}/download`, MAIN);
  const downloadEpisode = (episode) =>
    requestDownload(`/api/public/shares/${token}/episodes/${episode.id}/download`, episode.id);
  const downloadAll = () => requestDownload(`/api/public/shares/${token}/episodes/download`, 'all');

  // A ZIP stores the files as they are, so its size is theirs added up - known
  // only when every episode's is.
  $: allBytes = episodes.length && episodes.every((e) => e.sizeBytes > 0)
    ? episodes.reduce((sum, e) => sum + e.sizeBytes, 0)
    : 0;

  // What a confirm says: the size, and what the download costs.
  function downloadCost(what, bytes) {
    const size = formatBytes(bytes);
    const thing = size ? `${what} (${size})` : what;
    return lastPlay
      ? `Downloading ${thing} uses ${playLimit === 1 ? 'the link’s only' : 'the last'} play. The link can’t be streamed afterwards.`
      : `Downloading ${thing} uses one of the ${plural(playsLeft, 'play')} left.`;
  }

  // Escape inside a confirm is "Don’t download".
  const escapeTo = (key) => (e) => { if (e.key === 'Escape') keepPlay(key); };

  // Moves focus to the element the moment it appears: a confirm, so a keyboard
  // viewer lands on the choice rather than behind it, or the password field,
  // the only thing a locked page asks for.
  function focusOnMount(node) {
    node.focus();
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

  // In English, like the rest of the page, but in day-month order with a 24-hour
  // clock and the zone named, which reads unambiguously wherever the viewer is.
  const absoluteTime = new Intl.DateTimeFormat('en-GB', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZoneName: 'short'
  });

  // "1:00 of 2:00": a resume point means little without the length it is in.
  function clockOf(position, total) {
    return total > 0 ? `${formatClock(position)} of ${formatClock(total)}` : formatClock(position);
  }

  // "in 6 days" answers "is this still worth opening"; the exact local moment
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
    shareInfo.communityRating > 0 ? `Audience ${formatRating(shareInfo.communityRating)}/10` : null,
    shareInfo.criticRating > 0 ? `Critics ${shareInfo.criticRating}%` : null
  ].filter(Boolean);

  // Long titles step down a size rung rather than crowding themselves.
  // A season share is titled "Season 1" by Jellyfin; the show it belongs to is
  // the name a viewer knows it by, with the season beneath it.
  $: displayTitle = shareInfo.seriesName || shareInfo.title;
  $: subTitle = shareInfo.seriesName ? shareInfo.title : '';
  $: titleIsLong = (displayTitle || '').length > 32;

  $: foldImage = shareInfo.backdropUrl || shareInfo.posterUrl || '';
</script>
<svelte:head><title>{[displayTitle, subTitle].filter(Boolean).join(' · ')} · Jellyfin Share</title></svelte:head>
<svelte:document on:visibilitychange={handleVisibility} />

<main>
  {#if playerOpen}
    <!-- Keyed on the session: autoplay swaps in the next episode, and the player
         has to be rebuilt around it rather than handed a new URL mid-flight. -->
    {#key playbackData.sessionId}
      <Player {playbackData} title={currentPlayingTitle || shareInfo.title}
              subtitleLabel={chosenSubtitle ? subtitleLabels.get(chosenSubtitle.index) : 'Subtitles'}
              subtitleLanguage={toBcp47(chosenSubtitle?.language)}
              {startPosition}
              nextUp={nextUpCard}
              progressKey={currentEpisodeId || MAIN}
              {playsLeft}
              onProgress={recordProgress}
              on:close={() => handlePlayerClose()} on:ended={handlePlaybackEnded}
              on:next={handlePlaybackEnded} />
    {/key}
  {:else}
    <!-- ── The photographic fold. The artwork fills it; the type is annotation.
         Locked, it gives way on a phone: the password field, not the picture,
         is what the page needs to show above the keyboard. ── -->
    <section class="fold" class:fold--bare={!foldImage} class:fold--locked={showPasswordForm && !spent}>
      {#if foldImage}
        <img class="fold__image" src={foldImage} alt="" fetchpriority="high" />
        <div class="fold__scrim"></div>
      {/if}

      <!-- N9 · Edge-aligned minimal: the wordmark alone. The terms are stated
           once, in the headline above the controls. -->
      <header class="nav">
        <span class="wordmark"><BrandMark />Jellyfin Share</span>
      </header>

      <div class="plate">
        {#if shareInfo.logoUrl}
          <!-- The logo is the title, so it stays the page's heading. -->
          <h1><img class="plate__logo" src="{shareInfo.logoUrl}?maxWidth=500" alt={displayTitle} /></h1>
        {:else}
          <h1 class="plate__title" class:plate__title--long={titleIsLong}>{displayTitle}</h1>
        {/if}
        {#if subTitle}
          <p class="plate__sub">{subTitle}</p>
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
            srcset="{shareInfo.posterUrl}?maxWidth=400 400w, {shareInfo.posterUrl}?maxWidth=600 600w, {shareInfo.posterUrl}?maxWidth=800 800w"
            sizes="(min-width: 80rem) 18rem, (min-width: 60rem) 13.5rem, (min-width: 40rem) 11rem, 1px"
            alt={displayTitle}
            class:is-loaded={imageLoaded}
            on:load={handleImageLoad}
            on:error={handleImageError}
          />
        </figure>
      {/if}

      <div class="act">
        {#if !spent}
          <!-- The terms' headline, at reading size and before the button: what is
               left and until when, then - if it is about to bite - what the next
               tap costs. The finer print follows below the controls. -->
          <div class="terms-head">
            <p class="terms-head__line" tabindex="-1" bind:this={termsHead}>
              {#if playsHead}{playsHead} · {/if}{#if shareInfo.expiresAt}<time datetime={shareInfo.expiresAt}>{playsHead ? expiryHead : capitalised(expiryHead)}</time>{:else}{playsHead ? expiryHead : capitalised(expiryHead)}{/if}
            </p>
            {#if expiryExact}<p class="terms-head__exact">Until {expiryExact}</p>{/if}
            {#if playLimit}
              <div class="fuel__track" role="meter" aria-label="Plays left"
                   aria-valuemin="0" aria-valuemax={playLimit} aria-valuenow={playsLeft}
                   aria-valuetext={playsCount}>
                <div class="fuel__fill" class:fuel__fill--low={playsLeft <= 1}
                     style="transform: scaleX({playsLeft / playLimit})"></div>
              </div>
            {/if}
            {#if playsWarning}<p class="terms-head__warn">{playsWarning}</p>{/if}
          </div>
        {/if}

        {#if spent}
          <!-- A spent link is a dead end and says so first - before any password,
               which could lead nowhere: every control here could only fail.
               A live region speaks when its content changes, not when it is
               inserted with it, so on the way back from the player the page
               focuses this sentence and it is read out that way. -->
          <div class="spent" role="status">
            <p class="spent__head" tabindex="-1" bind:this={spentNote}>{playLimit === 1 ? 'The one play on this link has been used.' : `All ${playLimit} plays on this link have been used.`}</p>
            <p class="spent__next">Ask whoever shared it for a new link.</p>
          </div>
        {:else if showPasswordForm}
          <form class="gate" on:submit|preventDefault={submitPassword} novalidate>
            <div class="field">
              <label for="share-password">Password</label>
              <div class="gate__row">
                <input
                  id="share-password"
                  type={showPassword ? 'text' : 'password'}
                  autocomplete="current-password"
                  autocapitalize="off"
                  spellcheck="false"
                  use:focusOnMount
                  bind:this={passwordField}
                  value={passwordInput}
                  on:input={(e) => (passwordInput = e.currentTarget.value)}
                  readonly={passwordLoading}
                  aria-invalid={passwordError ? 'true' : 'false'}
                  aria-describedby="share-password-help"
                />
                <button type="button" class="btn-ghost" aria-controls="share-password"
                        on:click={() => { showPassword = !showPassword; passwordField?.focus(); }}>
                  {showPassword ? 'Hide' : 'Show'}<span class="sr-only"> password</span>
                </button>
              </div>
              <!-- Under the field it is about, above the button: on a phone the
                   keyboard leaves room for little more than these. -->
              <p class="field__help" id="share-password-help" aria-live="polite"
                 class:field__help--error={passwordError}>
                {passwordError || 'Whoever shared this link set a password for it.'}
              </p>
            </div>
            <button type="submit" class="btn-primary gate__submit" disabled={passwordLoading}>
              {#if passwordLoading}
                <span class="meter meter--inline" aria-hidden="true"></span>
                <span>Checking</span>
              {:else}
                <span>Unlock</span>
              {/if}
            </button>
          </form>
        {:else}
          {#if hasTracks}
            <!-- Most viewers keep the defaults, so the choice is one line until
                 it is wanted - and it sits above Play, where it still applies. -->
            <details class="tracks" bind:open={tracksOpen}>
              <summary class="tracks__summary">
                <span class="tracks__now">{tracksSummary}</span>
                <span class="tracks__change">{tracksOpen ? 'Done' : 'Change'}</span>
              </summary>
              <div class="tracks__fields">
                {#if shareInfo.audioTracks && shareInfo.audioTracks.length > 1}
                  <div class="field">
                    <label for="audio-select">Audio</label>
                    <select id="audio-select" bind:value={selectedAudioIndex}>
                      {#each shareInfo.audioTracks as track}
                        <option value={track.index}>{audioLabels.get(track.index)}</option>
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
                        <option value={track.index}>{subtitleLabels.get(track.index)}</option>
                      {/each}
                    </select>
                  </div>
                {/if}
              </div>
            </details>
          {/if}

          {#if isSeasonOrSeries}
            {#if episodes.length > 0}
              <div class="play">
                <div class="play__lead" class:play__lead--noted={resumeNote}>
                  <button class="btn-primary btn-primary--lg" bind:this={playButton} on:click={playSeries}
                          aria-describedby={resumeNote ? 'resume-note' : undefined}
                          disabled={busy || playsUsedUp}>
                    {#if starting !== null}
                      <span class="meter meter--inline" aria-hidden="true"></span>
                      <span>Starting</span>
                    {:else}
                      <span class="btn__main">
                        <PlayIcon />
                        <span>{seriesPlayLabel}</span>
                      </span>
                      {#if resumedMidway}
                        <span class="btn__aside">{clockOf(seriesResume.position, seriesResume.episode.runtimeSeconds)}</span>
                      {/if}
                    {/if}
                  </button>
                  {#if resumeNote}
                    <p class="play__note" class:play__note--link={resumedMidway}>
                      <span id="resume-note">{resumeNote}</span>
                      {#if resumedMidway}
                        <!-- The episode it names, from 0: a restart needs no stored
                             place, so there is nothing to wait for or recompute. -->
                        <button class="link" on:click={() => playEpisode(seriesResume.episode)}
                                disabled={busy || playsUsedUp}>
                          Restart {episodeLabel(seriesResume.episode, multiSeason)}
                        </button>
                      {/if}
                    </p>
                  {/if}
                </div>
                {#if shareInfo.allowDownload}
                  <button class="btn-ghost" id="dl-all" on:click={() => rowDownload('all', downloadAll)}
                          disabled={busy || playsUsedUp} aria-expanded={playLimit ? confirmingDownload === 'all' : undefined}>
                    <DownloadIcon />
                    <span>{preparingDownload === 'all' ? 'Preparing' : 'Download all'}</span>
                    {#if allBytes}<span class="btn__aside">{formatBytes(allBytes)}</span>{/if}
                  </button>
                {/if}
                {#if $castApiReady}
                  <!-- Connecting costs nothing: a play is spent once an episode is picked. -->
                  {#if $castConnected}
                    <button class="btn-ghost" on:click={endCastSession}>
                      <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                        <rect x="6" y="6" width="12" height="12" rx="1.5" />
                      </svg>
                      <span>Stop casting</span>
                    </button>
                  {:else}
                    <button class="btn-ghost" on:click={connectCast} disabled={casting}>
                      <CastIcon />
                      <span>{casting ? 'Connecting' : 'Cast to a TV'}</span>
                    </button>
                  {/if}
                {/if}
                {#if confirmingDownload === 'all'}
                  <div class="confirm" role="group" aria-labelledby="confirm-all">
                    <p class="confirm__q" id="confirm-all">{downloadCost('every episode', allBytes)}</p>
                    <button class={lastPlay ? 'btn-danger' : 'btn-primary'} use:focusOnMount on:keydown={escapeTo('all')} on:click={downloadAll}>Download it</button>
                    <button class="btn-ghost" on:keydown={escapeTo('all')} on:click={() => keepPlay('all')}>Don’t download</button>
                  </div>
                {/if}
              </div>
              {#if $castConnected}
                <p class="castbar__state" role="status">
                  {#if castSessionId}Playing on <strong>{$castDeviceName}</strong>. Keep this page open while it plays.
                  {:else}Connected to <strong>{$castDeviceName}</strong>. Pick an episode below to play it there.{/if}
                </p>
              {/if}
            {/if}
          {:else}
            <div class="play">
              <div class="play__lead" class:play__lead--noted={resumeNote}>
                <button class="btn-primary btn-primary--lg" bind:this={playButton}
                        aria-describedby={resumeNote ? 'resume-note' : undefined}
                        on:click={playMovie}
                        disabled={busy || playsUsedUp}>
                  {#if starting === MAIN}
                    <span class="meter meter--inline" aria-hidden="true"></span>
                    <span>Starting</span>
                  {:else}
                    <span class="btn__main">
                      <PlayIcon />
                      <span>{resumedMidway ? 'Resume' : 'Play'}</span>
                    </span>
                    {#if resumedMidway}
                      <span class="btn__aside">{clockOf(movieResumeAt, shareInfo.runtimeSeconds)}</span>
                    {/if}
                  {/if}
                </button>
                {#if resumeNote}
                  <p class="play__note play__note--link">
                    <span id="resume-note">{resumeNote}</span>
                    <button class="link" on:click={() => startPlayback()} disabled={busy || playsUsedUp}>Start from the beginning</button>
                  </p>
                {/if}
              </div>

              {#if shareInfo.allowDownload}
                <button class="btn-ghost" id="dl-{MAIN}" on:click={() => rowDownload(MAIN, downloadMovie)}
                        disabled={busy || playsUsedUp} aria-expanded={playLimit ? confirmingDownload === MAIN : undefined}>
                  <DownloadIcon />
                  <span>{preparingDownload === MAIN ? 'Preparing' : 'Download'}</span>
                  {#if shareInfo.downloadBytes}<span class="btn__aside">{formatBytes(shareInfo.downloadBytes)}</span>{/if}
                </button>
              {/if}

              {#if $castApiReady}
                <button class="btn-ghost" on:click={startCast} disabled={busy || playsUsedUp}>
                  <CastIcon />
                  <span>{casting ? 'Connecting' : ($castConnected ? $castDeviceName : 'Cast to a TV')}</span>
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

              {#if $castConnected}
                <p class="castbar__state" role="status">
                  {#if castSessionId}Playing on <strong>{$castDeviceName}</strong>. Keep this page open while it plays.
                  {:else}Connected to <strong>{$castDeviceName}</strong>.{/if}
                </p>
              {/if}

              <!-- On a line of its own under the row, so the row keeps its shape. -->
              {#if confirmingDownload === MAIN}
                <div class="confirm" role="group" aria-labelledby="confirm-main">
                  <p class="confirm__q" id="confirm-main">{downloadCost('the film', shareInfo.downloadBytes)}</p>
                  <button class={lastPlay ? 'btn-danger' : 'btn-primary'} use:focusOnMount on:keydown={escapeTo(MAIN)} on:click={downloadMovie}>Download it</button>
                  <button class="btn-ghost" on:keydown={escapeTo(MAIN)} on:click={() => keepPlay(MAIN)}>Don’t download</button>
                </div>
              {/if}
            </div>

            {#if movieWatched}
              <p class="watched"><CheckIcon /> <span>Watched</span></p>
            {/if}
          {/if}
        {/if}

        {#if !spent && (viewerLimit || countingNote || (tvNote && !$castApiReady))}
          <!-- The finer print: how the terms apply. -->
          <div class="terms">
            {#if viewersLine}
              <p class="terms__line" class:terms__line--warn={viewersFull}>{viewersLine}</p>
            {/if}
            {#if countingNote}<p class="terms__line">{countingNote}</p>{/if}
            {#if tvNote && !$castApiReady}<p class="terms__line">{tvNote}</p>{/if}
          </div>
        {/if}

        <!-- On a spent page the statement above has the last word; an error from
             before the TV let go would only repeat it. -->
        {#if playError && !spent}<p class="msg msg--error" role="alert">{playError}</p>{/if}
        {#if $subtitlesDropped}
          <p class="msg" role="status">Casting without subtitles: the TV wouldn’t take the track.</p>
        {/if}

        {#if shareInfo.overview && !needsPassword}
          <p class="lede">{shareInfo.overview}</p>
        {/if}

        {#if isSeasonOrSeries && !needsPassword}
          <div class="episodes">
            <h2 class="episodes__head" aria-label={episodes.length ? plural(episodes.length, 'episode') : 'Episodes'}>
              Episodes
              {#if episodes.length > 0}<span class="episodes__n">{episodes.length}</span>{/if}
            </h2>

            {#if episodesLoading}
              <div class="state" role="status"><div class="meter"></div><p>Loading episodes</p></div>
            {:else if episodesError}
              <div class="state">
                <p class="msg msg--error" role="alert">{episodesError}</p>
                <button class="link" on:click={loadEpisodes}>Try again</button>
              </div>
            {:else if episodes.length === 0}
              <p class="msg">This link doesn’t include any episodes yet.</p>
            {:else}
              <ul class="eplist" class:eplist--seasons={multiSeason} class:eplist--spent={spent}>
                {#each episodes as episode (episode.id)}
                  {@const seen = watchedFraction(progress.items[episode.id])}
                  {@const thumb = episode.posterUrl && !thumbFailed[episode.id]}
                  <li class="eprow">
                    {#if spent}
                      <!-- Nothing left to play it with: the list stays, as a record. -->
                      <div class="ep ep--static" class:ep--thumb={thumb}>
                        <span class="sr-only">{episodeLabel(episode, multiSeason)}:</span>
                        <span class="ep__no" aria-hidden="true">{#if multiSeason && episode.seasonNumber}{episode.seasonNumber}.{episode.indexNumber || '?'}{:else}{episode.indexNumber || '?'}{/if}</span>
                        {#if thumb}
                          <img class="ep__thumb" src="{episode.posterUrl}?maxWidth=320" alt="" loading="lazy"
                               on:error={() => (thumbFailed = { ...thumbFailed, [episode.id]: true })} />
                        {/if}
                        <span class="ep__body">
                          <span class="ep__name">{episode.name}</span>
                          {#if episode.overview}<span class="ep__about">{episode.overview}</span>{/if}
                        </span>
                        <span class="ep__end">
                          {#if seen >= 1}<span class="ep__watched"><CheckIcon title="Watched" /></span>{/if}
                          {#if episode.runtimeSeconds}<span class="ep__len">{formatDuration(episode.runtimeSeconds)}</span>{/if}
                        </span>
                      </div>
                    {:else}
                    <button class="ep" id="ep-{episode.id}"
                            class:ep--thumb={thumb}
                            class:ep--casting={$castConnected && castingEpisodeId === episode.id}
                            disabled={busy || playsUsedUp}
                            aria-describedby={episode.overview ? `ep-about-${episode.id}` : undefined}
                            on:click={() => resumeEpisode(episode)}>
                      <span class="sr-only">{$castConnected ? `Play on ${$castDeviceName}:` : 'Play'} {episodeLabel(episode, multiSeason)}:</span>
                      <span class="ep__no" aria-hidden="true">
                        {#if multiSeason && episode.seasonNumber}{episode.seasonNumber}.{episode.indexNumber || '?'}{:else}{episode.indexNumber || '?'}{/if}
                      </span>
                      {#if thumb}
                        <img class="ep__thumb" src="{episode.posterUrl}?maxWidth=320" alt="" loading="lazy"
                             on:error={() => (thumbFailed = { ...thumbFailed, [episode.id]: true })} />
                      {/if}
                      <span class="ep__body">
                        <span class="ep__name">{episode.name}</span>
                        {#if episode.overview}<span class="ep__about" id="ep-about-{episode.id}" aria-hidden="true">{episode.overview}</span>{/if}
                      </span>
                      <span class="ep__end" class:ep__end--cast={$castConnected}>
                        {#if seen >= 1}
                          <span class="ep__watched"><CheckIcon title="Watched" /></span>
                        {/if}
                        {#if episode.runtimeSeconds}
                          <span class="ep__len">{formatDuration(episode.runtimeSeconds)}</span>
                        {/if}
                        {#if starting === episode.id}
                          <span class="meter meter--dot" aria-hidden="true"></span>
                        {:else if $castConnected}
                          <CastIcon title="Plays on {$castDeviceName}" />
                        {:else}
                          <!-- Without a receiver the row plays here, and it has to say so:
                               a title and a runtime alone do not read as a control. -->
                          <svg class="ep__play" viewBox="0 0 24 24" fill="currentColor"
                               stroke="currentColor" stroke-width="1.75" stroke-linejoin="round"
                               aria-hidden="true">
                            <polygon points="6 3 20 12 6 21 6 3" />
                          </svg>
                        {/if}
                      </span>
                    </button>
                    {/if}
                    {#if shareInfo.allowDownload && !spent}
                      <button class="ep__dl" id="dl-{episode.id}" on:click={() => rowDownload(episode.id, () => downloadEpisode(episode))}
                              disabled={busy || playsUsedUp} aria-expanded={playLimit ? confirmingDownload === episode.id : undefined}
                              title={episode.sizeBytes ? `Download · ${formatBytes(episode.sizeBytes)}` : 'Download'}
                              aria-label="Download {episodeLabel(episode, multiSeason)}: {episode.name}{episode.sizeBytes ? `, ${formatBytes(episode.sizeBytes)}` : ''}{playLimit ? ' (uses a play)' : ''}">
                        {#if preparingDownload === episode.id}
                          <span class="meter meter--dot" aria-hidden="true"></span>
                        {:else}
                          <DownloadIcon />
                        {/if}
                      </button>
                    {/if}
                    {#if confirmingDownload === episode.id}
                      <div class="confirm confirm--row" role="group" aria-labelledby="confirm-{episode.id}">
                        <p class="confirm__q" id="confirm-{episode.id}">{downloadCost(episodeLabel(episode, multiSeason).toLowerCase(), episode.sizeBytes)}</p>
                        <button class={lastPlay ? 'btn-danger' : 'btn-primary'} use:focusOnMount on:keydown={escapeTo(episode.id)} on:click={() => downloadEpisode(episode)}>Download it</button>
                        <button class="btn-ghost" on:keydown={escapeTo(episode.id)} on:click={() => keepPlay(episode.id)}>Don’t download</button>
                      </div>
                    {/if}
                    {#if seen > 0}
                      <!-- How much of the episode has been seen on this link. -->
                      <span class="ep__seen" class:ep__seen--done={seen >= 1}
                            style="transform: scaleX({seen})" aria-hidden="true"></span>
                    {/if}
                  </li>
                {/each}
              </ul>
            {/if}
          </div>
        {/if}
      </div>
    </section>

    <!-- ── Credits. A spec list, not a card. ── -->
    {#if (shareInfo.directors && shareInfo.directors.length) || (shareInfo.studios && shareInfo.studios.length) || (shareInfo.actors && shareInfo.actors.length)}
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
              <dt>Starring</dt>
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
        </dl>
      </section>
    {/if}
  {/if}
</main>

<!-- Ft2 · Inline single line. Outside main, so it is the page's contentinfo. -->
{#if !playerOpen}
  <footer class="foot">
    <p>
      Shared via <a class="foot-link" href="https://github.com/stanislavhannes/jellyfin-share-backend" target="_blank" rel="noopener noreferrer">Jellyfin Share</a>
    </p>
  </footer>
{/if}

<style>
  /* Hallmark · genre: atmospheric · macrostructure: 08 Photographic
   * H6 photographic fold · nav: N9 edge-aligned · footer: Ft2 inline
   * design-system: design.md · designed-as-app
   */

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

  /* No artwork: no empty stage to wait through. The fold shrinks to the title
     on the same single bloom as the error and admin pages, and the actions
     rise to meet it. */
  .fold--bare {
    min-height: 0;
    background:
      radial-gradient(ellipse 70% 90% at 18% 0%, var(--bloom), transparent 70%),
      var(--color-paper);
  }

  /* Locked, on a phone: the password field has to sit above the keyboard, so
     the picture gives up most of the first screen. */
  @media not all and (min-width: 40rem) {
    .fold--locked:not(.fold--bare) { min-height: clamp(12rem, 32dvh, 16rem); }
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
    display: inline-flex;
    align-items: center;
    gap: var(--space-sm);
    font-family: var(--font-display);
    font-size: var(--text-md);
    letter-spacing: -0.01em;
    color: var(--color-ink);
    white-space: nowrap;
  }

  /* On a phone the first screen has to hold the title, the button and its
     terms, so the plate's bottom air is tight until 40rem. */
  .plate {
    align-self: end;
    max-width: min(40rem, 100%);
    padding: var(--space-2xl) var(--page-gutter) var(--space-md);
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

  /* The season under the show's name: a second line of the title, not a label. */
  .plate__sub {
    margin-bottom: var(--space-sm);
    font-family: var(--font-display);
    font-size: var(--text-lg);
    line-height: 1.1;
    color: var(--color-ink-2);
  }

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
    padding: var(--space-sm) var(--page-gutter) var(--space-xl);
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

  /* Doubled up so it outranks the two-column .stage of every wider query. */
  .stage.stage--noprint { grid-template-columns: minmax(0, 1fr); }

  .act {
    display: grid;
    gap: var(--space-md);
    justify-items: start;
    min-width: 0;
  }

  .lede {
    max-width: var(--measure);
    line-height: 1.65;
    color: var(--color-ink-2);
  }

  /* ── Controls ──────────────────────────────────────────────────────────── */
  .tracks { width: 100%; }

  .tracks__summary {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-2xs) var(--space-sm);
    min-height: var(--control-h);
    padding-block: var(--space-xs);
    list-style: none;
    cursor: pointer;
    border-radius: var(--radius-input);
  }

  .tracks__summary::-webkit-details-marker { display: none; }

  .tracks__now {
    font-size: var(--text-sm);
    color: var(--color-ink-2);
    overflow-wrap: anywhere;
  }

  /* Ink, not accent: Play is the one thing on the page in Jellyfin blue. */
  .tracks__change {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--color-ink);
    white-space: nowrap;
    border-bottom: var(--rule-hair) solid var(--color-ink-2);
  }

  @media (hover: hover) {
    .tracks__summary:hover .tracks__change { border-bottom-color: var(--color-ink); }
  }

  .tracks[open] .tracks__change { color: var(--color-ink-2); border-bottom-color: var(--color-rule); }

  .tracks__fields {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--space-md);
    padding-block: var(--space-xs) var(--space-2xs);
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
  input { min-width: 0; flex: 1 1 11rem; }

  @media (hover: hover) {
    select:hover, input:hover { background: var(--color-paper-3); }
  }

  select:focus-visible, input:focus-visible {
    outline-color: var(--color-focus);
    border-color: var(--color-ink-2);
  }

  input[aria-invalid="true"] { border-color: var(--color-danger); }
  select:disabled { opacity: 0.55; cursor: not-allowed; }
  input:read-only { color: var(--color-ink-2); }

  /* A grid, so on a phone Unlock stretches to the full width like every other
     action there (see .play). */
  .gate { display: grid; gap: var(--space-sm); width: 100%; max-width: 30rem; }

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

  /* On a phone Play can hold more than a line: "Resume Episode 2" and where
     it stopped, or a long "Season 2, episode 12". Rather than push the column
     past the screen's edge, it grows: the time drops under the label, and a
     label too long for the line wraps beside the glyph (see .btn__main). */
  @media not all and (min-width: 40rem) {
    .btn-primary--lg {
      flex-wrap: wrap;
      row-gap: 0;
      height: auto;
      min-height: 3.25rem;
      padding-block: var(--space-xs);
      line-height: 1.25;
      white-space: normal;
      /* Centres a wrapped label's second line, which flex alone does not. */
      text-align: center;
    }
  }

  .btn-primary :global(svg) { width: 1.15rem; height: 1.15rem; }

  /* Icons never shrink the label or get squeezed by it. */
  .btn-ghost :global(svg),
  .btn-ghost svg { width: 1.05rem; height: 1.05rem; flex-shrink: 0; }

  /* Narrow until 40rem, so Play and the first secondary action share a row. */
  .btn-ghost {
    padding-inline: var(--space-sm);
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

  /* Only ever behind an in-place confirm: the step that spends the last play. */
  .btn-danger {
    display: inline-flex;
    align-items: center;
    height: var(--control-h);
    padding-inline: var(--space-lg);
    background: var(--color-danger);
    color: var(--color-danger-ink);
    font-weight: 600;
    white-space: nowrap;
    border: none;
    border-radius: var(--radius-pill);
    cursor: pointer;
  }

  .btn-danger:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 3px; }

  .confirm {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-xs) var(--space-sm);
    flex-basis: 100%;
  }

  /* The paragraph takes the whole line so the buttons wrap below it; the
     measure applies to its text, not to the flex item. */
  .confirm__q {
    flex-basis: 100%;
    padding-inline-end: max(0px, calc(100% - var(--measure)));
    font-size: var(--text-sm);
    color: var(--color-ink);
  }

  .confirm--row { padding-bottom: var(--space-md); }

  /* On a phone every action takes the full width, Play first: a label is
     never cut short, and a size or a device name always fits. Wider screens
     put them in one row (see the 40rem query). */
  .play {
    display: grid;
    /* The track is the column's width, never its widest label's. */
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-xs);
    width: 100%;
  }

  /* Play and the line that goes with it. On a phone this box stays out of the
     layout; wider, once there is a line, it takes a row of its own. */
  .play__lead { display: contents; }

  /* Whose place Play goes back to, or why it moves on, said right at the
     button (under it on a phone, beside it where there is room), with the
     other way in as a quieter link rather than a second button. */
  .play__note {
    /* How far the link's hit area reaches below its line (see below). */
    --note-reach: 1.5rem;
    max-width: var(--measure);
    margin-block: var(--space-2xs) var(--space-xs);
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  /* With a link in it, the room below is the hit area's reach, so the reach
     can never lie over the next action. */
  .play__note--link { margin-block-end: var(--note-reach); }

  /* Ink, like "Change": it sits with Play, and the accent belongs to Play.
     Inline in the sentence, so its box is only a line high. On a phone the hit
     area grows to the 44px a thumb needs - downward only, so it never lies over
     the sentence above it: a stray tap there would spend a play. */
  .play__note .link {
    position: relative;
    margin-top: 0;
    color: var(--color-ink);
    border-bottom-color: var(--color-ink-2);
  }

  .play__note .link::after {
    content: "";
    position: absolute;
    inset: 0 -0.25rem calc(-1 * var(--note-reach));
  }

  @media (hover: hover) {
    .play__note .link:hover { border-bottom-color: var(--color-ink); }
  }

  .play__note .link:active { color: var(--color-ink-2); }

  /* The glyph and the words it belongs to, kept together when Play wraps. */
  .btn__main {
    display: inline-flex;
    align-items: center;
    gap: var(--space-xs);
    min-width: 0;
    max-width: 100%;
  }

  .btn__main :global(svg) { flex-shrink: 0; }

  /* A machine fact riding along in a button: the resume time, the format. */
  .btn__aside {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-weight: 400;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    opacity: 0.75;
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

  /* ── The terms: a headline before the button, the fine print after it ──── */
  .terms-head {
    display: grid;
    gap: var(--space-xs);
    width: 100%;
    max-width: var(--measure);
  }

  .terms-head__line {
    font-size: var(--text-base);
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--color-ink);
  }

  .terms-head__exact {
    margin-top: calc(-1 * var(--space-2xs));
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .terms-head__warn { color: var(--color-ink); }

  .terms {
    display: grid;
    gap: var(--space-xs);
    max-width: var(--measure);
  }

  .terms__line {
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  /* The moment a term is about to bite it moves up to ink - brighter, not red.
     Red stays for things that went wrong. */
  .terms__line--warn { color: var(--color-ink); }

  /* Thick enough to read as a gauge rather than a rule. */
  .fuel__track {
    width: 100%;
    max-width: 18rem;
    height: 4px;
    background: var(--color-rule);
    border-radius: var(--radius-pill);
    overflow: hidden;
  }

  .fuel__fill {
    width: 100%;
    height: 100%;
    background: var(--color-muted);
    transform-origin: left center;
    transition: transform var(--dur-long) var(--ease-out);
  }

  /* The last play is a term about to bite, not something gone wrong: the
     line brightens to ink rather than turning red. */
  .fuel__fill--low { background: var(--color-ink); }

  /* ── Episodes: an index, same voice as the admin index ─────────────────── */
  .episodes { width: 100%; margin-top: var(--space-md); }

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

  .castbar__state {
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .castbar__state strong { color: var(--color-accent); font-weight: 600; }

  .eplist { list-style: none; margin: 0; padding: 0; width: 100%; }

  /* The row owns the rule, so the play target and the download control beside
     it read as one line of the index. */
  .eprow {
    position: relative;
    display: flex;
    flex-wrap: wrap;
    align-items: stretch;
    /* Air between the row's play glyph and its download, so a thumb aimed at
       one does not land on the other. */
    gap: var(--space-md);
    border-bottom: var(--rule-hair) solid var(--color-rule-2);
  }

  .ep {
    display: grid;
    /* A zero basis so the row wraps only for a confirm, never for the
       download button beside it. */
    flex: 1 1 0;
    min-width: 0;
    grid-template-columns: 2.5rem minmax(0, 1fr) auto;
    grid-template-areas: "no body end";
    gap: var(--space-md);
    /* Centred, like the download beside it: with an overview under the name
       the row is taller than one line. */
    align-items: center;
    padding: var(--space-md) 0;
    text-align: start;
    background: none;
    border: none;
    cursor: pointer;
    transition: transform var(--dur-micro) var(--ease-out);
  }

  @media (hover: hover) {
    .ep:hover:not(:disabled) { transform: translateY(-1px); }
    .ep:hover:not(:disabled) .ep__name { color: var(--color-accent); }
  }

  /* "10.12" needs more than a lone episode number does. */
  .eplist--seasons .ep { grid-template-columns: 3.25rem minmax(0, 1fr) auto; }

  .ep__no { grid-area: no; }
  .ep__body { grid-area: body; display: grid; gap: var(--space-2xs); min-width: 0; }
  .ep__end { grid-area: end; }

  /* Jellyfin's own still for the episode, from 40rem where a row has room.
     Hidden below that, a lazy image never reaches the viewport, so phones
     do not fetch it. */
  .ep__thumb { display: none; }

  .ep__about {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--color-muted);
  }

  .ep:active:not(:disabled) { transform: translateY(1px); }

  .ep:disabled { opacity: 0.55; cursor: not-allowed; }

  /* Nothing left to play it with: the record reads as text, not as controls. */
  .ep--static { cursor: default; }
  .ep--static .ep__name { font-weight: 400; color: var(--color-ink-2); }

  /* A spent link: the statement leads where the button stood. */
  .spent {
    display: grid;
    gap: var(--space-2xs);
    max-width: var(--measure);
  }

  /* In the terms' own voice, at the headline's size: it is the last of the
     terms, not a second title under the show's. */
  .spent__head {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--color-ink);
  }

  .spent__next { color: var(--color-ink-2); }

  /* Focused only by script, when the last play ends: not controls, no ring. */
  .spent__head:focus, .terms-head__line:focus { outline: none; }

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

  .ep__end :global(svg),
  .ep__play { width: 1rem; height: 1rem; flex-shrink: 0; }

  .ep__play {
    color: var(--color-neutral);
    transition: color var(--dur-micro) var(--ease-out);
  }

  @media (hover: hover) {
    .ep:hover:not(:disabled) .ep__play { color: var(--color-accent); }
  }

  /* While a receiver is connected the glyph is the row's promise, so it takes
     the accent rather than sitting in the quiet grey. The local play arrow does
     not — it is the resting state, and every row having an accent mark would
     make the accent mean nothing. */
  .ep__end--cast :global(svg) { color: var(--color-accent); }

  .ep__dl {
    display: grid;
    place-items: center;
    flex-shrink: 0;
    width: var(--control-h);
    padding: 0;
    background: none;
    color: var(--color-neutral);
    border: none;
    border-radius: var(--radius-input);
    cursor: pointer;
    transition: color var(--dur-micro) var(--ease-out),
                background-color var(--dur-micro) var(--ease-out);
  }

  .ep__dl :global(svg) { width: 1.05rem; height: 1.05rem; }

  @media (hover: hover) {
    .ep__dl:hover:not(:disabled) { color: var(--color-ink); background: var(--color-paper-2); }
  }

  .ep__dl:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -2px; }
  .ep__dl:disabled { opacity: 0.55; cursor: not-allowed; }

  /* What this browser has seen of an episode, laid along the row's rule. */
  .ep__seen {
    position: absolute;
    left: 0;
    right: 0;
    bottom: -1px;
    height: 2px;
    background: var(--color-accent);
    transform-origin: left center;
    pointer-events: none;
  }

  .ep__seen--done { background: var(--color-accent-dim); }

  /* A spent page has nothing left to act on, so nothing on it is lit. */
  .eplist--spent .ep__seen { background: var(--color-rule); }

  .meter--dot { width: 1rem; }

  /* Watched: the tick takes the accent, the one mark in the row that is about
     the viewer rather than the file. */
  .ep__watched { display: inline-flex; color: var(--color-accent); }

  .watched {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2xs);
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .watched :global(svg) { width: 1rem; height: 1rem; color: var(--color-accent); }

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
    font-size: var(--text-sm);
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-neutral);
  }

  dd { margin: 0; min-width: 0; color: var(--color-ink-2); overflow-wrap: anywhere; }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

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
    font-size: var(--text-sm);
    line-height: 1.6;
    color: var(--color-neutral);
  }

  /* ── Wider: the print laps over the photograph's edge ──────────────────── */
  @media (min-width: 40rem) {
    .plate { max-width: min(40rem, 68%); padding-bottom: var(--space-2xl); }
    /* With no picture to lap over, the air under the title is only a gap. */
    .fold--bare .plate { padding-bottom: var(--space-sm); }
    .act { gap: var(--space-lg); }
    .play { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    .gate__submit { justify-self: start; }

    .ep--thumb {
      grid-template-columns: 2.5rem 7.5rem minmax(0, 1fr) auto;
      grid-template-areas: "no thumb body end";
    }
    .eplist--seasons .ep--thumb { grid-template-columns: 3.25rem 7.5rem minmax(0, 1fr) auto; }

    .ep__thumb {
      display: block;
      grid-area: thumb;
      width: 100%;
      aspect-ratio: 16 / 9;
      object-fit: cover;
      border-radius: var(--radius-input);
      background: var(--color-paper-2);
    }

    .eplist--spent .ep__thumb { opacity: 0.6; }

    /* Without a print the column keeps a reading width instead of running to
       the far edge. */
    .stage--noprint .act { max-width: 42rem; }
    .play > .confirm,
    .play > .castbar__state { flex-basis: 100%; }
    /* Beside Play where there is room, under it where not; either way the
       other actions start the next row. */
    .play__lead--noted {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--space-sm);
      flex-basis: 100%;
    }
    .play__note { flex: 1 1 16rem; margin-block: 0; }
    .play__note .link::after { content: none; }
    .btn-ghost { padding-inline: var(--space-lg); }

    /* The print sits beside the column it belongs to, not out at the far edge. */
    .stage {
      grid-template-columns: minmax(0, 38rem) 11rem;
      justify-content: start;
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
      grid-template-columns: minmax(0, 42rem) 13.5rem;
      gap: var(--space-2xl);
      padding-bottom: var(--space-xl);
    }

    .print { width: 13.5rem; margin-top: -9rem; }

    .credits { padding-block: var(--space-2xl); }

    .cast { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  }

  /* A wide window: the print grows with it, so the column and its picture
     still fill the stage instead of leaving the right third empty. */
  @media (min-width: 80rem) {
    .stage { grid-template-columns: minmax(0, 46rem) 18rem; }
    .print { width: 18rem; margin-top: -12rem; }
  }
</style>
