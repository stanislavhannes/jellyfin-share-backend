<script>
  import { onMount, createEventDispatcher } from 'svelte';

  export let apiKey;

  const dispatch = createEventDispatcher();

  let shares = [];
  let loading = true;
  let error = null;
  let selectedShare = null;
  let shareDetails = null;
  let detailsLoading = false;
  let dialogEl;

  // Revoking kills live sessions, so it asks — but in the row, not behind a
  // native confirm() the browser paints over the page.
  let confirmingId = null;
  let revokeError = '';
  // Copy has no visible effect, so the button says what happened, briefly.
  let copiedId = null;
  let copyTimer;

  onMount(() => {
    loadShares();
    return () => clearTimeout(copyTimer);
  });

  async function loadShares() {
    loading = true;
    error = null;
    try {
      const response = await fetch('/api/admin/shares', {
        headers: { 'X-Backend-Key': apiKey }
      });
      if (!response.ok) throw new Error('The backend refused the request. Check the key is still valid.');
      const data = await response.json();
      shares = data.shares || [];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function viewDetails(share) {
    selectedShare = share;
    shareDetails = null;
    detailsLoading = true;
    dialogEl?.showModal();
    try {
      const response = await fetch(`/api/admin/shares/${share.id}`, {
        headers: { 'X-Backend-Key': apiKey }
      });
      if (!response.ok) throw new Error('Failed to load details');
      shareDetails = await response.json();
    } catch (e) {
      shareDetails = null;
    } finally {
      detailsLoading = false;
    }
  }

  async function revokeShare(share) {
    confirmingId = null;
    revokeError = '';
    try {
      const response = await fetch(`/api/admin/shares/${share.id}/revoke`, {
        method: 'POST',
        headers: { 'X-Backend-Key': apiKey }
      });
      if (!response.ok) throw new Error('revoke failed');
      loadShares();
      if (selectedShare?.id === share.id) closeDetails();
    } catch (e) {
      revokeError = `Couldn’t revoke “${share.title}”. The link is still live — try again.`;
    }
  }

  function closeDetails() {
    dialogEl?.close();
  }

  function onDialogClose() {
    selectedShare = null;
    shareDetails = null;
  }

  function onDialogClick(event) {
    // Light-dismiss: a click that lands on the dialog element itself is a
    // click on its backdrop, since the panel fills it.
    if (event.target === dialogEl) closeDetails();
  }

  function logout() {
    dispatch('logout');
  }

  function getStatus(share) {
    if (share.revokedAt) return 'revoked';
    if (share.expiresAt && new Date(share.expiresAt) < new Date()) return 'expired';
    return 'active';
  }

  // Absolute times render in the reader's own zone, and name it: an admin
  // deciding whether a link is still live should not have to work out whether
  // the server meant UTC. The browser's locale picks the field order.
  const absoluteTime = new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZoneName: 'short'
  });

  function formatDate(dateStr) {
    if (!dateStr) return 'Never';
    const d = new Date(dateStr);
    return Number.isNaN(d.getTime()) ? '—' : absoluteTime.format(d);
  }

  function formatTimeAgo(dateStr) {
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now - date;
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days} d ago`;
    if (hours > 0) return `${hours} h ago`;
    if (minutes > 0) return `${minutes} min ago`;
    return 'just now';
  }

  function formatExpiry(dateStr) {
    if (!dateStr) return 'Never';
    const date = new Date(dateStr);
    const now = new Date();
    const diff = date - now;

    if (diff <= 0) return 'Expired';

    const hours = Math.floor(diff / (1000 * 60 * 60));
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days} d ${hours % 24} h`;
    if (hours > 0) return `${hours} h`;
    return `${Math.floor(diff / (1000 * 60))} min`;
  }

  function copyLink(share) {
    const url = `${window.location.origin}/s/${share.publicToken}`;
    navigator.clipboard.writeText(url);
    clearTimeout(copyTimer);
    copiedId = share.id;
    copyTimer = setTimeout(() => (copiedId = null), 1600);
  }

  $: liveCount = shares.filter((s) => getStatus(s) === 'active').length;
</script>

<!-- N5 · Floating pill. Content-sized, visibly detached, blurred over the canvas. -->
<nav class="pill" aria-label="Admin">
  <span class="pill__wordmark">Jellyfin Share</span>
  <span class="pill__count" aria-live="polite">{shares.length} links</span>
  <button class="pill__icon" on:click={loadShares} disabled={loading} aria-label="Reload the index">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
      <path d="M3 12a9 9 0 019-9 9.75 9.75 0 016.74 2.74L21 8M21 3v5h-5M21 12a9 9 0 01-9 9 9.75 9.75 0 01-6.74-2.74L3 16M3 21v-5h5"/>
    </svg>
  </button>
  <button class="pill__out" on:click={logout}>Sign out</button>
</nav>

<div class="admin bloom-ground">
  <main class="index">
    <p class="index__lede">
      Every link you have handed out, and what it is doing right now.
      {#if !loading && !error && shares.length > 0}
        <span class="index__live">{liveCount} of {shares.length} still live.</span>
      {/if}
    </p>

    {#if loading}
      <div class="state">
        <div class="state__meter"></div>
        <p>Reading the index</p>
      </div>
    {:else if error}
      <div class="state state--error">
        <p>{error}</p>
        <button class="btn-ghost" on:click={loadShares}>Try again</button>
      </div>
    {:else if shares.length === 0}
      <div class="state">
        <p class="state__head">No links yet.</p>
        <p>A link appears here the moment someone shares an item from Jellyfin.</p>
      </div>
    {:else}
      {#if revokeError}
        <p class="banner">{revokeError}</p>
      {/if}

      <div class="labels" aria-hidden="true">
        <span>Item</span><span>Status</span><span>Plays</span><span>Expires</span><span></span>
      </div>

      <ul class="rows">
        {#each shares as share (share.id)}
          {@const status = getStatus(share)}
          <li class="row" class:row--spent={status !== 'active'}>
            <button class="row__item" on:click={() => viewDetails(share)}>
              <span class="row__title">{share.title}</span>
              <span class="row__sub">
                {share.itemType} · created {formatTimeAgo(share.createdAt)}{#if share.hasPassword}{' · password'}{/if}
              </span>
            </button>

            <span class="row__status" data-status={status}>
              <span class="dot" aria-hidden="true"></span>{status}
            </span>

            <span class="row__num">
              {share.totalPlays}{#if share.maxTotalPlays}&thinsp;/&thinsp;{share.maxTotalPlays}{/if}
              {#if share.currentConcurrentViewers > 0}
                <span class="row__watching">{share.currentConcurrentViewers} watching</span>
              {/if}
            </span>

            <span class="row__num row__expiry">
              {status === 'revoked' ? 'Revoked' : formatExpiry(share.expiresAt)}
            </span>

            <span class="row__actions">
              {#if confirmingId === share.id}
                <span class="confirm">
                  <button class="btn-danger" on:click={() => revokeShare(share)}>Revoke it</button>
                  <button class="btn-ghost" on:click={() => (confirmingId = null)}>Keep</button>
                </span>
              {:else}
                <button class="btn-ghost" on:click={() => copyLink(share)}>
                  {copiedId === share.id ? 'Copied' : 'Copy link'}
                </button>
                {#if status === 'active'}
                  <button class="btn-ghost btn-ghost--warn" on:click={() => (confirmingId = share.id)}>
                    Revoke
                  </button>
                {/if}
              {/if}
            </span>
          </li>
        {/each}
      </ul>
    {/if}
  </main>

  <!-- Ft4 · Dense colophon -->
  <footer class="colophon">
    <p class="colophon__line">
      <a class="foot-link" href="https://github.com/stanislavhannes/jellyfin-share-backend" target="_blank" rel="noopener noreferrer">Jellyfin Share</a>
      · admin · {shares.length} indexed, {liveCount} live
    </p>
    <p class="colophon__note">
      The backend key is kept in this browser’s localStorage and sent only as an
      X-Backend-Key header. Signing out clears it.
    </p>
  </footer>
</div>

<!-- Light-dismiss on backdrop click. <dialog> already handles Escape natively and
     the panel carries an explicit close button, so the keyboard path is covered. -->
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialogEl} class="sheet" on:close={onDialogClose} on:click={onDialogClick}>
  <div class="sheet__panel">
    <header class="sheet__head">
      <h2>{selectedShare?.title || ''}</h2>
      <button class="pill__icon" on:click={closeDetails} aria-label="Close details">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
          <path d="M18 6L6 18M6 6l12 12"/>
        </svg>
      </button>
    </header>

    {#if detailsLoading}
      <div class="state"><div class="state__meter"></div><p>Loading</p></div>
    {:else if shareDetails}
      <dl class="spec">
        <div><dt>Token</dt><dd class="spec__mono">{shareDetails.share.publicToken}</dd></div>
        <div><dt>Item type</dt><dd>{shareDetails.share.itemType}</dd></div>
        <div>
          <dt>Total plays</dt>
          <dd class="spec__mono">
            {shareDetails.share.totalPlays}{#if shareDetails.share.maxTotalPlays}&thinsp;/&thinsp;{shareDetails.share.maxTotalPlays.Int64}{/if}
          </dd>
        </div>
        <div>
          <dt>Watching now</dt>
          <dd class="spec__mono">
            {shareDetails.share.currentConcurrentViewers}{#if shareDetails.share.maxConcurrentViewers}&thinsp;/&thinsp;{shareDetails.share.maxConcurrentViewers.Int64}{/if}
          </dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd class="spec__mono">
            <time datetime={shareDetails.share.createdAt}>{formatDate(shareDetails.share.createdAt)}</time>
          </dd>
        </div>
        <div>
          <dt>Expires</dt>
          <dd class="spec__mono">
            {#if shareDetails.share.expiresAt}
              <time datetime={shareDetails.share.expiresAt}>{formatDate(shareDetails.share.expiresAt)}</time>
            {:else}
              Never
            {/if}
          </dd>
        </div>
      </dl>

      <h3 class="sheet__sub">
        Sessions
        {#if shareDetails.sessions?.length}<span class="sheet__n">{shareDetails.sessions.length}</span>{/if}
      </h3>

      {#if shareDetails.sessions && shareDetails.sessions.length > 0}
        <ul class="sessions">
          {#each shareDetails.sessions as session}
            <li class="session">
              <span class="session__state" class:session__state--live={!session.finishedAt}>
                {session.finishedAt ? 'ended' : 'live'}
              </span>
              <span class="session__when">started {formatTimeAgo(session.startedAt)}</span>
              {#if session.userAgent}
                <span class="session__ua">{session.userAgent.String}</span>
              {/if}
            </li>
          {/each}
        </ul>
      {:else}
        <p class="sheet__empty">Nobody has opened this link yet.</p>
      {/if}
    {:else}
      <p class="sheet__empty">Couldn’t load the detail for this link.</p>
    {/if}
  </div>
</dialog>

<style>
  /* Hallmark · genre: atmospheric · macrostructure: 13 Index-First
   * nav: N5 floating pill · footer: Ft4 dense colophon
   * design-system: design.md · designed-as-app
   */
  /* 13 · Index-First. The page IS the list: hairline rules between rows,
     the title is the button, no table chrome, no cards. */
  .admin {
    display: grid;
    grid-template-rows: 1fr auto;
    min-height: 100dvh;
    padding-inline: var(--page-gutter);
  }

  .admin > * { position: relative; z-index: var(--z-base); }

  /* ---- N5 · Floating pill ---- */
  .pill {
    position: fixed;
    inset: var(--space-md) auto auto 50%;
    transform: translateX(-50%);
    display: inline-flex;
    align-items: center;
    gap: var(--space-sm);
    max-width: calc(100% - 2 * var(--space-md));
    padding: var(--space-2xs) var(--space-2xs) var(--space-2xs) var(--space-md);
    background: color-mix(in oklch, var(--color-paper-2) 76%, transparent);
    backdrop-filter: blur(14px) saturate(130%);
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius-pill);
    z-index: var(--z-sticky);
  }

  .pill__wordmark {
    font-family: var(--font-display);
    font-size: var(--text-md);
    letter-spacing: -0.01em;
    white-space: nowrap;
  }

  .pill__count {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    color: var(--color-muted);
    white-space: nowrap;
  }

  .pill__icon {
    display: grid;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    background: none;
    color: var(--color-ink-2);
    border: none;
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: color var(--dur-micro) var(--ease-out),
                background-color var(--dur-micro) var(--ease-out);
  }

  /* 44px hit target without growing the glyph. */
  .pill__icon::before { content: ""; position: absolute; inset: -0.35rem; }
  .pill__icon { position: relative; }

  .pill__icon svg { width: 1.05rem; height: 1.05rem; }

  @media (hover: hover) {
    .pill__icon:hover:not(:disabled) { color: var(--color-accent); background: var(--color-paper-3); }
  }

  .pill__icon:active:not(:disabled) { transform: translateY(1px); }

  .pill__icon:disabled { opacity: 0.45; cursor: not-allowed; }

  .pill__out {
    height: 2.25rem;
    padding-inline: var(--space-md);
    background: var(--color-paper-3);
    color: var(--color-ink);
    font-size: var(--text-sm);
    font-weight: 600;
    white-space: nowrap;
    border: none;
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: background-color var(--dur-micro) var(--ease-out);
  }

  @media (hover: hover) {
    .pill__out:hover { background: var(--color-rule); }
  }

  .pill__out:active { transform: translateY(1px); }

  /* ---- The index ---- */
  .index {
    padding-block: calc(var(--space-3xl) + var(--space-md)) var(--space-2xl);
    max-width: 78rem;
    width: 100%;
  }

  .index__lede {
    max-width: 54ch;
    color: var(--color-muted);
    margin-bottom: var(--space-2xl);
  }

  .index__live { color: var(--color-ink-2); }

  .labels {
    display: none;
    padding-block: var(--space-xs);
    border-bottom: var(--rule-hair) solid var(--color-rule);
    font-size: var(--text-xs);
    font-weight: 600;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--color-neutral);
  }

  .rows { list-style: none; margin: 0; padding: 0; }

  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-xs) var(--space-md);
    align-items: baseline;
    padding-block: var(--space-md);
    border-bottom: var(--rule-hair) solid var(--color-rule-2);
    transition: transform var(--dur-micro) var(--ease-out);
  }

  @media (hover: hover) {
    .row:hover { transform: translateY(-1px); }
    .row:hover .row__title { color: var(--color-accent); }
  }

  .row--spent .row__title,
  .row--spent .row__num { color: var(--color-neutral); }

  .row__item {
    display: grid;
    gap: var(--space-3xs);
    justify-items: start;
    text-align: start;
    min-width: 0;
    padding: 0;
    background: none;
    border: none;
    cursor: pointer;
  }

  .row__title {
    font-weight: 600;
    line-height: 1.3;
    color: var(--color-ink);
    overflow-wrap: anywhere;
    transition: color var(--dur-micro) var(--ease-out);
  }

  .row__sub {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .row__status {
    display: inline-flex;
    align-items: center;
    gap: var(--space-xs);
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    letter-spacing: 0.06em;
    color: var(--color-muted);
    white-space: nowrap;
  }

  /* The word carries the state; the dot only reinforces it. Never colour alone. */
  .dot { width: 0.4rem; height: 0.4rem; border-radius: var(--radius-pill); background: var(--color-neutral); }
  .row__status[data-status="active"] { color: var(--color-accent); }
  .row__status[data-status="active"] .dot { background: var(--color-accent); }
  .row__status[data-status="revoked"] { color: var(--color-danger); }
  .row__status[data-status="revoked"] .dot { background: var(--color-danger); }

  .row__num {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    color: var(--color-ink-2);
    white-space: nowrap;
  }

  .row__watching { color: var(--color-accent); }

  .row__actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-xs);
  }

  .confirm { display: inline-flex; gap: var(--space-xs); }

  .btn-ghost, .btn-danger {
    min-height: 2.25rem;
    padding-inline: var(--space-sm);
    font-size: var(--text-sm);
    font-weight: 600;
    white-space: nowrap;
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: background-color var(--dur-micro) var(--ease-out),
                color var(--dur-micro) var(--ease-out);
  }

  .btn-ghost {
    background: none;
    color: var(--color-muted);
    border: var(--rule-hair) solid var(--color-rule);
  }

  @media (hover: hover) {
    .btn-ghost:hover { background: var(--color-paper-2); color: var(--color-ink); }
    .btn-ghost--warn:hover { color: var(--color-danger); border-color: var(--color-danger); }
  }

  .btn-danger {
    background: var(--color-danger);
    color: var(--color-danger-ink);
    border: var(--rule-hair) solid var(--color-danger);
  }

  .btn-danger:focus-visible { outline-offset: 3px; }

  .btn-ghost:active, .btn-danger:active { transform: translateY(1px); }

  .btn-ghost:disabled, .btn-danger:disabled { opacity: 0.55; cursor: not-allowed; transform: none; }

  .row__item:active .row__title { color: var(--color-ink-2); }

  .row__item:disabled { opacity: 0.55; cursor: not-allowed; }

  /* ---- States ---- */
  .state {
    display: grid;
    gap: var(--space-sm);
    justify-items: start;
    padding-block: var(--space-2xl);
    color: var(--color-muted);
  }

  .state__head { font-size: var(--text-md); color: var(--color-ink); }

  .state--error { color: var(--color-danger); }

  .state__meter {
    width: 7.5rem;
    height: 2px;
    background: var(--color-rule);
    overflow: hidden;
  }

  .state__meter::after {
    content: "";
    display: block;
    width: 40%;
    height: 100%;
    background: var(--color-accent);
    animation: sweep 1.1s var(--ease-in-out) infinite;
  }

  @keyframes sweep {
    0%   { transform: translateX(-110%); }
    100% { transform: translateX(260%); }
  }

  @media (prefers-reduced-motion: reduce) {
    .state__meter::after { width: 100%; }
  }

  .banner {
    margin-bottom: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    background: var(--color-paper-2);
    border-radius: var(--radius-card);
    color: var(--color-danger);
    font-size: var(--text-sm);
  }

  /* ---- Ft4 · Dense colophon ---- */
  .colophon {
    display: grid;
    gap: var(--space-2xs);
    padding-block: var(--space-lg);
    padding-bottom: max(var(--space-lg), env(safe-area-inset-bottom));
    border-top: var(--rule-hair) solid var(--color-rule-2);
    font-size: var(--text-sm);
    line-height: 1.55;
    color: var(--color-neutral);
  }

  /* Identity and counts are machine facts, so they keep the mono register. */
  .colophon__line {
    font-family: var(--font-outlier);
    font-variant-numeric: tabular-nums;
  }

  /* The note is a sentence, so it is set as one — a mono wall was what made the
     old footer read as noise rather than as a closing remark. */
  .colophon__note { max-width: 68ch; }

  /* ---- Detail sheet ---- */
  .sheet {
    position: fixed;
    inset: 0;
    margin: auto;
    width: min(46rem, calc(100% - 2 * var(--space-md)));
    height: fit-content;
    max-height: min(84dvh, 46rem);
    padding: 0;
    background: var(--color-paper-2);
    color: var(--color-ink);
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius-card);
    overflow: auto;
  }

  .sheet::backdrop { background: var(--scrim-strong); }

  .sheet__panel { padding: var(--space-lg); }

  .sheet__head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-md);
    margin-bottom: var(--space-lg);
  }

  .sheet__head h2 {
    font-size: var(--text-lg);
    overflow-wrap: anywhere;
    min-width: 0;
  }

  .spec {
    display: grid;
    gap: var(--space-md);
    margin: 0 0 var(--space-xl);
  }

  .spec > div {
    display: grid;
    grid-template-columns: 9.5rem minmax(0, 1fr);
    gap: var(--space-md);
    align-items: baseline;
    padding-bottom: var(--space-sm);
    border-bottom: var(--rule-hair) solid var(--color-rule-2);
  }

  dt {
    font-size: var(--text-sm);
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-neutral);
  }

  dd { margin: 0; overflow-wrap: anywhere; }

  .spec__mono {
    font-family: var(--font-outlier);
    font-variant-numeric: tabular-nums;
    color: var(--color-ink-2);
  }

  .sheet__sub {
    display: flex;
    align-items: baseline;
    gap: var(--space-xs);
    font-size: var(--text-md);
    margin-bottom: var(--space-md);
  }

  .sheet__n {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .sessions { list-style: none; margin: 0; padding: 0; }

  .session {
    display: grid;
    gap: var(--space-3xs);
    padding-block: var(--space-sm);
    border-bottom: var(--rule-hair) solid var(--color-rule-2);
  }

  .session__state {
    font-size: var(--text-sm);
    font-weight: 600;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--color-neutral);
  }

  .session__state--live { color: var(--color-accent); }

  .session__when { font-size: var(--text-sm); color: var(--color-ink-2); }

  .session__ua {
    font-family: var(--font-outlier);
    font-size: var(--text-xs);
    color: var(--color-neutral);
    overflow-wrap: anywhere;
  }

  .sheet__empty { color: var(--color-muted); }

  /* ---- Wider than a phone: the index gets its columns ---- */
  @media (min-width: 60rem) {
    .labels,
    .row {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 7rem 8rem 7rem 13.5rem;
      gap: var(--space-md);
      align-items: baseline;
    }

    .row { padding-block: var(--space-lg); }

    .row__actions { justify-content: flex-end; }

    .row__watching { display: block; }
  }

  @media (max-width: 60rem) {
    .row__watching { margin-inline-start: var(--space-xs); }

    .row__status,
    .row__num { font-size: var(--text-sm); }

    .row__actions { margin-top: var(--space-2xs); }

    .spec > div { grid-template-columns: minmax(0, 1fr); gap: var(--space-2xs); }
  }

  @media (max-width: 40rem) {
    .pill { gap: var(--space-xs); padding-left: var(--space-sm); }
    .pill__count { display: none; }
    .pill__wordmark { font-size: var(--text-base); }
  }
</style>
