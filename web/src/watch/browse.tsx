import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type TopList, type WatchCard, type WatchHome, type WatchKind, type WeekEpisode } from '../api'
import Icon from '../components/Icon'
import { Card, epCode, ordinal, titleHref } from './parts'
import { useProfiles } from './profiles'

// Ways in: the Top 10 on each service, Home's greeting, the All / Movies /
// Shows pills, the genre tiles on Search and This week on Coming soon.

// ---- Pills: All, Movies, Shows (Search and Coming soon alike).

export type KindFilter = 'all' | WatchKind

export function Pills({ value, onChange, counts }: { value: KindFilter; onChange: (v: KindFilter) => void; counts?: Partial<Record<KindFilter, number>> }) {
  const opts: [KindFilter, string][] = [
    ['all', 'All'],
    ['movie', 'Movies'],
    ['tv', 'Shows'],
  ]
  return (
    <div className="wx-pills" role="tablist">
      {opts.map(([k, label]) => (
        <button key={k} role="tab" aria-selected={value === k} className={`wx-pill${value === k ? ' on' : ''}`} onClick={() => onChange(k)}>
          {label}
          {counts?.[k] !== undefined ? <span className="wx-pill-n">{counts[k]}</span> : null}
        </button>
      ))}
    </div>
  )
}

// ---- Top 10 on each service: tabs for the services, the ten titles with
// big numbers beside them.

export function TopTen({ lists, only }: { lists: TopList[]; only?: WatchKind }) {
  const shown = lists.map((l) => ({ ...l, items: only ? l.items.filter((c) => c.kind === only) : l.items })).filter((l) => l.items.length > 0)
  const [key, setKey] = useState(shown[0]?.key ?? '')
  const strip = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()
  const list = shown.find((l) => l.key === key) ?? shown[0]
  useEffect(() => {
    strip.current?.scrollTo({ left: 0 })
  }, [key])
  if (!list) return null
  return (
    <section className="wx-row wx-top">
      <div className="wx-top-head">
        <h2>Top 10 on {list.name} today</h2>
        <button className="wx-top-all" onClick={() => navigate(`/watch/top?service=${list.key}${only ? `&kind=${only}` : ''}`)}>
          See all ›
        </button>
        <div className="wx-top-tabs" role="tablist" aria-label="Streaming service">
          {shown.map((l) => (
            <button key={l.key} role="tab" aria-selected={l.key === list.key} className={`wx-top-tab ${l.key}${l.key === list.key ? ' on' : ''}`} onClick={() => setKey(l.key)}>
              {l.name}
            </button>
          ))}
        </div>
      </div>
      <div className="wx-row-scroll wx-top-scroll" ref={strip}>
        {list.items.slice(0, 10).map((c, i) => (
          <div className="wx-top-item" key={`${list.key}-${c.kind}-${c.tmdbId}`}>
            <span className="wx-top-n" aria-hidden="true">
              {i + 1}
            </span>
            <Card card={c} />
          </div>
        ))}
      </div>
    </section>
  )
}

// ---- Greeting: "Good evening, Sam" and a few tiles saying what's new.

function partOfDay(h: number): string {
  if (h < 5) return 'Up late'
  if (h < 12) return 'Good morning'
  if (h < 17) return 'Good afternoon'
  return 'Good evening'
}

export function Greeting({ home }: { home: WatchHome }) {
  const name = useProfiles().active?.name
  const navigate = useNavigate()
  const [week, setWeek] = useState<WeekEpisode[] | null>(null)
  const [parties, setParties] = useState(0)
  useEffect(() => {
    let live = true
    api
      .thisWeek()
      .then((w) => live && setWeek(w ?? []))
      .catch(() => live && setWeek([]))
    api
      .partiesNow()
      .then((p) => live && setParties(p.length))
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [])
  const row = (k: string) => home.rows.find((r) => r.key === k)
  const fresh = row('new-episodes')?.items.length ?? 0
  const going = row('continue')?.items.length ?? 0
  const soonDays = new Set((week ?? []).map((e) => e.airDate)).size
  const go = (k: string) => document.querySelector(`[data-row="${k}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  const tiles: { key: string; n: number; label: string; icon: 'sparkles' | 'play' | 'calendar' | 'users'; onClick: () => void }[] = []
  if (fresh) tiles.push({ key: 'new', n: fresh, label: fresh === 1 ? 'new episode' : 'new episodes', icon: 'sparkles', onClick: () => go('new-episodes') })
  if (week?.length)
    tiles.push({ key: 'week', n: week.length, label: `coming ${soonDays > 1 ? 'in the next two weeks' : 'soon'}`, icon: 'calendar', onClick: () => navigate('/watch/coming-soon?show=tv') })
  if (parties) tiles.push({ key: 'party', n: parties, label: parties === 1 ? 'watch party on now' : 'watch parties on now', icon: 'users', onClick: () => go('parties') })
  if (going) tiles.push({ key: 'go', n: going, label: going === 1 ? 'title to carry on with' : 'titles to carry on with', icon: 'play', onClick: () => go('continue') })
  return (
    <section className="wx-greet" aria-label="What's new">
      <h2>
        {partOfDay(new Date().getHours())}
        {name ? `, ${name}` : ''}
      </h2>
      {tiles.length > 0 && (
        <div className="wx-greet-tiles">
          {tiles.map((t) => (
            <button key={t.key} className={`wx-greet-tile ${t.key}`} onClick={t.onClick}>
              <Icon name={t.icon} size={18} />
              <b>{t.n}</b>
              <span>{t.label}</span>
            </button>
          ))}
        </div>
      )}
    </section>
  )
}

// ---- Genre tiles (Search before anything's typed).

export interface GenreTile {
  name: string
  movie?: number // TMDB's genre id for films
  tv?: number // and for shows
  from: string
  to: string
}

export const GENRES: GenreTile[] = [
  { name: 'Action', movie: 28, tv: 10759, from: '#ff512f', to: '#dd2476' },
  { name: 'Comedy', movie: 35, tv: 35, from: '#f7b733', to: '#fc4a1a' },
  { name: 'Drama', movie: 18, tv: 18, from: '#614385', to: '#516395' },
  { name: 'Sci-Fi', movie: 878, tv: 10765, from: '#00c6ff', to: '#0072ff' },
  { name: 'Family', movie: 10751, tv: 10751, from: '#56ab2f', to: '#a8e063' },
  { name: 'Animation', movie: 16, tv: 16, from: '#ee9ca7', to: '#c471ed' },
  { name: 'Crime', movie: 80, tv: 80, from: '#434343', to: '#8e0e00' },
  { name: 'Mystery', movie: 9648, tv: 9648, from: '#141e30', to: '#4b6cb7' },
  { name: 'Thriller', movie: 53, from: '#232526', to: '#b31217' },
  { name: 'Romance', movie: 10749, from: '#ff758c', to: '#ff7eb3' },
  { name: 'Documentary', movie: 99, tv: 99, from: '#11998e', to: '#38ef7d' },
  { name: 'Reality', tv: 10764, from: '#f857a6', to: '#ff5858' },
  { name: 'Kids', movie: 10751, tv: 10762, from: '#36d1dc', to: '#5b86e5' },
  { name: 'Fantasy', movie: 14, tv: 10765, from: '#8e2de2', to: '#4a00e0' },
  { name: 'History & War', movie: 10752, tv: 10768, from: '#7f5a3c', to: '#c0874f' },
  { name: 'Western', movie: 37, tv: 37, from: '#c79081', to: '#dfa579' },
]

export function GenreTiles({ kind }: { kind: KindFilter }) {
  const navigate = useNavigate()
  const tiles = GENRES.filter((g) => kind === 'all' || (kind === 'movie' ? g.movie : g.tv))
  return (
    <section className="wx-genres" aria-label="Browse by genre">
      <h2>Browse by genre</h2>
      <div className="wx-genre-grid">
        {tiles.map((g) => (
          <button
            key={g.name}
            className="wx-genre"
            style={{ background: `linear-gradient(135deg, ${g.from}, ${g.to})` }}
            onClick={() => navigate(`/watch/search?genre=${encodeURIComponent(g.name)}${kind === 'all' ? '' : `&kind=${kind}`}`)}
          >
            <span>{g.name}</span>
          </button>
        ))}
      </div>
    </section>
  )
}

// useGenre loads a genre's titles: films, shows, or both mixed (alternating,
// most popular first).
export function useGenre(name: string, kind: KindFilter): { items: WatchCard[] | null; error: string } {
  const [state, setState] = useState<{ key: string; items: WatchCard[] | null; error: string }>({ key: '', items: null, error: '' })
  const g = GENRES.find((x) => x.name === name)
  const key = `${name}|${kind}`
  useEffect(() => {
    if (!g) return
    let live = true
    const want: Promise<WatchCard[]>[] = []
    if ((kind === 'all' || kind === 'movie') && g.movie) want.push(api.watchGenre('movie', g.movie))
    if ((kind === 'all' || kind === 'tv') && g.tv) want.push(api.watchGenre('tv', g.tv))
    Promise.all(want)
      .then((lists) => {
        const out: WatchCard[] = []
        for (let i = 0; i < Math.max(0, ...lists.map((l) => l.length)); i++) for (const l of lists) if (l[i]) out.push(l[i])
        if (live) setState({ key, items: out, error: '' })
      })
      .catch((e) => live && setState({ key, items: [], error: e instanceof Error ? e.message : String(e) }))
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])
  if (!g) return { items: [], error: '' }
  return state.key === key ? state : { items: null, error: '' }
}

// ---- This week: the next episodes of shows you watch, by day.

function dayName(day: string): string {
  const [y, m, d] = day.split('-').map(Number)
  const at = new Date(y, m - 1, d)
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const diff = Math.round((at.getTime() - today.getTime()) / 86_400_000)
  if (diff === 0) return 'Today'
  if (diff === 1) return 'Tomorrow'
  const wd = at.toLocaleDateString('en-US', { weekday: 'long' })
  const mo = at.toLocaleDateString('en-US', { month: 'long' })
  return `${wd}, ${mo} ${ordinal(d)}`
}

export function ThisWeek({ items }: { items: WeekEpisode[] }) {
  const navigate = useNavigate()
  if (items.length === 0) return null
  const days: [string, WeekEpisode[]][] = []
  for (const e of items) {
    const last = days[days.length - 1]
    if (last && last[0] === e.airDate) last[1].push(e)
    else days.push([e.airDate, [e]])
  }
  return (
    <section className="wx-week" aria-label="New episodes coming">
      <h2>New episodes coming</h2>
      <div className="wx-week-days">
        {days.map(([day, eps]) => (
          <div className="wx-week-day" key={day}>
            <h3>{dayName(day)}</h3>
            {eps.map((e) => (
              <button className="wx-week-ep" key={`${e.tmdbId}-${e.season}-${e.episode}`} title={e.title} onClick={() => navigate(titleHref(e))}>
                {e.backdropUrl || e.posterUrl ? <img src={(e.backdropUrl || e.posterUrl)!.replace(/\/t\/p\/w\d+\//, '/t/p/w300/')} alt="" loading="lazy" /> : <span />}
                <span className="wx-week-txt">
                  <b>{e.title}</b>
                  <small>
                    {epCode(e.season, e.episode)}
                    {e.episodeTitle ? ` · ${e.episodeTitle}` : ''}
                  </small>
                </span>
              </button>
            ))}
          </div>
        ))}
      </div>
    </section>
  )
}

// ---- Networks and decades (the Films and Series homes): each opens the
// Top 10 page for it.

export function NetworkStrip({ kind, services }: { kind: WatchKind; services: { key: string; name: string }[] }) {
  const navigate = useNavigate()
  const what = kind === 'movie' ? 'films' : 'shows'
  return (
    <section className="wx-networks" aria-label={`Browse ${what}`}>
      <div className="wx-network-row">
        {services.map((s) => (
          <button key={s.key} className={`wx-network ${s.key}`} onClick={() => navigate(`/watch/top?service=${s.key}&kind=${kind}`)}>
            {s.name}
          </button>
        ))}
      </div>
      <div className="wx-network-row decades">
        {[2020, 2010, 2000, 1990, 1980, 1970].map((d) => (
          <button key={d} className="wx-decade" onClick={() => navigate(`/watch/top?decade=${d}&kind=${kind}`)}>
            {d}s
          </button>
        ))}
      </div>
    </section>
  )
}
