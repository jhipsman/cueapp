import { useEffect, useRef, useState } from 'react'
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

function Mark() {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true">
      <g fill="#34D1BF">
        <path d="M25.96 7.64A13 13 0 1 0 25.96 24.36L22.51 21.46A8.5 8.5 0 1 1 22.51 10.54Z" />
        <path d="M13.5 11.2L21 16L13.5 20.8Z" />
      </g>
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

const FACES = [
  ['L', 'Living Room', '#34d1bf'],
  ['M', 'Mom', '#f0718f'],
  ['D', 'Dad', '#f2b84b'],
  ['K', 'Kids', '#9b8cff'],
]

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
            <p className="lp-eyebrow">
              <i />
              Now streaming at home
            </p>
            <h1>
              Every movie.
              <br />
              Every show.
              <br />
              <span className="lp-sheen">One app.</span>
            </h1>
            <p className="lp-lead">Cue is this household's own streaming app. Movies, shows and live TV, on the TV, your phone and your laptop.</p>
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
              <Loop name="phone" label="Cue on a phone, swiping through rows of shows" />
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
            <p className="lp-eyebrow">What's inside</p>
            <h2>
              It doesn't just list things. <em>It plays them.</em>
            </h2>
            <p>Every clip below is the real app, recorded as it runs.</p>
          </div>
          <div className="lp-bento">
            <div className="lp-tile t-title">
              <Loop name="title" className="bg" label="A title's page with its trailer playing behind the title" />
              <span className="lp-tag">
                <b>●</b> Title pages
              </span>
              <div className="lp-cap">
                <h3>Open anything and it comes alive</h3>
                <p>The trailer plays behind the title, with ratings from IMDb and Rotten Tomatoes, every episode, and more like it.</p>
              </div>
            </div>
            <div className="lp-tile t-phone">
              <div className="lp-mini">
                <Loop name="phone" label="Swiping through Cue on a phone" />
              </div>
              <span className="lp-tag">
                <b>●</b> Phone
              </span>
              <div className="lp-cap">
                <h3>In your pocket</h3>
                <p>Swipe through rows, tap to dive in.</p>
              </div>
            </div>
            <div className="lp-tile t-hover">
              <Loop name="hover" className="bg" label="Resting the pointer on a poster plays its trailer inside it" />
              <span className="lp-tag">
                <b>●</b> Hover
              </span>
              <div className="lp-cap">
                <h3>Rest on a poster, it plays</h3>
              </div>
            </div>
            <div className="lp-tile t-play">
              <Loop name="play" className="bg" label="A movie playing full screen in Cue's player" />
              <span className="lp-tag">
                <b>●</b> Player
              </span>
              <div className="lp-cap">
                <h3>Hit Play. Picks up where you stopped.</h3>
              </div>
            </div>
            <div className="lp-tile t-live">
              <img className="bg" src={`${M}live.jpg`} alt="Live TV with a guide of channels" loading="lazy" />
              <span className="lp-tag">
                <b>●</b> Live
              </span>
              <div className="lp-cap">
                <h3>Live TV and a full guide</h3>
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
              <p>Your top shows, your movie of the year, your biggest day on the couch.</p>
            </div>
            <div className="lp-tile t-people">
              <div className="lp-faces" aria-hidden="true">
                {FACES.map(([l, , c]) => (
                  <span key={l} style={{ background: c }}>
                    {l}
                  </span>
                ))}
              </div>
              <h3>Everyone gets their own profile</h3>
              <p className="lp-dim">Your list, your progress, your recommendations.</p>
            </div>
          </div>
        </div>
      </section>

      <section className="lp-part" id="ad">
        <div className="lp-wrap">
          <div className="lp-head">
            <p className="lp-eyebrow">The ad</p>
            <h2>
              Fifty seconds. <em>Sound on.</em>
            </h2>
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
          <div className="lp-meta">
            <span>0:50</span>
            <span>Original soundtrack</span>
            <span>Every screen is the real app</span>
          </div>
        </div>
      </section>

      <section className="lp-part" id="get">
        <div className="lp-wrap">
          <div className="lp-head">
            <p className="lp-eyebrow">Get it</p>
            <h2>
              On your screens in <em>five minutes.</em>
            </h2>
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
                  Install <b>Downloader</b> from the Amazon Appstore, or the Play Store on a Google TV.
                </p>
              </div>
              <div className="lp-step">
                <span className="n">2</span>
                <h3>Let it install apps</h3>
                <p>Fire TV: Settings &gt; My Fire TV &gt; Developer options &gt; Install unknown apps &gt; Downloader.</p>
              </div>
              <div className="lp-step">
                <span className="n">3</span>
                <h3>Type this in Downloader</h3>
                <Copy text={`${host}/tv.apk`} />
                <p>After that, Cue updates itself.</p>
              </div>
            </div>
          )}
          {tab === 'web' && (
            <div className="lp-panel" role="tabpanel" id="lp-p-web" aria-labelledby="lp-tab-web">
              <div className="lp-step">
                <span className="n">1</span>
                <h3>Open Cue</h3>
                <Copy text={host} />
                <p>In Safari, Chrome or any browser.</p>
              </div>
              <div className="lp-step">
                <span className="n">2</span>
                <h3>iPhone or iPad</h3>
                <p>
                  Tap Share, then <b>Add to Home Screen</b>. It opens full screen, like an app.
                </p>
              </div>
              <div className="lp-step">
                <span className="n">3</span>
                <h3>Android</h3>
                <p>
                  Chrome offers <b>Install app</b>. Tap it and Cue sits with your other apps.
                </p>
              </div>
            </div>
          )}
          {tab === 'apple' && (
            <div className="lp-panel" role="tabpanel" id="lp-p-apple" aria-labelledby="lp-tab-apple">
              <div className="lp-step">
                <span className="n">1</span>
                <h3>It's set up for you</h3>
                <p>The Apple TV app goes on from a Mac, so whoever runs this Cue puts it on.</p>
              </div>
              <div className="lp-step">
                <span className="n">2</span>
                <h3>Say which TV</h3>
                <p>Let them know which Apple TV you'd like it on.</p>
              </div>
              <div className="lp-step">
                <span className="n">3</span>
                <h3>Then just open it</h3>
                <p>Same profiles, same Continue Watching, on the big screen.</p>
              </div>
            </div>
          )}

          <div className="lp-signin">
            <div>
              <p className="lp-eyebrow">Signing in</p>
              <h2>Sign in, then pick your profile.</h2>
              <p>Ask whoever set up Cue for your sign-in. Your profile keeps your own list, your progress and your year in Cue.</p>
              <Link className="lp-btn primary" to="/login">
                Sign in
              </Link>
            </div>
            <div className="lp-who" aria-hidden="true">
              {FACES.map(([l, name, c]) => (
                <div key={l}>
                  <span style={{ background: c }}>{l}</span>
                  {name}
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <footer className="lp-wrap lp-foot">
        <span>Cue, a household's own streaming app.</span>
        <span>Posters and artwork from TMDB.</span>
      </footer>
    </div>
  )
}
