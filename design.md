# Design — Jellyfin Share

A locked design system for this app. Every page redesign reads this file before
emitting code. Do not regenerate per page — extend or amend this file when the
system needs to grow. Where this file and the Hallmark references disagree,
this file wins.

Produced by `hallmark redesign` (multi-page flow), genre **atmospheric**,
theme route **custom (tuned)**, vibe *"Jellyfin after dark — its blue, held quiet"*.

## Genre

**atmospheric** — dark canvas, confident type, one accent used as light
rather than as decoration. The product is a film link you hand to somebody;
the page should feel like a room with the lights down, not like a dashboard.

Consequences that follow from the genre and are not negotiable per page:

- The default canvas is dark. No white sections sneak in.
- No glassmorphism. Blur is used once, on the admin nav pill, because the pill
  genuinely floats over content.
- Elevation is **lightness**, never shadow. `paper → paper-2 → paper-3`.
- Motion is fade only. No slide, no bounce, no scroll-reveal.
- One accent hue. `--color-danger` (hue 25) is a status colour, not a second
  accent: it appears only on failures, on a revoked link's status, on the
  admin Revoke trigger's hover, and on the destructive confirms (spending the
  last play, revoking a link).

## Macrostructure families

- **Viewer pages** (`ShareView`, `ErrorView`) — **08 Photographic**.
  The artwork Jellyfin supplies fills the fold; type is annotation in the
  bottom-left corner. `ErrorView` is the degenerate case: no photograph, so the
  statement itself sits low-left against the bloom.
  *Variation knobs:* fold height, whether a logo image stands in for the
  title (inside the `h1`, so the page keeps its heading),
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
- Reduced-motion collapses everything to ≤ 150 ms; the looping loader meters
  stop and paint full instead.

## Microinteractions stance

- **Silent success.** Copying a share link swaps the button label to "Copied"
  for 1.6 s. No toast.
- **No native `confirm()` / `alert()`.** Revoking asks in the row itself
  (`Revoke it` / `Keep`) because it kills live sessions. Failures land in an
  inline banner, not a browser dialog.
- **The viewer's one confirm.** On a link with a play limit, a download asks
  in place (`Download it` / `Don’t download`) and names what it costs,
  because a download spends a play the viewer cannot get back. Focus moves
  onto `Download it`; `Don’t download` and Escape return it to the download
  button. Play itself never asks — it is what the link is for.
- **Focus comes back.** Closing the player returns focus to what was playing:
  that episode's row (after a refused next episode, the row of the one that
  didn't start), Play for a film, or, once that was the last play, the
  statement saying the link is spent (the terms' headline while a TV still
  holds the controls).
- **Refusals are sentences.** The server's error strings are for logs. The
  viewer sees what happened, whether a play was used, and what to do, in a
  `role="alert"` line. Player and Cast failures say the same: no codes (those
  go to the console), whether the play was already counted, and a next step
  ("Check it’s on and on the same Wi-Fi…", "Open the link in Chrome, Edge or
  Safari") — or, when that was the last play, that it can’t be started again
  and whom to ask.
  While something plays on a TV the page says so ("Playing on Living Room. Keep
  this page open while it plays."), since progress is saved from it. Every
  control that spends a play is held while any one of them is in flight, and
  Play says *Starting* meanwhile.
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
- **Alternative way in:** a text link, never a second pill beside Play. When
  Play resumes, a muted line with it (beside it where there is room) says
  whose place that is ("Where anyone with this link last stopped, on any
  device.") and ends in *Start from the beginning* — on a series *Restart
  Episode 2*, since it restarts that episode, not the series. Play announces
  the sentence (not the link) as its description. From 40 rem Play and the
  line share a row of their own and the other actions start the next one; on
  a phone everything stacks. When a series moves on to the next episode
  instead, the line says so ("Next after the last episode anyone with this
  link finished.") and has no link. The link is ink with a hairline
  underline, like "Change": the accent stays on Play.
- **Destructive:** `--color-danger` fill, only after an in-row confirm step.
  On the viewer page the one destructive act is spending the last play.
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

- The wordmark: the mark from the logo set, then the words "Jellyfin Share" in
  `--font-display` at `--text-md`. The mark is `BrandMark.svelte` — the small
  cut, drawn in `--color-ink`, `--color-ink-2` and `--color-accent` rather than
  loaded as a file, sized `1.1em` with `--space-sm` before the words. It is
  decorative (`aria-hidden`); the words carry the name. Its accent dot is the
  one accent mark allowed outside a filled control. Favicons, the touch icon
  and the manifest live in `web/public/brand/`.
- The footer wordmark links to the project on GitHub — the app's one outbound
  link, styled by the global `.foot-link` in `tokens.css`: inherits the footer's
  colour, hairline underline, accent on hover, never wraps, opens in a new tab
  with `rel="noopener noreferrer"`. It appears in all four footers and nowhere
  else; the nav wordmark stays unlinked so the header carries no navigation the
  page does not have.
- The accent hue and its footprint — under 5 % of any viewport.
- The three faces and the role each one tags.
- CTA voice: pill radius, `--control-h`, one-verb labels on primary actions.
- The mono register for machine facts. If it is a number a machine produced,
  it is Geist Mono with tabular figures.
- No section eyebrows, no numbered chapter tags, no tag-left/heading-right
  section heads anywhere.

## What pages MAY differ on

- Macrostructure, within the family declared above.
- Nav archetype — but not repeated across families:
  **N9 edge-aligned minimal** on viewer pages (the wordmark alone: the terms
  are stated once, in the terms block); **N5 floating pill** on the admin
  surface.
- Footer archetype, same rule:
  **Ft2 inline single line** on viewer pages ("Shared via Jellyfin Share", in
  the body face); **Ft4 dense colophon** on admin.
- Column count and label-row presence in the index, by viewport.

## The terms block

The terms come in two parts, split around the button that spends them.

Above the controls, the headline: one line at body size, weight 600, in
`--color-ink` — what is left and until when ("Last of 5 plays · expires in
2 days", "Expires in 23 hours" on a link with no play limit). Under it, in
`--text-sm` muted, the exact end ("Until 21 Oct 2026, 21:00 CEST"), then the
4px plays meter (pill ends, `--color-muted` fill on `--color-rule`), and on
the last play the sentence saying what the next tap costs ("This is the last
play. Watching here or on a TV, or downloading, uses it."), so it is read
before Play rather than after. A password link shows the headline above its
form too.

Below the controls, the fine print in `--text-sm` muted: the concurrent-viewer
limit, how a series counts plays ("When an episode runs on into the next, or
you take Play now at its end, that is still one play. Starting an episode
yourself uses one."), and how to reach a TV in a browser with no Cast button.
Where resume comes from is not fine print: it sits with Play (see CTA voice).
When a term is about to bite (the last play, every viewer place taken) it
moves up to `--color-ink`, brighter rather than red, and the meter's fill does
the same. Red stays for things that went wrong and for the one confirm that
spends the last play.

The audio and subtitle line's "Change" is ink with a hairline underline, not
accent: the accent belongs to Play.

Words, not codes: "Episode 2" (with "Season 2, episode 3" only on a link that
spans seasons), "in 6 days", file sizes in decimal units. No codec line.

The viewer-limit count is a snapshot and may include a viewer who closed the
tab without stopping; the server clears those when Play is pressed. So a full
link warns, and Play stays enabled. A link with no plays left disables every
control that spends one, because the server will refuse it.

Before the password, a locked link shows its title, artwork and terms, and
nothing else: the backend withholds the synopsis, credits, ratings, file and
tracks (`SharePublicInfo.WithholdDetails`). The form is the field with its
Show toggle, the help or error line directly under it, then Unlock. Below
40 rem a locked fold with artwork shrinks to about a third of the screen, so
the field stays above the keyboard, and Unlock takes the full width.

A spent link says so in the terms' voice (body size, weight 600), not in the
display face: it is the last of the terms, not a second title.

## Responsive floor

Every page renders clean at **320 / 375 / 414 / 768 px**. Non-negotiable:
`overflow-x: clip` on `html` and `body` (never `hidden`); no clickable text
wrapping to two lines; image-bearing grid tracks use `minmax(0, 1fr)`; display
headers carry `overflow-wrap: anywhere; min-width: 0`; hit targets ≥ 44 px
below 40 rem; `dvh` not `vh`; `env(safe-area-inset-*)` respected top and
bottom. The admin index does **not** ship a table below 60 rem — it restacks.

Breakpoints are `40rem` and `60rem`, plus `80rem` on the share page. The
share page goes `min-width` first; its queries below 40 rem
(`not all and (min-width: 40rem)`) are the locked fold and the one that lets
Play grow a second line. The admin index restacks with `max-width` queries.

Below 40 rem the share page's first screen holds the title, the terms'
headline and Play: the audio and subtitle choice is one line ("Audio: English
(Stereo) · Subtitles: None — Change") above Play, Play takes the full width,
and each secondary action takes a full-width line of its own below it, so no
label is ever cut short. When Play holds more than a line ("Resume Episode 2"
and "14:05 of 38:41"), it grows: the time drops under the label, and a label
too long for the line wraps beside the play glyph, rather than pushing the
column past the screen's edge. From 40 rem they share one row, and the poster
sits beside the action column (38 / 42 rem wide) rather than at the far edge;
without a poster the column stops at 42 rem. From 80 rem the column widens to
46 rem and the print to 18 rem, so a wide window is not left a third empty. A
title with no artwork gets a bare fold that shrinks to the nav and title, with
the single bloom and only a small gap before the terms.

Each view owns its landmarks: `main` for the page, and the footer outside it
as the page's `contentinfo`. The app shell is a plain `div`. On the share page
the wordmark's `header` sits inside `main`, because it is laid over the fold's
artwork, so it is not a `banner` there; it carries nothing but the wordmark.

An episode row carries the overview under the name, clamped to two lines. From
40 rem it also shows the episode's 16:9 still (7.5 rem) between its number and
name; a row whose still is missing or fails to load drops the column. On a spent link
the list stays as a record, with stills at 60 % and watched bars in
`--color-rule`.

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
  --color-paper:   oklch(15.5% 0.014 232);
  --color-paper-2: oklch(19% 0.015 232);
  --color-ink:     oklch(94% 0.008 232);
  --color-muted:   oklch(64% 0.011 232);
  --color-accent:  oklch(67.5% 0.137 232);
  --color-danger:  oklch(65% 0.170 25);
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
    "paper":  { "$value": "oklch(15.5% 0.014 232)", "$type": "color" },
    "ink":    { "$value": "oklch(94% 0.008 232)",   "$type": "color" },
    "accent": { "$value": "oklch(67.5% 0.137 232)", "$type": "color" },
    "danger": { "$value": "oklch(65% 0.170 25)",    "$type": "color" }
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
  --background:         oklch(15.5% 0.014 232);
  --foreground:         oklch(94% 0.008 232);
  --primary:            oklch(67.5% 0.137 232);
  --primary-foreground: oklch(16% 0.020 232);
  --muted:              oklch(19% 0.015 232);
  --muted-foreground:   oklch(64% 0.011 232);
  --destructive:        oklch(65% 0.170 25);
  --border:             oklch(30% 0.012 232);
  --input:              oklch(30% 0.012 232);
  --ring:               oklch(75% 0.160 232);
  --radius:             10px;
}
```
