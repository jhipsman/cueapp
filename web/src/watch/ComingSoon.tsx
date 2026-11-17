import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchCard } from '../api'
import Icon from '../components/Icon'
import { shortDate, titleHref } from './parts'

// Coming soon: films not out to stream yet, kept apart from everything you
// can watch now. Each has its day (when TMDB knows it), its picture, what
// it's about, and Remind me: Cue tells you once it's out (at the top of Home,
// and on your phone with alerts on).

const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']

function DateBlock({ day }: { day?: string }) {
  const m = day ? /^\d{4}-(\d{2})-(\d{2})/.exec(day) : null
  return (
    <div className="wx-soon-date" aria-hidden="true">
      {m ? (
        <>
          <span>{MONTHS[Number(m[1]) - 1]}</span>
          <b>{Number(m[2])}</b>
        </>
      ) : (
        <span>SOON</span>
      )}
    </div>
  )
}

export default function ComingSoon() {
  const [items, setItems] = useState<WatchCard[] | null>(null)
  const [error, setError] = useState('')
  const navigate = useNavigate()
  useEffect(() => {
    api
      .comingSoon()
      .then(setItems)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])

  const toggle = async (c: WatchCard) => {
    const next = !c.reminding
    setItems((l) => l?.map((x) => (x.kind === c.kind && x.tmdbId === c.tmdbId ? { ...x, reminding: next } : x)) ?? l)
    try {
      await api.setRemind(c.kind, c.tmdbId, next)
    } catch {
      setItems((l) => l?.map((x) => (x.kind === c.kind && x.tmdbId === c.tmdbId ? { ...x, reminding: !next } : x)) ?? l)
    }
  }

  return (
    <div className="wx-page wx-soon">
      <h1>Coming soon</h1>
      <p className="wx-dim">Films on their way. Press Remind me and Cue tells you the day you can watch.</p>
      {error && <p className="wx-error">{error}</p>}
      {items === null && !error && <div className="wx-soon-item wx-skeleton" style={{ height: 260 }} />}
      {items?.length === 0 && <p className="wx-dim">Nothing announced right now.</p>}
      {items?.map((c) => (
        <article className="wx-soon-item" key={`${c.kind}-${c.tmdbId}`}>
          <DateBlock day={c.outDate} />
          <div className="wx-soon-body">
            <button className="wx-soon-pic" onClick={() => navigate(titleHref(c))} aria-label={c.title}>
              {c.backdropUrl && <img src={c.backdropUrl} alt="" loading="lazy" />}
            </button>
            <div className="wx-soon-head">
              <h2>{c.title}</h2>
              <div className="wx-soon-actions">
                <button className={`wx-btn small${c.reminding ? '' : ' play'}`} onClick={() => void toggle(c)} aria-pressed={!!c.reminding}>
                  <Icon name={c.reminding ? 'check' : 'clock'} size={16} /> {c.reminding ? 'Reminding you' : 'Remind me'}
                </button>
                <button className="wx-btn small" onClick={() => navigate(titleHref(c))}>
                  <Icon name="info" size={16} /> Info
                </button>
              </div>
            </div>
            <p className="wx-soon-when">{c.outDate ? `Streaming ${shortDate(c.outDate)}` : 'Streaming date not announced yet'}</p>
            {c.overview && <p className="wx-soon-about">{c.overview}</p>}
          </div>
        </article>
      ))}
    </div>
  )
}
