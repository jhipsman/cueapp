import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useDarkPage } from '../useDarkPage'
import { titleFor } from '../documentTitle'
import './landing.css'

// The front page for someone not signed in: a short headline, the app
// playing, and how to get Cue on each screen. Signing in is one button
// away; once signed in, Cue opens straight to Watch and this page isn't
// seen again.

const M = '/landing/'

// Downloader's short code for cuetv.me/tv.apk (made at go.aftvnews.com).
// Only on cuetv.me: another Cue gives its own address instead.
const DOWNLOADER_CODE = location.hostname.endsWith('cuetv.me') ? '3575368' : ''

type Device = 'tv' | 'web' | 'apple'

function Mark() {
  return (
    <svg viewBox="0 0 256 256" aria-hidden="true">
      <path fill="#34D1BF" d="M206.77,198.93 A106,106 0 1 1 206.77,57.07 A21.0,21.0 0 0 1 175.56,85.18 A64,64 0 1 0 175.56,170.82 A21.0,21.0 0 0 1 206.77,198.93 Z M115.76,106.20 Q115.76,95.00 125.60,100.35 L166.64,122.65 Q176.48,128.00 166.64,133.35 L125.60,155.65 Q115.76,161.00 115.76,149.80 Z" />
    </svg>
  )
}

const PlayIcon = () => (
  <svg viewBox="0 0 24 24" aria-hidden="true">
    <path d="M8 5v14l11-7z" />
  </svg>
)

// The app's Home screen, playing (muted, looping) while it's on screen.
// "Watch the ad" swaps it for the commercial, with sound and controls.
function Screen() {
  const [ad, setAd] = useState(false)
  const v = useRef<HTMLVideoElement>(null)
  useEffect(() => {
    const el = v.current
    if (!el) return
    if (ad) {
      el.play().catch(() => undefined)
      return
    }
    if (matchMedia('(prefers-reduced-motion: reduce)').matches || !('IntersectionObserver' in window)) return
    const io = new IntersectionObserver(
      ([e]) => {
        if (e.isIntersecting) el.play().catch(() => undefined)
        else el.pause()
      },
      { threshold: 0.25 },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [ad])
  return (
    <figure className="lp-screen">
      {ad ? (
        <video key="ad" ref={v} src={`${M}commercial.mp4`} poster={`${M}commercial.jpg`} controls playsInline preload="auto" aria-label="The Cue commercial" />
      ) : (
        <>
          <video key="app" ref={v} src={`${M}hero.mp4`} poster={`${M}hero.jpg`} muted loop playsInline preload="metadata" aria-label="Cue's Home screen" />
          <button type="button" className="lp-ad" onClick={() => setAd(true)}>
            <PlayIcon />
            Watch the ad
          </button>
        </>
      )}
    </figure>
  )
}

function Copy({ text }: { text: string }) {
  const [done, setDone] = useState(false)
  const code = useRef<HTMLElement>(null)
  return (
    <div className="lp-copy">
      <code ref={code}>{text}</code>
      <button
        type="button"
        onClick={() => {
          const pick = () => {
            const r = document.createRange()
            if (code.current) r.selectNodeContents(code.current)
            const s = getSelection()
            s?.removeAllRanges()
            s?.addRange(r)
          }
          try {
            navigator.clipboard.writeText(text).then(() => {
              setDone(true)
              setTimeout(() => setDone(false), 1600)
            }, pick)
          } catch {
            pick()
          }
        }}
      >
        {done ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}

function Step({ n, title, children }: { n: number; title: string; children?: ReactNode }) {
  return (
    <li className="lp-step">
      <span className="n" aria-hidden="true">
        {n}
      </span>
      <div>
        <h3>{title}</h3>
        {children}
      </div>
    </li>
  )
}

export default function Landing() {
  useDarkPage(true)
  useEffect(() => {
    document.title = titleFor('Welcome')
  }, [])
  const host = window.location.host
  const [tab, setTab] = useState<Device>('tv')
  const tabs: { id: Device; name: string }[] = [
    { id: 'tv', name: 'Fire TV · Google TV' },
    { id: 'web', name: 'Phone · laptop' },
    { id: 'apple', name: 'Apple TV' },
  ]

  return (
    <div className="lp">
      <nav className="lp-nav lp-wrap" aria-label="Page">
        <a className="lp-brand" href="#top" aria-label="Cue, back to the top">
          <Mark />
          Cue
        </a>
        <a className="lp-link" href="#get">
          Download
        </a>
        <Link className="lp-btn primary small" to="/login">
          Sign in
        </Link>
      </nav>

      <header className="lp-hero lp-wrap" id="top">
        <h1>
          Every movie. Every show.
          <br />
          <span>One app.</span>
        </h1>
        <div className="lp-ctas">
          <a className="lp-btn primary" href="#get">
            Get Cue
          </a>
          <Link className="lp-btn ghost" to="/login">
            Sign in
          </Link>
        </div>
        <Screen />
      </header>

      <section className="lp-get lp-wrap" id="get" aria-labelledby="lp-get-h">
        <h2 id="lp-get-h">Get Cue</h2>
        <div className="lp-tabs" role="tablist" aria-label="Your device">
          {tabs.map((t, i) => (
            <button
              key={t.id}
              role="tab"
              id={`lp-tab-${t.id}`}
              aria-selected={tab === t.id}
              aria-controls={`lp-p-${t.id}`}
              tabIndex={tab === t.id ? 0 : -1}
              onClick={() => setTab(t.id)}
              onKeyDown={(e) => {
                const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
                if (!d) return
                const n = tabs[(i + d + tabs.length) % tabs.length]
                setTab(n.id)
                document.getElementById(`lp-tab-${n.id}`)?.focus()
              }}
            >
              {t.name}
            </button>
          ))}
        </div>

        {tab === 'tv' && (
          <ol className="lp-panel" role="tabpanel" id="lp-p-tv" aria-labelledby="lp-tab-tv">
            <Step n={1} title="Get Downloader">
              <p>From the Amazon Appstore or Play Store.</p>
            </Step>
            <Step n={2} title="Allow installs">
              <p>Fire TV: Settings &gt; My Fire TV &gt; Developer options &gt; Install unknown apps &gt; Downloader.</p>
            </Step>
            {DOWNLOADER_CODE ? (
              <Step n={3} title="Type this code in Downloader">
                <p className="lp-code">{DOWNLOADER_CODE}</p>
                <p>
                  Or the address <b>{host}/tv.apk</b>
                </p>
                <p>
                  <a href="/setup-tv">Step by step, with pictures</a>
                </p>
              </Step>
            ) : (
              <Step n={3} title="Enter this in Downloader">
                <Copy text={`${host}/tv.apk`} />
              </Step>
            )}
          </ol>
        )}
        {tab === 'web' && (
          <ol className="lp-panel" role="tabpanel" id="lp-p-web" aria-labelledby="lp-tab-web">
            <Step n={1} title="Open Cue">
              <Copy text={host} />
            </Step>
            <Step n={2} title="iPhone or iPad">
              <p>
                Share &gt; <b>Add to Home Screen</b>
              </p>
            </Step>
            <Step n={3} title="Android">
              <p>
                Chrome &gt; <b>Install app</b>
              </p>
            </Step>
          </ol>
        )}
        {tab === 'apple' && (
          <ol className="lp-panel" role="tabpanel" id="lp-p-apple" aria-labelledby="lp-tab-apple">
            <Step n={1} title="Ask whoever runs Cue" />
            <Step n={2} title="Tell them which Apple TV" />
            <Step n={3} title="Open Cue" />
          </ol>
        )}
      </section>

      <footer className="lp-foot lp-wrap">
        <span className="lp-brand small">
          <Mark />
          Cue
        </span>
        <span>Posters and artwork from TMDB.</span>
      </footer>
    </div>
  )
}
