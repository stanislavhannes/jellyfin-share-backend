// Fullscreen across the browsers the share page meets: the standard API where
// there is one, the prefixed API on Safari before 16.4, and - on an iPhone,
// where no element but a <video> may go fullscreen - the video itself.

export function fullscreenElement() {
  return document.fullscreenElement || document.webkitFullscreenElement || null;
}

// Fullscreen goes to the whole page, so what the page draws over the video
// stays visible. Only on an iPhone does it fall back to the video's own,
// native fullscreen, where nothing else can show.
export function enterFullscreen(video) {
  const page = document.documentElement;
  if (page.requestFullscreen) return page.requestFullscreen().catch(() => {});
  if (page.webkitRequestFullscreen) return page.webkitRequestFullscreen();
  video?.webkitEnterFullscreen?.();
}

export function exitFullscreen() {
  if (document.exitFullscreen) return document.exitFullscreen().catch(() => {});
  document.webkitExitFullscreen?.();
}
