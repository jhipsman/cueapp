// The Cue mark (an even ring round a soft play button), inlined so its
// fill="currentColor" picks up whatever color the caller sets — paired
// with .brand-mark { color: var(--accent) } by default. Inlining (rather
// than an <img src>) lets it inherit currentColor.
export default function BrandMark({ className }: { className?: string }) {
  return (
    <svg className={className ?? 'brand-mark'} viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M206.77,198.93 A106,106 0 1 1 206.77,57.07 A21.0,21.0 0 0 1 175.56,85.18 A64,64 0 1 0 175.56,170.82 A21.0,21.0 0 0 1 206.77,198.93 Z M115.76,106.20 Q115.76,95.00 125.60,100.35 L166.64,122.65 Q176.48,128.00 166.64,133.35 L125.60,155.65 Q115.76,161.00 115.76,149.80 Z" fill="currentColor" />
    </svg>
  )
}
