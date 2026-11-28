import './legacyStorage'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './design.css'
import './setup.css'
import { demoEnabled, installDemo } from './demo'
import App from './App.tsx'
import { applyLite } from './lite'
import { ToastProvider } from './components/Toast'
import { ConfirmProvider } from './components/ConfirmProvider'

if (demoEnabled()) installDemo()

// Applied before first paint so a stored theme override (ThemeToggle.tsx)
// takes effect immediately instead of flashing the OS default first.
try {
  const stored = localStorage.getItem('cue-theme')
  if (stored === 'light') {
    document.documentElement.dataset.theme = stored
  }
} catch {
  // ignore — worst case is one flash of the OS-default theme
}

// Cue's only service worker is the one for phone alerts (push-sw.js: no
// caching, and it controls no page). One left behind by another app that
// once ran on the same address would answer API calls from its own cache and
// show stale data, so remove any other we find, and reload once if it was
// in charge of this page.
if ('serviceWorker' in navigator) {
  const ours = (url?: string) => !!url && new URL(url).pathname === '/push-sw.js'
  navigator.serviceWorker
    .getRegistrations()
    .then(async (regs) => {
      const stale = regs.filter((r) => !ours((r.active ?? r.waiting ?? r.installing)?.scriptURL))
      if (stale.length === 0) return
      await Promise.all(stale.map((r) => r.unregister()))
      const boss = navigator.serviceWorker.controller
      if (boss && !ours(boss.scriptURL)) location.reload()
    })
    .catch(() => {
      // ignore — not having permission to look is fine
    })
}

// Light mode on a slow device, before anything draws.
applyLite()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ToastProvider>
      <ConfirmProvider>
        <App />
      </ConfirmProvider>
    </ToastProvider>
  </StrictMode>,
)
