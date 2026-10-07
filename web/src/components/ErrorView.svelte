<script>
  export let error;

  // Errors name what broke, then what to do about it. No apology, no emoji —
  // the status code carries the diagnosis for anyone who needs it.
  const errorMessages = {
    404: {
      title: 'Not found',
      description: 'This link doesn’t point at anything. It was either never created, or it has been deleted since.',
      action: 'Ask whoever sent it for a fresh link.'
    },
    410: {
      title: 'No longer available',
      description: 'The link has run out — it expired, hit its play limit, or the owner revoked it.',
      action: 'Ask whoever sent it for a fresh link.'
    },
    403: {
      title: 'Access denied',
      description: 'This link exists, but it doesn’t grant access to that content.',
      action: 'Check you opened the full link, including everything after the last slash.'
    },
    default: {
      title: 'Couldn’t load the share',
      description: 'The server answered, but not with a share.',
      action: 'Reload the page. If it keeps failing, the backend may be down.'
    }
  };

  $: errorInfo = errorMessages[error?.status] || errorMessages.default;
  $: detail = error?.message && error.message !== errorInfo.description ? error.message : '';
</script>

<div class="error bloom-ground">
  <header class="nav">
    <span class="wordmark">Jellyfin Share</span>
    <span class="nav__status">{error?.status || '—'}</span>
  </header>

  <div class="error__body">
    <h1 class="error__title">{errorInfo.title}</h1>
    <p class="error__lede">{errorInfo.description}</p>
    <p class="error__action">{errorInfo.action}</p>

    {#if detail}
      <p class="error__detail">{detail}</p>
    {/if}
  </div>

  <footer class="foot">
    <p>Shared via <a class="foot-link" href="https://github.com/stanislavhannes/jellyfin-share-backend" target="_blank" rel="noopener noreferrer">Jellyfin Share</a> · nothing was sent to the server</p>
  </footer>
</div>

<style>
  /* Hallmark · genre: atmospheric · macrostructure: 08 Photographic (bare fold)
   * nav: N9 edge-aligned · footer: Ft2 inline single line
   * design-system: design.md · designed-as-app
   */
  /* No card, no centred column, no icon. The statement sits low-left against a
     single warm bloom; the emptiness above it is the design. */
  .error {
    display: grid;
    grid-template-rows: auto 1fr auto;
    min-height: 100dvh;
    padding-inline: var(--page-gutter);
  }

  .error > * { position: relative; z-index: var(--z-base); }

  /* N9 · Edge-aligned minimal — wordmark hard-left, status hard-right, nothing between. */
  .nav {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    padding-block: var(--space-lg);
  }

  .wordmark {
    font-family: var(--font-display);
    font-size: var(--text-md);
    letter-spacing: -0.01em;
    color: var(--color-ink-2);
    white-space: nowrap;
  }

  .nav__status {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
    letter-spacing: 0.08em;
    color: var(--color-muted);
    white-space: nowrap;
  }

  .error__body {
    align-self: end;
    max-width: var(--measure);
    padding-block: var(--space-2xl) var(--space-3xl);
  }

  .error__title {
    font-size: var(--text-display);
    overflow-wrap: anywhere;
    min-width: 0;
    margin-bottom: var(--space-lg);
  }

  .error__lede {
    font-size: var(--text-md);
    line-height: 1.55;
    color: var(--color-ink-2);
    margin-bottom: var(--space-sm);
  }

  .error__action {
    color: var(--color-muted);
  }

  .error__detail {
    margin-top: var(--space-xl);
    padding-top: var(--space-md);
    border-top: var(--rule-hair) solid var(--color-rule-2);
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    color: var(--color-muted);
    overflow-wrap: anywhere;
  }

  /* Ft2 · Inline single line */
  .foot {
    padding-block: var(--space-md) var(--space-lg);
    padding-bottom: max(var(--space-lg), env(safe-area-inset-bottom));
    border-top: var(--rule-hair) solid var(--color-rule-2);
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    color: var(--color-neutral);
  }

  @media (min-width: 60rem) {
    .error__body { padding-block: var(--space-3xl) var(--space-4xl); }
  }
</style>
