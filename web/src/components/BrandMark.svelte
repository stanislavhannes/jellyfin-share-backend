<!-- The mark from the logo set, in its heavier small-size cut (the nav draws it
     at about 22px). Drawn here rather than loaded as a file so its three
     colours are the palette's tokens and cannot drift from tokens.css.
     Decorative: the words beside it carry the name. -->
<svg class="mark" viewBox="0 0 120 120" aria-hidden="true" focusable="false">
  <path class="mark__line" d="M34 26 L94 60 L34 94 Z" />
  <circle class="mark__node" cx="34" cy="26" r="15" />
  <circle class="mark__node" cx="34" cy="94" r="15" />
  <circle class="mark__apex" cx="94" cy="60" r="16" />
</svg>

<style>
  /* About 1.1x the wordmark's size, whatever size the wordmark is set at. */
  .mark { width: 1.1em; height: 1.1em; flex-shrink: 0; }
  .mark__line { fill: none; stroke: var(--color-ink-2); stroke-width: 11; stroke-linejoin: round; }
  .mark__node { fill: var(--color-ink); }
  .mark__apex { fill: var(--color-accent); }
</style>
