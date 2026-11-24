import type { CSSProperties } from 'react'
import BrandMark from './BrandMark'

declare global {
  interface Window {
    __cueSplashAt?: number
  }
}

// When the loading screen first showed (index.html notes it), so the
// animation carries on from there instead of starting again: the logo pops
// in once per page load, however many times the loading screen is drawn.
function since(): number {
  if (typeof performance === 'undefined') return 0
  if (window.__cueSplashAt === undefined) window.__cueSplashAt = performance.now()
  return Math.max(0, Math.round(performance.now() - window.__cueSplashAt))
}

// Full-screen loading screen, as the TV app opens: the Cue mark grows and
// turns into place with a soft glow breathing behind it, and "cue" slides in
// beside it. The same markup and styles are inlined in index.html so it shows
// before the app's code has even arrived; keep the two in step.
export default function Splash({ label = 'Loading', hint, onRetry }: { label?: string; hint?: string; onRetry?: () => void }) {
  const quiet = label === 'Loading' && !hint && !onRetry
  return (
    <div className="splash" role="status" aria-live="polite" style={{ '--splash-t': `-${since()}ms` } as CSSProperties}>
      <div className="splash-lockup">
        <span className="splash-glow" aria-hidden="true" />
        <BrandMark className="splash-logo" />
        <span className="splash-word" aria-hidden="true">
          cue
        </span>
      </div>
      <span className={quiet ? 'splash-label quiet' : 'splash-label'}>{quiet ? 'Loading Cue' : label}</span>
      {hint && <p className="splash-hint">{hint}</p>}
      {onRetry && (
        <button type="button" className="primary" onClick={onRetry}>
          Try again
        </button>
      )}
    </div>
  )
}
