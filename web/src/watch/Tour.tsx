import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import BrandMark from '../components/BrandMark'
import Icon from '../components/Icon'
import { useProfiles } from './profiles'

// The tour: a few slides the first time someone opens Watch, on what's
// worth knowing before the first bad version (Versions > Try another
// version), Skip intro and Next episode, and subtitles and searching from a
// phone. Each slide has a small drawing of the real control, lit. It shows
// once on this browser; "How Cue works" in the profile menu shows it again.

const SEEN = 'cue.tour.v1'

function seen(): boolean {
  try {
    return localStorage.getItem(SEEN) === '1'
  } catch {
    return true // no storage (a private window): don't show it every time
  }
}

function remember() {
  try {
    localStorage.setItem(SEEN, '1')
  } catch {
    /* nothing to keep it in */
  }
}

interface Slide {
  kicker: string
  title: string
  body: ReactNode
  art: ReactNode
}

const slides: Slide[] = [
  {
    kicker: 'Welcome',
    title: 'Everything to watch, in one place',
    body: 'Movies, shows and live TV for the whole house, each with your own profile, list and place in every episode. Here are three things worth knowing.',
    art: <WelcomeArt />,
  },
  {
    kicker: 'If something looks wrong',
    title: 'Try another version',
    body: (
      <>
        Cue finds several versions of everything. If one is blurry, in the wrong language, out of sync or keeps stopping, open <b>Versions</b> on the bar at the
        bottom and choose <b>Try another version</b>. On the TV it’s the <b>Other version</b> button. You carry on from the same moment.
      </>
    ),
    art: <VersionsArt />,
  },
  {
    kicker: 'Binge-friendly',
    title: 'Skip the intro, roll into the next',
    body: (
      <>
        <b>Skip intro</b> shows as an episode starts, and <b>Next episode</b> is always on the bar. Skip an intro once and Cue learns it for the whole show; when
        the credits roll, the next one is lined up.
      </>
    ),
    art: <SkipArt />,
  },
  {
    kicker: 'Small things, big difference',
    title: 'Subtitles, and search from your phone',
    body: (
      <>
        The <b>CC</b> button turns subtitles on and off. On a Fire TV, open <b>Search</b> and scan the code with your phone: type or say what you want there and
        it appears on the TV.
      </>
    ),
    art: <PhoneArt />,
  },
]

export default function Tour() {
  const { active } = useProfiles()
  const { pathname } = useLocation()
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const asked = params.get('tour') === '1'
  const home = pathname === '/watch' || pathname === '/watch/'
  const [open, setOpen] = useState(false)
  const [at, setAt] = useState(0)
  const [dir, setDir] = useState<1 | -1>(1)
  const card = useRef<HTMLDivElement>(null)

  // First time on Home with a profile picked, or asked for from the menu.
  useEffect(() => {
    if (!active) return
    if (asked || (home && !seen())) {
      const t = window.setTimeout(() => {
        setAt(0)
        setOpen(true)
      }, asked ? 0 : 700)
      return () => window.clearTimeout(t)
    }
  }, [active, asked, home])

  const close = useCallback(() => {
    remember()
    setOpen(false)
    if (asked) {
      params.delete('tour')
      setParams(params, { replace: true })
    }
  }, [asked, params, setParams])

  const go = useCallback(
    (to: number) => {
      if (to < 0) return
      if (to >= slides.length) return close()
      setDir(to > at ? 1 : -1)
      setAt(to)
    },
    [at, close],
  )

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
      else if (e.key === 'ArrowRight') go(at + 1)
      else if (e.key === 'ArrowLeft') go(at - 1)
      else return
      e.preventDefault()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, at, go, close])

  // Focus the main button on each slide, so Enter moves on.
  useEffect(() => {
    if (open) card.current?.querySelector<HTMLButtonElement>('.wx-tour-next')?.focus()
  }, [open, at])

  if (!open) return null
  const s = slides[at]
  const last = at === slides.length - 1
  return (
    <div className="wx-tour" role="dialog" aria-modal="true" aria-labelledby="wx-tour-title" onClick={(e) => e.target === e.currentTarget && close()}>
      <div className="wx-tour-card" ref={card}>
        <button className="wx-tour-close" onClick={close} aria-label="Close the tour">
          <Icon name="x" size={18} />
        </button>
        <div className={`wx-tour-art ${dir > 0 ? 'fwd' : 'back'}`} key={`art-${at}`} aria-hidden="true">
          {s.art}
        </div>
        <div className={`wx-tour-text ${dir > 0 ? 'fwd' : 'back'}`} key={`text-${at}`}>
          <p className="wx-tour-kicker">{s.kicker}</p>
          <h2 id="wx-tour-title">{s.title}</h2>
          <p className="wx-tour-body">{s.body}</p>
        </div>
        <div className="wx-tour-foot">
          <div className="wx-tour-dots" role="tablist" aria-label="Slides">
            {slides.map((x, i) => (
              <button
                key={x.title}
                role="tab"
                aria-selected={i === at}
                aria-label={`Slide ${i + 1} of ${slides.length}`}
                className={i === at ? 'on' : ''}
                onClick={() => go(i)}
              />
            ))}
          </div>
          <div className="wx-tour-buttons">
            {at > 0 ? (
              <button className="wx-tour-back" onClick={() => go(at - 1)}>
                Back
              </button>
            ) : (
              <button className="wx-tour-back" onClick={close}>
                Skip
              </button>
            )}
            <button
              className="wx-tour-next"
              onClick={() => {
                if (last) {
                  close()
                  navigate('/watch', { replace: true })
                } else go(at + 1)
              }}
            >
              {last ? 'Start watching' : at === 0 ? 'Show me' : 'Next'}
              {!last && <Icon name="open" size={16} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ---- The drawings: small, calm versions of the real screens ----

function Screen({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={`wx-tour-screen ${className ?? ''}`}>{children}</div>
}

function Bar({ children }: { children: ReactNode }) {
  return (
    <div className="wx-tour-bar">
      <div className="wx-tour-time">
        <span>18:42</span>
        <i>
          <b style={{ width: '34%' }} />
        </i>
        <span>-31:06</span>
      </div>
      <div className="wx-tour-row">
        <span className="wx-tour-btn">
          <Icon name="pause" size={14} fill="currentColor" />
        </span>
        <span className="wx-tour-gap" />
        {children}
      </div>
    </div>
  )
}

function WelcomeArt() {
  return (
    <Screen className="welcome">
      <div className="wx-tour-glow" />
      <div className="wx-tour-tiles">
        {[0, 1, 2, 3, 4].map((i) => (
          <span key={i} style={{ ['--i' as string]: i }} />
        ))}
      </div>
      <BrandMark className="wx-tour-mark" />
    </Screen>
  )
}

function VersionsArt() {
  return (
    <Screen className="versions">
      <div className="wx-tour-pic blurry" />
      <div className="wx-tour-pop">
        <div className="wx-tour-pop-head">Version 1 of 6</div>
        <div className="wx-tour-pop-item dim">720p · via Premiumize</div>
        <div className="wx-tour-pop-item lit">
          Try another version
          <small>Version 2 of 6</small>
        </div>
        <div className="wx-tour-pop-item dim">Search again</div>
      </div>
      <Bar>
        <span className="wx-tour-btn">
          <Icon name="captions" size={14} />
        </span>
        <span className="wx-tour-btn lit ring">
          <Icon name="layers" size={14} />
          Versions
        </span>
      </Bar>
    </Screen>
  )
}

function SkipArt() {
  return (
    <Screen className="skip">
      <div className="wx-tour-pic warm" />
      <span className="wx-tour-ep">S2 · E4</span>
      <span className="wx-tour-skip ring">
        Skip intro <Icon name="open" size={13} />
      </span>
      <Bar>
        <span className="wx-tour-btn lit">
          <Icon name="skip-forward" size={14} fill="currentColor" />
          Next episode
        </span>
      </Bar>
    </Screen>
  )
}

function PhoneArt() {
  return (
    <Screen className="phone">
      <div className="wx-tour-pic cool" />
      <div className="wx-tour-tvsearch">
        <span className="wx-tour-btn lit ring cc">
          <Icon name="captions" size={14} />
          CC
        </span>
        <div className="wx-tour-qr" />
      </div>
      <div className="wx-tour-handset">
        <div className="wx-tour-handset-field">
          <Icon name="search" size={11} />
          <span className="wx-tour-typing">Dune</span>
        </div>
        <div className="wx-tour-handset-mic" />
        <div className="wx-tour-handset-hits">
          <span />
          <span />
        </div>
      </div>
    </Screen>
  )
}
