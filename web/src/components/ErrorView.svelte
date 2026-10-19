<script>
  import BrandMark from './BrandMark.svelte';
  export let error;

  // Errors name what broke, then what to do about it. No apology, no emoji, and
  // no status codes or server strings: those are for logs, not for whoever
  // opened the link.
  const errorMessages = {
    404: {
      title: 'Not found',
      description: 'This link doesn’t point at anything. Chat apps sometimes cut a link short, so check you opened all of it.',
      action: 'If it is complete, ask whoever shared it for a new link.'
    },
    410: {
      title: 'No longer available',
      description: 'This link has expired, or whoever shared it switched it off.',
      action: 'Ask whoever shared it for a new link.'
    },
    403: {
      title: 'Access denied',
      description: 'This link exists, but it doesn’t grant access to that content.',
      action: 'Check you opened the full link, including everything after the last slash.'
    },
    0: {
      title: 'Couldn’t reach the server',
      description: 'The page loaded, but the server behind it didn’t answer.',
      action: 'Check your connection and reload the page.'
    },
    default: {
      title: 'Couldn’t load the link',
      description: 'The server answered, but not with a link.',
      action: 'Reload the page. If it keeps failing, let whoever shared it know.'
    }
  };

  // Same day-month order and 24-hour clock as the share page.
  const when = new Intl.DateTimeFormat('en-GB', {
    day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', timeZoneName: 'short'
  });

  // A link that ran out says which one, why and since when - the server knows,
  // so the page does not make the viewer guess between causes.
  function goneMessage(gone) {
    const at = new Date(gone.endedAt);
    const date = Number.isNaN(at.getTime()) ? '' : ` on ${when.format(at)}`;
    const name = [gone.seriesName, gone.title].filter(Boolean).join(', ');
    const link = name ? `The link to ${name}` : 'This link';
    if (gone.reason === 'revoked') {
      return { title: 'Switched off', description: `${link} was switched off by whoever shared it${date}.`, action: errorMessages[410].action };
    }
    if (gone.reason === 'expired') {
      return { title: 'This link has expired', description: `${link} expired${date}.`, action: errorMessages[410].action };
    }
    return errorMessages[410];
  }

  $: errorInfo = error?.gone
    ? goneMessage(error.gone)
    : errorMessages[error?.status] || errorMessages.default;
</script>

<svelte:head><title>{errorInfo.title} · Jellyfin Share</title></svelte:head>

<div class="error bloom-ground">
  <header class="nav">
    <span class="wordmark"><BrandMark />Jellyfin Share</span>
  </header>

  <main class="error__body">
    <h1 class="error__title">{errorInfo.title}</h1>
    <p class="error__lede">{errorInfo.description}</p>
    <p class="error__action">{errorInfo.action}</p>
  </main>

  <footer class="foot">
    <p>Shared via <a class="foot-link" href="https://github.com/stanislavhannes/jellyfin-share-backend" target="_blank" rel="noopener noreferrer">Jellyfin Share</a></p>
  </footer>
</div>

<style>
  /* Hallmark · genre: atmospheric · macrostructure: 08 Photographic (bare fold)
   * nav: N9 edge-aligned · footer: Ft2 inline single line
   * design-system: design.md · designed-as-app
   */
  /* No card, no centred column, no icon. The statement sits low-left against a
     single accent bloom; the emptiness above it is the design. */
  .error {
    display: grid;
    grid-template-rows: auto 1fr auto;
    min-height: 100dvh;
    padding-inline: var(--page-gutter);
  }

  .error > * { position: relative; z-index: var(--z-base); }

  /* N9 · Edge-aligned minimal — wordmark hard-left, nothing else: the statement below is the status. */
  .nav {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    padding-block: var(--space-lg);
  }

  .wordmark {
    display: inline-flex;
    align-items: center;
    gap: var(--space-sm);
    font-family: var(--font-display);
    font-size: var(--text-md);
    letter-spacing: -0.01em;
    color: var(--color-ink-2);
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

  /* Ft2 · Inline single line */
  .foot {
    padding-block: var(--space-md) var(--space-lg);
    padding-bottom: max(var(--space-lg), env(safe-area-inset-bottom));
    border-top: var(--rule-hair) solid var(--color-rule-2);
    font-size: var(--text-sm);
    color: var(--color-neutral);
  }

  @media (min-width: 60rem) {
    .error__body { padding-block: var(--space-3xl) var(--space-4xl); }
  }
</style>
