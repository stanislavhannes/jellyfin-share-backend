<script>
  // Marks something the viewer has watched. With a title it names that for
  // assistive tech; without one it is decoration beside a visible label.
  export let title = '';
</script>

<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"
     stroke-linecap="round" stroke-linejoin="round"
     role={title ? 'img' : 'presentation'} aria-hidden={title ? undefined : 'true'}
     aria-label={title || undefined}>
  <path d="M20 6 9 17l-5-5" />
</svg>
