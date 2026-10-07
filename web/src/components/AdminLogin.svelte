<script>
  import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  let apiKey = '';
  let error = '';
  let loading = false;
  // Validate on blur, not on every keystroke — the field only starts
  // complaining once the user has left it at least once.
  let touched = false;

  $: empty = !apiKey.trim();
  $: showEmptyError = touched && empty;

  async function login() {
    touched = true;
    if (empty) {
      error = 'The key is empty. Paste the value of JF_SHARE_BACKEND_KEY from your .env.';
      return;
    }

    loading = true;
    error = '';

    try {
      const response = await fetch('/api/admin/stats', {
        headers: { 'X-Backend-Key': apiKey }
      });

      if (response.ok) {
        dispatch('login', { apiKey });
      } else {
        error = 'The backend rejected that key. Check it matches JF_SHARE_BACKEND_KEY on the server.';
      }
    } catch (e) {
      error = 'Couldn’t reach the backend. Check it is running and that /api is reachable from this host.';
    } finally {
      loading = false;
    }
  }
</script>

<div class="login bloom-ground">
  <header class="nav">
    <span class="wordmark">Jellyfin Share</span>
    <span class="nav__status">admin</span>
  </header>

  <div class="login__body">
    <h1 class="login__title">Sign in</h1>
    <p class="login__lede">
      The admin surface is held open by the backend key, not by an account.
      Paste it once and this browser keeps it.
    </p>

    <form on:submit|preventDefault={login} novalidate>
      <div class="field">
        <label for="apiKey">Backend API key</label>
        <input
          type="password"
          id="apiKey"
          autocomplete="off"
          spellcheck="false"
          placeholder="JF_SHARE_BACKEND_KEY"
          bind:value={apiKey}
          on:blur={() => (touched = true)}
          aria-invalid={error || showEmptyError ? 'true' : 'false'}
          aria-describedby="apiKey-help"
        />
        <p class="field__help" id="apiKey-help" class:field__help--error={error}>
          {#if error}
            {error}
          {:else}
            Stored in this browser’s localStorage. Signing out clears it.
          {/if}
        </p>
      </div>

      <button type="submit" class="submit" disabled={loading}>
        {#if loading}
          <span class="submit__meter" aria-hidden="true"></span>
          <span>Checking the key</span>
        {:else}
          <span>Sign in</span>
        {/if}
      </button>
    </form>
  </div>

  <footer class="foot">
    <p class="foot__line">
      <a class="foot-link" href="https://github.com/stanislavhannes/jellyfin-share-backend" target="_blank" rel="noopener noreferrer">Jellyfin Share</a>
      · admin
    </p>
    <p class="foot__note">
      The key never leaves this browser except as an X-Backend-Key header.
    </p>
  </footer>
</div>

<style>
  /* Hallmark · genre: atmospheric · macrostructure: 13 Index-First (single-field variant)
   * nav: N9 edge-aligned · footer: Ft4 dense colophon
   * design-system: design.md · designed-as-app
   */
  /* App family, single-field variant: left-biased, no card, one warm bloom.
     The old centred login-box was the most templated shape in the app. */
  .login {
    display: grid;
    grid-template-rows: auto 1fr auto;
    min-height: 100dvh;
    padding-inline: var(--page-gutter);
  }

  .login > * { position: relative; z-index: var(--z-base); }

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
    font-size: var(--text-sm);
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-muted);
    white-space: nowrap;
  }

  .login__body {
    align-self: center;
    width: 100%;
    max-width: 34rem;
    padding-block: var(--space-2xl);
  }

  .login__title {
    font-size: var(--text-display-s);
    overflow-wrap: anywhere;
    min-width: 0;
    margin-bottom: var(--space-sm);
  }

  .login__lede {
    max-width: 48ch;
    color: var(--color-muted);
    margin-bottom: var(--space-xl);
  }

  .field {
    display: grid;
    gap: var(--space-2xs);
    margin-bottom: var(--space-lg);
  }

  label {
    font-size: var(--text-sm);
    font-weight: 600;
    letter-spacing: 0.02em;
    color: var(--color-ink-2);
  }

  input {
    width: 100%;
    height: var(--control-h);
    padding-inline: var(--space-sm);
    background: var(--color-paper-2);
    color: var(--color-ink);
    font-family: var(--font-outlier);
    /* 1px in every state — the box never changes geometry on focus or error. */
    border: var(--rule-hair) solid var(--color-rule);
    border-radius: var(--radius-input);
    outline: 2px solid transparent;
    outline-offset: 1px;
    transition: background-color var(--dur-micro) var(--ease-out),
                border-color var(--dur-micro) var(--ease-out);
  }

  input::placeholder { color: var(--color-neutral); }

  @media (hover: hover) {
    input:hover { background: var(--color-paper-3); }
  }

  input:focus-visible {
    outline-color: var(--color-focus);
    border-color: var(--color-ink-2);
  }

  input[aria-invalid="true"] { border-color: var(--color-danger); }

  input:disabled { opacity: 0.55; cursor: not-allowed; }

  .field__help {
    /* Reserved height, so an error never pushes the button down. */
    min-height: 1lh;
    font-size: var(--text-sm);
    color: var(--color-muted);
  }

  .field__help--error { color: var(--color-danger); }

  .submit {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-xs);
    height: var(--control-h);
    padding-inline: var(--space-lg);
    background: var(--color-accent);
    color: var(--color-accent-ink);
    font-weight: 600;
    letter-spacing: 0.01em;
    white-space: nowrap;
    border: none;
    border-radius: var(--radius-pill);
    cursor: pointer;
    transition: transform var(--dur-micro) var(--ease-out),
                background-color var(--dur-micro) var(--ease-out);
  }

  @media (hover: hover) {
    .submit:hover:not(:disabled) { transform: translateY(-1px); }
  }

  .submit:active:not(:disabled) { transform: translateY(1px); }

  /* Ring sits clear of the accent fill, where it has paper to contrast against. */
  .submit:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 3px; }

  .submit:disabled { opacity: 0.55; cursor: not-allowed; }

  .submit__meter {
    width: 1.5rem;
    height: 2px;
    background: var(--color-accent-ink);
    opacity: 0.35;
    overflow: hidden;
    position: relative;
  }

  .submit__meter::after {
    content: "";
    position: absolute;
    inset: 0 auto 0 0;
    width: 45%;
    background: var(--color-accent-ink);
    animation: sweep 900ms var(--ease-in-out) infinite;
  }

  @keyframes sweep {
    0%   { transform: translateX(-110%); }
    100% { transform: translateX(240%); }
  }

  @media (prefers-reduced-motion: reduce) {
    .submit__meter::after { width: 100%; }
  }

  /* Ft4 · Dense colophon */
  /* Same two-block colophon as the dashboard: identity in the mono register,
     the note set as the sentence it is. */
  .foot {
    display: grid;
    gap: var(--space-2xs);
    padding-block: var(--space-md) var(--space-lg);
    padding-bottom: max(var(--space-lg), env(safe-area-inset-bottom));
    border-top: var(--rule-hair) solid var(--color-rule-2);
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--color-neutral);
  }

  .foot__line { font-family: var(--font-outlier); }

  .foot__note { max-width: 68ch; }
</style>
