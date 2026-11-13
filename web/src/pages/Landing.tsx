import { lazy, Suspense, useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useDarkPage } from '../useDarkPage'
import { titleFor } from '../documentTitle'
import './landing.css'

// The front page for someone not signed in: what Cue is, the real app
// playing (clips recorded from it), the commercial, and how to get it on
// each screen. Signing in is one button away; once signed in, Cue opens
// straight to Watch and this page isn't seen again.

const POSTERS = [
  1368337, 1792170, 1375441, 1294189, 969681, 1003596, 1283515, 791523, 1570488, 1384216, 1300968, 1170608, 1117898, 1567605, 1465063,
  1290418, 1084244, 1153576, 1380417, 1377237, 247718, 225385, 213375, 258989, 120911, 153312, 314271, 81723, 84773, 248830,
]
const M = '/landing/'

// The 3D phone (three.js), fetched only for this page and only after it shows.
const PhoneModel = lazy(() => import('./PhoneModel'))

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

// A clip of the app, playing (muted, looping) while it's on screen.
function Loop({ name, label, className }: { name: string; label: string; className?: string }) {
  const v = useRef<HTMLVideoElement>(null)
  useEffect(() => {
    const el = v.current
    if (!el || matchMedia('(prefers-reduced-motion: reduce)').matches || !('IntersectionObserver' in window)) return
    const io = new IntersectionObserver(
      ([e]) => {
        if (e.isIntersecting) el.play().catch(() => undefined)
        else el.pause()
      },
      { threshold: 0.25 },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [])
  return <video ref={v} className={className} src={`${M}${name}.mp4`} poster={`${M}${name}.jpg`} muted loop playsInline preload="metadata" aria-label={label} />
}

// Phone draws a modern phone around a clip: a brushed titanium edge with
// its buttons, a thin black border, the pill-shaped camera cutout and a
// status bar, so the recording sits on a real-looking screen.
function Phone({ children }: { children: ReactNode }) {
  return (
    <div className="lp-device">
      <i className="lp-key action" />
      <i className="lp-key up" />
      <i className="lp-key down" />
      <i className="lp-key side" />
      <div className="lp-device-bezel">
        <div className="lp-device-screen">
          <div className="lp-status" aria-hidden="true">
            <span>9:41</span>
            <span className="lp-island" />
            <span className="lp-status-r">
              <b className="lp-bars" />
              <b className="lp-batt" />
            </span>
          </div>
          {children}
        </div>
      </div>
    </div>
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

export default function Landing() {
  useDarkPage(true)
  useEffect(() => {
    document.title = titleFor('Welcome')
  }, [])
  const host = window.location.host
  const [tab, setTab] = useState<'tv' | 'web' | 'apple'>('tv')
  const [adOn, setAdOn] = useState(false)
  const ad = useRef<HTMLVideoElement>(null)
  const cinema = useRef<HTMLDivElement>(null)
  const hours = useRef<HTMLSpanElement>(null)
  const still = typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches

  useEffect(() => {
    if (adOn) ad.current?.play().catch(() => undefined)
  }, [adOn])

  // The year counts up once it's seen.
  useEffect(() => {
    const el = hours.current
    if (!el || still || !('IntersectionObserver' in window)) return
    const once = new IntersectionObserver(
      ([e]) => {
        if (!e.isIntersecting) return
        once.disconnect()
        const t0 = performance.now()
        const tick = (now: number) => {
          const k = Math.min(1, (now - t0) / 1400)
          el.textContent = (212.5 * (1 - Math.pow(1 - k, 4))).toFixed(1)
          if (k < 1) requestAnimationFrame(tick)
        }
        requestAnimationFrame(tick)
      },
      { threshold: 0.6 },
    )
    once.observe(el)
    return () => once.disconnect()
  }, [still])

  const watchAd = () => {
    cinema.current?.scrollIntoView({ behavior: still ? 'auto' : 'smooth', block: 'center' })
    setAdOn(true)
  }
  const tabs: { id: 'tv' | 'web' | 'apple'; name: string }[] = [
    { id: 'tv', name: 'Fire TV · Google TV' },
    { id: 'web', name: 'Phone · laptop' },
    { id: 'apple', name: 'Apple TV' },
  ]

  return (
    <div className="lp">
      <nav className="lp-nav" aria-label="Page">
        <div className="lp-nav-in">
          <a className="lp-brand" href="#top" aria-label="Cue, back to the top">
            <Mark />
            Cue
          </a>
          <a className="lp-link" href="#inside">
            Inside
          </a>
          <a className="lp-link" href="#ad">
            The ad
          </a>
          <a className="lp-link" href="#get">
            Get it
          </a>
          <Link className="lp-btn primary" to="/login">
            Sign in
          </Link>
        </div>
      </nav>

      <header className="lp-hero" id="top">
        <div className="lp-wrap lp-hero-grid">
          <div>
            <h1>
              Every movie.
              <br />
              Every show.
              <br />
              <span className="lp-sheen">One app.</span>
            </h1>
            <div className="lp-ctas">
              <Link className="lp-btn primary" to="/login">
                Sign in
              </Link>
              <button type="button" className="lp-btn ghost" onClick={watchAd}>
                <PlayIcon />
                Watch the ad
              </button>
            </div>
          </div>
          <div className="lp-stage3d">
            <div className="lp-screen">
              <Loop name="hero" label="Cue's Home screen, its banner playing a trailer while the pointer moves across Continue Watching" />
            </div>
            <div className="lp-phone">
              <Phone>
                <Loop name="phone" label="Cue on a phone, swiping through rows of shows" />
              </Phone>
              <Suspense fallback={null}>
                <PhoneModel model={`${M}iphone.glb`} clip={`${M}phone.mp4`} poster={`${M}phone.jpg`} />
              </Suspense>
            </div>
          </div>
        </div>
      </header>

      <div className="lp-marquee" aria-hidden="true">
        {[POSTERS.slice(0, 15), POSTERS.slice(15)].map((row, i) => (
          <div className="lp-mrow" key={i}>
            {row.concat(row).map((n, k) => (
              <img key={k} src={`${M}p${n}.jpg`} alt="" loading="lazy" />
            ))}
          </div>
        ))}
      </div>

      <section className="lp-part" id="inside">
        <div className="lp-wrap">
          <div className="lp-head">
            <h2>What's in it</h2>
          </div>
          <div className="lp-bento">
            <div className="lp-tile t-title">
              <Loop name="title" className="bg" label="A movie's page in Cue, its trailer playing behind the title" />
              <span className="lp-tag">
                <b>●</b> Everything
              </span>
              <div className="lp-cap">
                <h3>Every movie and show. No subscriptions.</h3>
              </div>
            </div>
            <div className="lp-tile t-phone">
              <div className="lp-mini">
                <Phone>
                  <div className="lp-remote" aria-hidden="true">
                    <p>Searching on the TV</p>
                    <div className="lp-remote-field">
                      <span className="lp-typed">Dune</span>
                    </div>
                    <div className="lp-remote-mic" />
                    <div className="lp-remote-hits">
                      <img src={`${M}p1170608.jpg`} alt="" loading="lazy" />
                    </div>
                  </div>
                </Phone>
                <Suspense fallback={null}>
                  <PhoneModel model={`${M}iphone.glb`} searchPoster={`${M}p1170608.jpg`} turn={0.22} />
                </Suspense>
              </div>
              <span className="lp-tag">
                <b>●</b> Fire TV
              </span>
              <div className="lp-cap">
                <h3>Search the TV from your phone</h3>
              </div>
            </div>
            <div className="lp-tile t-hover t-versions">
              <img className="bg" src={`${M}hero.jpg`} alt="" loading="lazy" />
              <div className="lp-pop" aria-hidden="true">
                <small>Version 1 of 6</small>
                <span className="dim">720p</span>
                <span className="lit">
                  Try another version
                  <em>Version 2 of 6 · 4K</em>
                </span>
              </div>
              <span className="lp-tag">
                <b>●</b> Versions
              </span>
              <div className="lp-cap">
                <h3>Bad copy? Try another version.</h3>
              </div>
            </div>
            <div className="lp-tile t-play">
              <Loop name="play" className="bg" label="A show playing in Cue's player" />
              <span className="lp-skip" aria-hidden="true">
                Skip intro <b>›</b>
              </span>
              <span className="lp-tag">
                <b>●</b> Player
              </span>
              <div className="lp-cap">
                <h3>Skips the intro for you</h3>
              </div>
            </div>
            <div className="lp-tile t-live">
              <img className="bg" src={`${M}live.jpg`} alt="Live TV with a guide of channels" loading="lazy" />
              <span className="lp-tag">
                <b>●</b> Live
              </span>
              <div className="lp-cap">
                <h3>Live TV with a full guide</h3>
              </div>
            </div>
            <div className="lp-tile t-year">
              <span className="lp-tag">
                <b>●</b> Year in Cue
              </span>
              <div className="lp-num">
                <span ref={hours}>212.5</span>
                <small>hours</small>
              </div>
              <h3>Your year in Cue</h3>
            </div>
            <div className="lp-tile t-people">
              <div className="lp-devices" aria-hidden="true">
                {['Fire TV', 'Google TV', 'Apple TV', 'iPhone', 'Android', 'Laptop'].map((d) => (
                  <span key={d}>{d}</span>
                ))}
              </div>
              <h3>On every screen</h3>
            </div>
          </div>
        </div>
      </section>

      <section className="lp-part" id="ad">
        <div className="lp-wrap">
          <div className="lp-head">
            <h2>The ad</h2>
          </div>
          <div className="lp-cinema" ref={cinema}>
            {adOn ? (
              <video ref={ad} src={`${M}commercial.mp4`} poster={`${M}commercial.jpg`} controls playsInline aria-label="The Cue commercial" />
            ) : (
              <>
                <img src={`${M}commercial.jpg`} alt="Meet Cue, from the commercial" />
                <button className="lp-play" type="button" onClick={() => setAdOn(true)} aria-label="Play the commercial with sound">
                  <span>
                    <PlayIcon />
                    Play the commercial
                  </span>
                </button>
              </>
            )}
          </div>
        </div>
      </section>

      <section className="lp-part" id="get">
        <div className="lp-wrap">
          <div className="lp-head">
            <h2>Get Cue</h2>
          </div>
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
            <div className="lp-panel" role="tabpanel" id="lp-p-tv" aria-labelledby="lp-tab-tv">
              <div className="lp-step">
                <span className="n">1</span>
                <h3>Get Downloader</h3>
                <p>
                  From the Amazon Appstore or Play Store.
                </p>
              </div>
              <div className="lp-step">
                <span className="n">2</span>
                <h3>Allow installs</h3>
                <p>Fire TV: Settings &gt; My Fire TV &gt; Developer options &gt; Install unknown apps &gt; Downloader.</p>
              </div>
              <div className="lp-step">
                <span className="n">3</span>
                <h3>Enter this in Downloader</h3>
                <Copy text={`${host}/tv.apk`} />
              </div>
            </div>
          )}
          {tab === 'web' && (
            <div className="lp-panel" role="tabpanel" id="lp-p-web" aria-labelledby="lp-tab-web">
              <div className="lp-step">
                <span className="n">1</span>
                <h3>Open Cue</h3>
                <Copy text={host} />
              </div>
              <div className="lp-step">
                <span className="n">2</span>
                <h3>iPhone or iPad</h3>
                <p>
                  Share &gt; <b>Add to Home Screen</b>
                </p>
              </div>
              <div className="lp-step">
                <span className="n">3</span>
                <h3>Android</h3>
                <p>
                  Chrome &gt; <b>Install app</b>
                </p>
              </div>
            </div>
          )}
          {tab === 'apple' && (
            <div className="lp-panel" role="tabpanel" id="lp-p-apple" aria-labelledby="lp-tab-apple">
              <div className="lp-step">
                <span className="n">1</span>
                <h3>Ask whoever runs Cue</h3>
              </div>
              <div className="lp-step">
                <span className="n">2</span>
                <h3>Tell them which Apple TV</h3>
              </div>
              <div className="lp-step">
                <span className="n">3</span>
                <h3>Open Cue</h3>
              </div>
            </div>
          )}

        </div>
      </section>

      <footer className="lp-wrap lp-foot">
        <span>Posters and artwork from TMDB.</span>
      </footer>
    </div>
  )
}
