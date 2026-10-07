<script>
  // The Google Cast mark is what makes the control recognisable — a text-only
  // button leaves the viewer guessing. Drawn in the same stroke voice as the
  // rest of the app's icons (Lucide geometry, 1.75) so it belongs to the page
  // while staying the glyph people already know.
  export let title = '';
</script>

<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75"
     stroke-linecap="round" stroke-linejoin="round"
     role={title ? 'img' : 'presentation'} aria-hidden={title ? undefined : 'true'}
     aria-label={title || undefined}>
  <path d="M2 8V6a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-6" />
  <path d="M2 12a9 9 0 0 1 8 8" />
  <path d="M2 16a5 5 0 0 1 4 4" />
  <path d="M2 20h.01" />
</svg>
