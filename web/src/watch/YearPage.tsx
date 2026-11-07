import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchYear } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { TitleMark, useExtras, useTint } from './parts'
import { useProfiles } from './profiles'

// Your year in Cue: the profile's year told in full-screen slides, one
// after another (tap, click or an arrow key to move on): the hours, the show
// of the year, the top five, the movie, the busiest day.

const SLIDE_MS = 6500

function hours(h: number): string {
  if (h < 1) return `${Math.round(h * 60)} minutes`
  return `${h % 1 === 0 ? h : h.toFixed(1)} hour${h === 1 ? '' : 's'}`
}

function day(iso: string): string {
  const d = new Date(`${iso}T12:00:00`)
  return d.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })
}

interface Slide {
  key: string
  picture?: string
  body: React.ReactNode
}

export default function YearPage() {
  const navigate = useNavigate()
  const { active } = useProfiles()
  const [year, setYear] = useState<WatchYear | null>(null)
  const [error, setError] = useState('')
  const [at, setAt] = useState(0)
  const [paused, setPaused] = useState(false)
  useDocumentTitle('Your year in Cue')

  useEffect(() => {
    api
      .watchYear()
      .then(setYear)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])

  const top = year?.top[0]
  const topExtras = useExtras(top ?? null)
  useTint(topExtras?.colour)

  const slides: Slide[] = useMemo(() => {
    if (!year) return []
    const s: Slide[] = []
    const name = active?.name ?? ''
    s.push({
      key: 'hello',
      picture: year.top[0]?.backdropUrl,
      body: (
        <>
          <p className="wx-yr-kicker">{name ? `${name}, this is` : 'This is'}</p>
          <h1 className="wx-yr-huge">
            {year.year}
            <span>in Cue</span>
          </h1>
        </>
      ),
    })
    if (year.titles === 0) {
      s.push({
        key: 'none',
        body: (
          <>
            <h2 className="wx-yr-big">Nothing watched yet this year.</h2>
            <p className="wx-yr-line">Press Play on something and come back: this fills in as you watch.</p>
          </>
        ),
      })
      return s
    }
    s.push({
      key: 'hours',
      body: (
        <>
          <p className="wx-yr-kicker">You spent</p>
          <h2 className="wx-yr-number">{hours(year.hours)}</h2>
          <p className="wx-yr-line">
            watching {year.titles} title{year.titles === 1 ? '' : 's'} on {year.days} day{year.days === 1 ? '' : 's'}
            {year.episodes > 0 ? `, ${year.episodes} episode${year.episodes === 1 ? '' : 's'} among them` : ''}.
          </p>
          {year.first && <p className="wx-yr-small">It started with {year.first}.</p>}
        </>
      ),
    })
    if (top) {
      s.push({
        key: 'show',
        picture: top.backdropUrl,
        body: <ShowOfTheYear show={top} />,
      })
    }
    if (year.top.length > 1) {
      s.push({
        key: 'top',
        body: (
          <>
            <p className="wx-yr-kicker">Your top shows</p>
            <ol className="wx-yr-list">
              {year.top.map((t, i) => (
                <li key={t.tmdbId} style={{ animationDelay: `${0.15 + i * 0.12}s` }}>
                  <b>{i + 1}</b>
                  {t.backdropUrl ? <img src={t.backdropUrl.replace('/w1280/', '/w300/')} alt="" /> : <span className="wx-yr-blank" />}
                  <span>
                    {t.title}
                    <small>
                      {hours(t.hours)} · {t.episodes} episode{t.episodes === 1 ? '' : 's'}
                    </small>
                  </span>
                </li>
              ))}
            </ol>
          </>
        ),
      })
    }
    if (year.topMovie) {
      s.push({
        key: 'movie',
        picture: year.topMovie.backdropUrl,
        body: (
          <>
            <p className="wx-yr-kicker">Movie of the year</p>
            <h2 className="wx-yr-big">{year.topMovie.title}</h2>
            <p className="wx-yr-line">One of {year.movies} movie{year.movies === 1 ? '' : 's'} you watched.</p>
          </>
        ),
      })
    }
    if (year.busiestDay) {
      s.push({
        key: 'day',
        body: (
          <>
            <p className="wx-yr-kicker">Your biggest day</p>
            <h2 className="wx-yr-big">{day(year.busiestDay)}</h2>
            <p className="wx-yr-line">
              {hours(year.busiestDayHours ?? 0)} in one day.
              {year.busiestMonth ? ` And ${year.busiestMonth} was your month.` : ''}
            </p>
          </>
        ),
      })
    }
    s.push({
      key: 'end',
      picture: year.top[1]?.backdropUrl ?? year.top[0]?.backdropUrl,
      body: (
        <>
          <h2 className="wx-yr-big">See you on the couch in {year.year + 1}.</h2>
          <div className="wx-actions">
            <button className="wx-btn play" onClick={() => navigate('/watch')}>
              Back to Watch
            </button>
            <button className="wx-btn" onClick={() => setAt(0)}>
              Watch it again
            </button>
          </div>
        </>
      ),
    })
    return s
  }, [year, top, active?.name, navigate])

  // Slides move on by themselves, unless held (pointer down) or at the end.
  useEffect(() => {
    if (!slides.length || paused || at >= slides.length - 1) return
    const t = window.setTimeout(() => setAt((i) => Math.min(i + 1, slides.length - 1)), SLIDE_MS)
    return () => window.clearTimeout(t)
  }, [at, paused, slides.length])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'ArrowRight' || e.key === ' ') setAt((i) => Math.min(i + 1, slides.length - 1))
      if (e.key === 'ArrowLeft') setAt((i) => Math.max(i - 1, 0))
      if (e.key === 'Escape') navigate('/watch')
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [slides.length, navigate])

  if (error) {
    return (
      <div className="wx-page">
        <p className="wx-error">{error}</p>
      </div>
    )
  }
  if (!year) return <div className="wx-yr" />
  const slide = slides[Math.min(at, slides.length - 1)]
  return (
    <div
      className="wx-yr"
      onPointerDown={() => setPaused(true)}
      onPointerUp={(e) => {
        setPaused(false)
        // Tap the left third to go back, anywhere else to go on.
        if ((e.target as HTMLElement).closest('button')) return
        const left = e.clientX < window.innerWidth / 3
        setAt((i) => (left ? Math.max(i - 1, 0) : Math.min(i + 1, slides.length - 1)))
      }}
    >
      {slides.map((s, i) =>
        s.picture ? <div key={s.key} className={`wx-yr-pic${i === at ? ' show' : ''}`} style={{ backgroundImage: `url(${s.picture})` }} /> : null,
      )}
      <div className="wx-yr-shade" />
      <div className="wx-yr-bars" aria-hidden="true">
        {slides.map((s, i) => (
          <span key={s.key}>
            <i className={i < at ? 'done' : i === at ? (paused ? 'now paused' : 'now') : ''} style={{ animationDuration: `${SLIDE_MS}ms` }} />
          </span>
        ))}
      </div>
      <button className="wx-yr-close" onClick={() => navigate('/watch')} aria-label="Close">
        <Icon name="x" size={22} />
      </button>
      <section className="wx-yr-body" key={slide.key} aria-live="polite">
        {slide.body}
      </section>
    </div>
  )
}

function ShowOfTheYear({ show }: { show: WatchYear['top'][number] }) {
  const ex = useExtras(show)
  return (
    <>
      <p className="wx-yr-kicker">Your show of the year</p>
      <TitleMark title={show.title} logo={ex === undefined ? undefined : (ex?.logo ?? '')} />
      <p className="wx-yr-line">
        {hours(show.hours)} across {show.episodes} episode{show.episodes === 1 ? '' : 's'}.
      </p>
    </>
  )
}
