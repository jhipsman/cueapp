// The Cue wordmark: the mark (a C around a play button) stands in for the
// C, and "ue" follows in Sora, tight and heavy, so the logo reads "Cue".
export default function CueWordmark({ className }: { className?: string }) {
  return (
    <span className={`cue-wordmark ${className ?? ''}`} aria-label="Cue">
      <svg viewBox="0 0 32 32" aria-hidden="true" className="cue-wordmark-c">
        <defs>
          <linearGradient id="cue-wm-grad" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" style={{ stopColor: 'var(--wx-accent-hi, #5eead4)' }} />
            <stop offset="1" style={{ stopColor: 'var(--wx-accent-lo, #22b3a2)' }} />
          </linearGradient>
        </defs>
        <path d="M25.96 7.64A13 13 0 1 0 25.96 24.36L22.51 21.46A8.5 8.5 0 1 1 22.51 10.54Z" fill="url(#cue-wm-grad)" />
        <path d="M13.5 11.2L21 16L13.5 20.8Z" fill="#fff" />
      </svg>
      <span className="cue-wordmark-ue" aria-hidden="true">
        ue
      </span>
    </span>
  )
}
