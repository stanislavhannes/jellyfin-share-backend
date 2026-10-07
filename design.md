# Design — Jellyfin Share

A locked design system for this app. Every page redesign reads this file before
emitting code. Do not regenerate per page — extend or amend this file when the
system needs to grow. Where this file and the Hallmark references disagree,
this file wins.

Produced by `hallmark redesign` (multi-page flow), genre **atmospheric**,
theme route **custom (tuned)**, vibe *"Jellyfin after dark — its blue, held quiet"*.

## Genre

**atmospheric** — dark canvas, confident type, one warm accent used as light
rather than as decoration. The product is a film link you hand to somebody;
the page should feel like a room with the lights down, not like a dashboard.

Consequences that follow from the genre and are not negotiable per page:

- The default canvas is dark. No white sections sneak in.
- No glassmorphism. Blur is used once, on the admin nav pill, because the pill
  genuinely floats over content.
- Elevation is **lightness**, never shadow. `paper → paper-2 → paper-3`.
- Motion is fade only. No slide, no bounce, no scroll-reveal.
- One accent hue. `--color-danger` is an escalation of the same warm family
  (hue 28 against the anchor's 68), not a second accent.

## Macrostructure families

- **Viewer pages** (`ShareView`, `ErrorView`) — **08 Photographic**.
  The artwork Jellyfin supplies fills the fold; type is annotation in the
  bottom-left corner. `ErrorView` is the degenerate case: no photograph, so the
  statement itself sits low-left against the bloom.
  *Variation knobs:* fold height, whether a logo image replaces the title,
  whether the poster print laps the fold edge.
  *Documented deviation:* the archetype's CTA is "a typographic link tucked
  under the caption". This page has exactly one job, so the primary action is
  an accent-filled pill instead. Everything else about the archetype holds.

- **App pages** (`AdminDashboard`, `AdminLogin`) — **13 Index-First**.
  The page *is* the list. Hairline rules between rows, the title is the button,
  no table chrome, no cards, one short introducing paragraph and no hero.
  `AdminLogin` is the single-field variant of the same family: same rules, same
  left bias, one row instead of many.
  *Variation knobs:* column count above 60 rem, whether a label row is shown.

- **Player** — no macrostructure. The video is the content; chrome is a scrim
  that dissolves into the picture.

## Theme

Custom, tuned, anchored on **Jellyfin's own accent `#00a4dc`** (OKLCH hue 232) so
the share page reads as part of the same product as the server it came from.
Every neutral is tinted toward that hue, so the greys belong to the blue rather
than sitting beside it.

```
--color-paper       oklch(15.5% 0.014 232)   #060d11
--color-paper-2     oklch(19%   0.015 232)   #0d1519
--color-paper-3     oklch(23.5% 0.016 232)   #172024
--color-ink         oklch(94%   0.008 232)   #e6ecf0
--color-ink-2       oklch(78%   0.010 232)   #b1b9bd
--color-muted       oklch(64%   0.011 232)   #868d92
--color-neutral     oklch(60%   0.011 232)   #7a8186
--color-rule        oklch(30%   0.012 232)   #282f33
--color-rule-2      oklch(25%   0.011 232)   #1d2326
--color-accent      oklch(67.5% 0.137 232)   #05a4dc  ← Jellyfin blue
--color-accent-ink  oklch(16%   0.020 232)   #050f14
--color-accent-dim  oklch(55%   0.090 232)   #307a9d
--color-focus       oklch(75%   0.160 232)   #00bdff
--color-danger      oklch(65%   0.170 25)    #e45d58
--color-danger-ink  oklch(16%   0.020 25)    #150a09
```

Axes: **dark / classical-serif / cool ~232°**.

Contrast, computed from the OKLCH values against `--color-paper`: ink 16.4:1 ·
ink-2 9.8:1 · muted 5.8:1 · neutral 5.0:1 · accent 6.8:1 · danger 5.6:1 ·
focus 9.1:1. `--color-accent-ink` on `--color-accent` is 6.8:1. `--color-muted`
stays body-grade on every surface including `paper-3` (5.0:1).

**`--color-focus` is only 1.3:1 against `--color-accent`.** A ring drawn directly
on an accent fill would be invisible, so every focusable element with an accent
background uses `outline-offset: 3px`, putting the ring on paper where it reads
at 9.1:1. Do not remove those offsets.

Why this does not fall back into the generic dark-blue-with-neon-cyan look it
replaced: the accent is Jellyfin's `#00a4dc`, not a brighter invented cyan; it
appears on one filled control per view and nowhere else; the neutrals carry a
real chroma tint rather than being flat grey; and the display face is a serif.
The brand colour is borrowed — the restraint around it is the design.

## Typography

- Display: **Instrument Serif**, weight 400, style **normal**. Roman always —
  no italic headers anywhere in this app.
- Body: **Geist**, weight 350 (dark-mode optical compensation), 600 for labels
  and emphasis.
- Outlier: **Geist Mono**, 400/500. It tags exactly one role — *machine facts*:
  runtimes, resolutions, codecs, counts, tokens, timestamps, status words,
  colophons. Every instance of that role uses it; nothing else does.
- Display tracking: `-0.015em`. Body line-height 1.6, display 1.08.
- Scale: 1.25 major third off 16 px.
  `--text-display: clamp(2.25rem, 4.5vw + 1rem, 4.25rem)`.
  **Titles over 32 characters step down to `--text-display-s`** — media titles
  are user data and can be long. `ShareView` does this with `titleIsLong`.
- `font-variant-numeric: tabular-nums` on every column of figures.

## Spacing

4-point named scale, in `web/src/tokens.css`. Pages use named tokens
(`var(--space-md)`), never raw values. `--page-gutter` is
`clamp(1.125rem, 4vw, 3.5rem)` and is the only horizontal page padding.
`--control-h: 2.75rem` is the single height shared by every input and every
button adjacent to one.

## Motion

- Easings: `--ease-out` `cubic-bezier(0.16, 1, 0.3, 1)`,
  `--ease-in` `cubic-bezier(0.7, 0, 0.84, 0)`,
  `--ease-in-out` `cubic-bezier(0.65, 0, 0.35, 1)`. Never the browser default.
- Durations: `--dur-micro` 120 ms · `--dur-short` 220 ms · `--dur-long` 420 ms.
- **Three primitives on the whole app, and no more:**
  1. `content-fade` — the poster fades in when it decodes (`--dur-long`).
  2. `row-hover` — `translateY(-1px)` plus a title colour shift on index rows,
     inside `@media (hover: hover)` only.
  3. `meter-sweep` — the loading line and the play-budget fill.
- No scroll-triggered reveals. No parallax. No stagger.
- Reduced-motion collapses everything to ≤ 150 ms; the functional meters keep
  running, just slower.

## Microinteractions stance

- **Silent success.** Copying a share link swaps the button label to "Copied"
  for 1.6 s. No toast.
- **No native `confirm()` / `alert()`.** Revoking asks in the row itself
  (`Revoke it` / `Keep`) because it kills live sessions. Failures land in an
  inline banner, not a browser dialog.
- Focus rings appear **instantly**, never transitioned. On accent fills the
  ring uses `outline-offset: 3px` so it sits on paper, where it has contrast.
- Inputs validate on **blur**, not per keystroke. Helper text reserves
  `min-height: 1lh` so an error never pushes the page down.
- Border width is 1 px in every input state. State goes to background and
  outline, never to geometry.
- Hover effects live inside `@media (hover: hover)`. Every hover affordance has
  a tap and a focus equivalent.

## CTA voice

- **Primary:** accent fill, `--color-accent-ink` text, `--radius-pill`,
  `--control-h` (3.25 rem for the single Play button, which is the app's one
  primary action). Label is one verb: *Play · Unlock · Sign in*.
- **Secondary:** ghost — no fill, 1 px `--color-rule` border, pill, ink-2 text.
  *Cast to a TV · Copy link · Stop*.
- **Destructive:** `--color-danger` fill, only after an in-row confirm step.
- Labels never wrap. `white-space: nowrap` on every affordance.

## Icons

One stroke voice across the whole app: **Lucide geometry, `stroke-width: 1.75`,
round caps and joins**, sized 1–1.15 rem and `flex-shrink: 0` so a label never
squeezes a glyph. No emoji anywhere, no second icon library.

Two symbols are filled rather than stroked — the play triangle and the stop
square. Those are transport marks, not icons; an outlined play triangle reads as
a decoration instead of a control.

**The Cast glyph is not decoration and must not be dropped.** It is what tells a
viewer that the button means Google Cast rather than some other second screen,
so it ships on every cast control: the play row, the episode cast bar, and — while
a receiver is connected — the trailing cell of every episode row, in accent, to
say that clicking casts rather than plays locally. It lives in
`web/src/components/CastIcon.svelte` so all three sites stay identical. When no
receiver is connected the episode rows carry no glyph at all; the absence is the
signal that playback is local.

## Per-page allowances

- Viewer pages carry **no** enrichment — Jellyfin's backdrop and poster are the
  real imagery and any added artwork would compete with them.
- Chrome-only pages (`ErrorView`, `AdminLogin`, `AdminDashboard`) carry exactly
  **one** radial bloom via `.bloom-ground`: fixed, unanimated, ~26 % footprint.
  Never two, never animated.
- App pages MUST NOT use enrichment beyond that bloom.

## What pages MUST share

- The wordmark: the words "Jellyfin Share" in `--font-display` at `--text-md`.
- The footer wordmark links to the project on GitHub — the app's one outbound
  link, styled by the global `.foot-link` in `tokens.css`: inherits the footer's
  colour, hairline underline, accent on hover, never wraps, opens in a new tab
  with `rel="noopener noreferrer"`. It appears in all four footers and nowhere
  else; the nav wordmark stays unlinked so the header carries no navigation the
  page does not have.
- The accent hue and its footprint — under 5 % of any viewport.
- The three faces and the role each one tags.
- CTA voice: pill radius, `--control-h`, one-verb labels.
- The mono register for machine facts. If it is a number a machine produced,
  it is Geist Mono with tabular figures.
- No section eyebrows, no numbered chapter tags, no tag-left/heading-right
  section heads anywhere.

## What pages MAY differ on

- Macrostructure, within the family declared above.
- Nav archetype — but not repeated across families:
  **N9 edge-aligned minimal** on viewer pages (wordmark left, expiry right,
  nothing between); **N5 floating pill** on the admin surface.
- Footer archetype, same rule:
  **Ft2 inline single line** on viewer pages; **Ft4 dense colophon** on admin.
- Column count and label-row presence in the index, by viewport.

## Responsive floor

Every page renders clean at **320 / 375 / 414 / 768 px**. Non-negotiable:
`overflow-x: clip` on `html` and `body` (never `hidden`); no clickable text
wrapping to two lines; image-bearing grid tracks use `minmax(0, 1fr)`; display
headers carry `overflow-wrap: anywhere; min-width: 0`; hit targets ≥ 44 px
below 40 rem; `dvh` not `vh`; `env(safe-area-inset-*)` respected top and
bottom. The admin index does **not** ship a table below 60 rem — it restacks.

Breakpoints are `40rem` and `60rem`, in that direction (`min-width` first).

## Verified

Rendered headless (Chrome) with fixture data at 320 / 375 / 414 / 768 / 1280 px
across all five views. Measured, not eyeballed: `documentElement.scrollWidth`
equals the viewport at every width and no element's bounding box exceeds it —
gate 34 passes by measurement. Contrast ratios in the Theme section above are
computed from the OKLCH values, not estimated.

Four defects were found by rendering and fixed:

1. The stage's negative margin pulled the whole text column under the fold's
   absolutely-positioned scrim, which swallowed the overview line. Only the
   poster print laps the fold now, and the stage carries its own stacking
   context.
2. On phones the poster print sat right-aligned in an otherwise empty row. It
   is hidden below 40 rem — the fold already shows the artwork, falling back to
   the poster when a share has no backdrop.
3. The admin label row's trailing `auto` track resolved to zero width while the
   data rows' resolved to the width of the buttons, so every heading sat ~160 px
   right of its column. Both now use a fixed final track.
4. Svelte trims the space before a `{#if}`, which ran two footer fragments
   together ("Share· 3 of 5"). The separators are explicit strings now.

5. Checked against the live backend, a bright backdrop (SMPTE bars on the test
   library) left the wordmark and expiry unreadable — the top scrim was too
   soft. It now holds full strength past the nav row before fading, so the fold
   is legible over any frame Jellyfin returns, not only a dark one.

A poster that fails to load drops its figure rather than painting alt text
across the layout.

## Known deviation

**Gate 38 (outlier face in at most two slots) is deliberately not met.** The
2+1 rule caps the outlier at two typographic moments — wordmark plus hero stat.
This app is not a marketing page: runtimes, resolutions, codecs, play counts,
expiry windows, tokens, timestamps and session agents are machine facts that
appear on every surface, and setting them in the body face costs the tabular
alignment that makes them readable as data. Geist Mono therefore tags one
consistent *role* rather than two slots. The rule it actually honours is the one
behind the gate: the outlier means something specific, and every instance of
that meaning uses it. No other gate is open.

## Exports

### tokens.css

The canonical file is [`web/src/tokens.css`](web/src/tokens.css) — it is linked
from `web/index.html` and is the only place a colour, face, step, easing or
z-level is defined. Nothing downstream may inline a value.

### Tailwind v4 `@theme`

```css
@theme {
  --color-paper:   oklch(14% 0.012 68);
  --color-paper-2: oklch(17.5% 0.013 68);
  --color-ink:     oklch(94% 0.010 78);
  --color-muted:   oklch(64% 0.013 72);
  --color-accent:  oklch(78% 0.145 68);
  --color-danger:  oklch(65% 0.170 28);
  --font-display:  "Instrument Serif", ui-serif, Georgia, serif;
  --font-body:     "Geist", ui-sans-serif, system-ui, sans-serif;
  --font-mono:     "Geist Mono", ui-monospace, monospace;
  --spacing-md:    1rem;
  --spacing-lg:    1.5rem;
  --text-md:       1.25rem;
  --ease-out:      cubic-bezier(0.16, 1, 0.3, 1);
}
```

### DTCG `tokens.json`

```json
{
  "color": {
    "paper":  { "$value": "oklch(14% 0.012 68)",  "$type": "color" },
    "ink":    { "$value": "oklch(94% 0.010 78)",  "$type": "color" },
    "accent": { "$value": "oklch(78% 0.145 68)",  "$type": "color" },
    "danger": { "$value": "oklch(65% 0.170 28)",  "$type": "color" }
  },
  "font": {
    "display": { "$value": "Instrument Serif", "$type": "fontFamily" },
    "body":    { "$value": "Geist",            "$type": "fontFamily" },
    "mono":    { "$value": "Geist Mono",       "$type": "fontFamily" }
  },
  "space": {
    "md": { "$value": "1rem",   "$type": "dimension" },
    "lg": { "$value": "1.5rem", "$type": "dimension" }
  }
}
```

### shadcn/ui CSS variables

```css
:root {
  --background:         oklch(14% 0.012 68);
  --foreground:         oklch(94% 0.010 78);
  --primary:            oklch(78% 0.145 68);
  --primary-foreground: oklch(19% 0.030 62);
  --muted:              oklch(22% 0.014 68);
  --muted-foreground:   oklch(64% 0.013 72);
  --destructive:        oklch(65% 0.170 28);
  --border:             oklch(30% 0.012 68);
  --input:              oklch(30% 0.012 68);
  --ring:               oklch(83% 0.170 78);
  --radius:             10px;
}
```
