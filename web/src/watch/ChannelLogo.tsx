import { useState } from 'react'

// A channel's logo, or, when it has none or it won't load, a badge with the
// channel's initials in a colour of its own (the same colour every time).
// Provider names like "US: ESPN 2 FHD" become "E2" (ESPN 2).
export function channelInitials(name: string): string {
  const clean = name
    .replace(/^[A-Z]{2,3}\s*[:|-]\s*/i, '') // "US: ", "UK | "
    .replace(/\b(FHD|UHD|HD|SD|4K|HEVC|H265|RAW|VIP|\d{3,4}P)\b/gi, '')
    .replace(/[^\p{L}\p{N}\s&+]/gu, ' ')
    .trim()
  const words = clean.split(/\s+/).filter(Boolean)
  if (words.length === 0) return name.slice(0, 2).toUpperCase()
  if (words.length === 1) return words[0].slice(0, 3).toUpperCase()
  return words
    .slice(0, 3)
    .map((w) => (/^\d+$/.test(w) ? w : w[0]))
    .join('')
    .slice(0, 3)
    .toUpperCase()
}

const HUES = [174, 205, 262, 330, 14, 38, 140, 190]

export default function ChannelLogo({ name, src, className }: { name: string; src?: string; className?: string }) {
  const [failed, setFailed] = useState(false)
  if (src && !failed) {
    return <img className={className} src={src} alt="" loading="lazy" onError={() => setFailed(true)} />
  }
  let h = 0
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  const hue = HUES[h % HUES.length]
  return (
    <span className={`wx-chan-initials ${className ?? ''}`} style={{ background: `linear-gradient(135deg, hsl(${hue} 55% 38%), hsl(${hue} 60% 24%))` }} aria-hidden="true">
      {channelInitials(name)}
    </span>
  )
}
