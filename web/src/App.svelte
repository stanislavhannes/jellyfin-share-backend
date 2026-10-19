<script>
  import { onMount } from 'svelte';
  import ShareView from './components/ShareView.svelte';
  import ErrorView from './components/ErrorView.svelte';
  import Admin from './components/Admin.svelte';

  let token = '';
  let loading = true;
  let error = null;
  let shareInfo = null;
  let isAdminRoute = false;

  onMount(() => {
    const path = window.location.pathname;

    // Check if admin route
    if (path === '/admin' || path.startsWith('/admin/')) {
      isAdminRoute = true;
      loading = false;
      return;
    }

    // Extract token from URL path: /s/{token}
    const pathParts = path.split('/');
    const sIndex = pathParts.indexOf('s');
    if (sIndex !== -1 && pathParts[sIndex + 1]) {
      token = pathParts[sIndex + 1];
      loadShareInfo();
    } else {
      error = { message: 'Invalid share link', status: 400 };
      loading = false;
    }
  });

  async function loadShareInfo() {
    try {
      const response = await fetch(`/api/public/shares/${token}`);

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        if (response.status === 404) {
          error = { status: 404 };
        } else if (response.status === 410) {
          // Why it ended, when, and which link it was - when the server says.
          error = { status: 410, gone: data.reason ? data : null };
        } else {
          error = { status: response.status };
        }
      } else {
        shareInfo = await response.json();
      }
    } catch (e) {
      error = { status: 0 };
    } finally {
      loading = false;
    }
  }

  function handlePasswordSuccess() {
    // Password validated - ShareView handles the state internally
  }
</script>

<!-- Each view names its own main and footer: a footer inside a shared main
     would not count as the page's contentinfo. -->
<div class="app">
  {#if isAdminRoute}
    <Admin />
  {:else if loading}
    <main class="boot">
      <div class="boot__meter"></div>
      <p class="boot__label">Opening the link</p>
    </main>
  {:else if error}
    <ErrorView {error} />
  {:else if shareInfo}
    <ShareView {shareInfo} {token} on:passwordSuccess={handlePasswordSuccess} />
  {/if}
</div>

<style>
  .app {
    min-height: 100dvh;
  }

  /* Matches the pre-JS boot state in index.html exactly, so the handover
     from static HTML to Svelte is invisible. */
  .boot {
    display: grid;
    place-items: center;
    gap: var(--space-md);
    min-height: 100dvh;
  }

  .boot__meter {
    width: 7.5rem;
    height: 2px;
    background: var(--color-rule);
    overflow: hidden;
  }

  .boot__meter::after {
    content: "";
    display: block;
    width: 40%;
    height: 100%;
    background: var(--color-accent);
    animation: boot-sweep 1.1s var(--ease-in-out) infinite;
  }

  .boot__label {
    font-family: var(--font-outlier);
    font-size: var(--text-sm);
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--color-muted);
  }

  @keyframes boot-sweep {
    0%   { transform: translateX(-110%); }
    100% { transform: translateX(260%); }
  }

  @media (prefers-reduced-motion: reduce) {
    .boot__meter::after { width: 100%; }
  }
</style>
