// Runs as soon as the loading screen in index.html is drawn, before the
// app's code arrives. A file rather than an inline <script>, because Cue's
// Content Security Policy allows only scripts from its own files.

// When the loading screen showed: components/Splash.tsx carries the logo's
// animation on from here, so it plays once per page load.
window.__cueSplashAt = performance.now()

// Cue is dark unless light was chosen (main.tsx does the same, later).
try {
  if (localStorage.getItem('cue-theme') === 'light') document.documentElement.dataset.theme = 'light'
} catch (e) {
  // storage blocked: stay dark
}
