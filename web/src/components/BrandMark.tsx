// The Cue mark (a C around a play button), inlined so its
// fill="currentColor" picks up whatever color the caller sets — paired
// with .brand-mark { color: var(--accent) } by default. Inlining (rather
// than an <img src>) is what the brand kit's own README recommends, since
// an <img> can't inherit currentColor.
export default function BrandMark({ className }: { className?: string }) {
  return (
    <svg className={className ?? 'brand-mark'} viewBox="0 0 32 32" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M25.96 7.64A13 13 0 1 0 25.96 24.36L22.51 21.46A8.5 8.5 0 1 1 22.51 10.54Z" fill="currentColor" />
      <path d="M13.5 11.2L21 16L13.5 20.8Z" fill="currentColor" />
    </svg>
  )
}
