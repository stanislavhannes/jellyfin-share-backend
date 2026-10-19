# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **Sharer.** Someone who self-hosts Jellyfin and wants to hand a film, episode,
  season or series to a person outside their server. They usually create the link
  from the Jellyfin UI with the companion plugin
  ([jellyfin-share-plugin](https://github.com/stanislavhannes/jellyfin-share-plugin))
  and use `/admin` to see, check and revoke the links they have handed out. They
  sign in to `/admin` with the backend API key, not a Jellyfin account.
- **Viewer.** A friend or family member with no Jellyfin account, often not
  technical, who opens the link on whatever device they have: a phone, a laptop,
  sometimes a browser they then cast to a TV from. Their job is to start watching,
  with the right audio and subtitles, and pick up where they left off.
- **Installer.** This is a public open-source fork, so other self-hosters read the
  README, set up `.env` and run it with Docker Compose. Setup docs and defaults are
  part of the product.

## Product Purpose

It gives people temporary, limited access to specific Jellyfin media without giving
them access to the Jellyfin server. It works when a viewer opens a link and it plays
on the first try, the terms of the link are never a surprise, and the sharer can
see at a glance what is live and stop it.

## Positioning

These are in priority order. When two of them conflict, the higher one wins.

1. **Honest, plain terms.** A link's expiry, plays left, concurrent-viewer limit
   and password are stated in plain words, never hidden or dressed up as badges.
   A viewer always knows what they have and how long it lasts.
2. **Feels like Jellyfin.** The share page uses the artwork Jellyfin already has
   and Jellyfin's own accent, so it reads as part of the server it came from, not
   as a generic file drop.
3. **Just works for viewers.** No account. The codec is negotiated with the
   viewer's browser, tracks can be chosen before playback, playback resumes,
   the next episode is offered, and the viewer can cast. Playback should succeed
   without the viewer having to understand any of it.
4. **Security and isolation.** Jellyfin is never exposed. Credentials stay on the
   server, and the item, media source, codec, bitrate and tracks are pinned to the
   session, so a link cannot be pointed at anything else in the library.

## Operating Context

- Shares are created in Jellyfin through the plugin or through the admin API
  (`POST /api/admin/shares` with `X-Backend-Key`). The web `/admin` surface lists,
  inspects and revokes them.
- Viewers arrive from a link sent over chat or email, often on a phone, and may
  hand off to a TV: Google Cast works in Chromium browsers only, AirPlay through
  Safari's own player control, and Firefox has no casting route.
- Deployed as a Go backend serving the built Svelte app, with PostgreSQL, through
  Docker Compose, usually behind the sharer's own domain (`PUBLIC_BASE_URL`).
- Works with Jellyfin 10.11 and 12.x.

## Capabilities and Constraints

- Time-limited links, an optional password, total-play and concurrent-viewer
  limits, and a quality cap per share (1080p / 720p / 480p).
- Downloads of the original file, or of a season or series as a ZIP filed by
  season. Each download counts as one play, and downloads can be turned off per
  share or globally (`JFSHARE_ALLOW_DOWNLOADS`).
- HLS proxied through the backend. Text subtitles are sent as a WebVTT sidecar.
  Resume position and watched state are kept on the server per link, so a
  viewer resumes on any device; everyone holding the same link shares them.
- Session tracking. Revoking a link ends live sessions.
- Stack: Go backend, Svelte 4 + Vite frontend in `web/`, hls.js. Brand tokens live
  in `web/src/tokens.css`.
- UI copy is English only. No translation layer is planned.
- Terms: a *share* / *link* is one handed-out grant. *Plays* are counted starts
  and downloads. *Watching now* means live sessions.

## Brand Commitments

- The name is **Jellyfin Share**. The product is related to Jellyfin and anchored
  on Jellyfin's accent colour, but it is not an official Jellyfin product.
- Voice: plain, specific and calm. Say the terms in words ("Nobody has opened this
  link yet.", "Set by whoever shared this"). No marketing tone, no exclamation
  marks, no emoji.
- The footer wordmark links to the project on GitHub. This is the app's one
  outbound link.
- The visual system is recorded separately in `design.md` and is not part of this
  record.

## Evidence on Hand

- Screenshots: `docs/screenshots/share-movie.png`, `share-series.png`,
  `admin-dashboard.png`.
- The README's "What this fork changes" section has measured security and playback
  fixes, each reproduced against a live Jellyfin.
- There are no user counts, testimonials, press or benchmarks beyond those notes.
  Do not invent any.

## Product Principles

1. State the terms before they bite. Expiry, limits and play counts are always
   visible where they matter, in plain words.
2. The viewer owes nothing. No account, no setup, no codec knowledge. If something
   can't work on their device, say so plainly instead of failing silently.
3. Borrow Jellyfin, don't compete with it. The media and the server's own artwork
   lead, and the share page frames them.
4. The sharer stays in control. Every live link can be seen, understood at a
   glance and revoked.
5. Never trade isolation for convenience. A feature that would expose Jellyfin or
   let a link reach beyond its item does not ship.

## Accessibility & Inclusion

No formal standard is required. Good practice applies: keyboard and focus support,
readable contrast, touch targets usable on phones, and reduced-motion respected.
Viewers are often non-technical and on a phone.
