import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type FavoriteTeam, type GuideGame, type SportsAnswer, type SportsGame, type SportsTeam } from '../api'
import Icon from '../components/Icon'
import ChannelLogo from './ChannelLogo'
import { liveHref } from './LivePage'

// Sports on Live TV: the next big game on top (with the teams, the live
// score, and where it's on), then the games on now and coming up. The
// schedule and scores come from ESPN; Watch goes to the channel in your
// Live TV list that carries it. Your teams' games come first, and with
// alerts on your phone hears before kickoff.

const hhmm = (s: string) => new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
function dayTime(s: string): string {
  const start = new Date(s)
  const today = new Date()
  const tomorrow = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 1)
  const day = start.toDateString() === today.toDateString() ? 'Today' : start.toDateString() === tomorrow.toDateString() ? 'Tomorrow' : start.toLocaleDateString([], { weekday: 'long' })
  return `${day} ${hhmm(s)}`
}
function countdown(s: string): string {
  const mins = Math.round((new Date(s).getTime() - Date.now()) / 60000)
  if (mins <= 0) return 'Starting'
  if (mins < 60) return `In ${mins} min`
  const h = Math.floor(mins / 60)
  return h < 24 ? `In ${h}h ${mins % 60}m` : dayTime(s)
}

export default function SportsHub() {
  const navigate = useNavigate()
  const [data, setData] = useState<SportsAnswer | null>(null)
  const [picking, setPicking] = useState(false)
  const load = () =>
    api
      .liveSports()
      .then(setData)
      .catch(() => undefined)
  useEffect(() => {
    void load()
    const t = window.setInterval(load, 60_000)
    return () => window.clearInterval(t)
  }, [])
  if (!data || (data.now.length === 0 && data.soon.length === 0)) return null
  if (data.source !== 'espn') return <GuideSports data={data as { now: GuideGame[]; soon: GuideGame[]; big?: GuideGame }} />
  const open = (g: SportsGame) => g.channel && navigate(liveHref(g.channel.id, 'all'))
  const big = data.big
  const same = (a: SportsGame, b?: SportsGame) => !!b && a.id === b.id && a.league === b.league
  return (
    <section className="wx-sports" aria-label="Sports">
      <div className="wx-sports-head">
        <h2>
          <Icon name="activity" size={20} /> Sports
        </h2>
        <button className="wx-btn small" onClick={() => setPicking(true)}>
          <Icon name="star" size={15} /> {data.teams && data.teams.length > 0 ? `Your teams (${data.teams.length})` : 'Pick your teams'}
        </button>
      </div>
      {big && <BigGame g={big} onWatch={() => open(big)} />}
      {[
        ['On now', data.now.filter((g) => !same(g, big))] as const,
        ['Coming up', data.soon.filter((g) => !same(g, big))] as const,
      ].map(([label, list]) =>
        list.length === 0 ? null : (
          <div key={label}>
            <h3>{label}</h3>
            <div className="wx-sports-row">
              {list.slice(0, 30).map((g) => (
                <GameCard key={`${g.league}-${g.id}`} g={g} onWatch={() => open(g)} />
              ))}
            </div>
          </div>
        ),
      )}
      <p className="wx-sports-credit">Schedules and scores from ESPN.</p>
      {picking && (
        <TeamPicker
          onClose={() => {
            setPicking(false)
            void load()
          }}
        />
      )}
    </section>
  )
}

function Side({ t, big }: { t: SportsTeam; big?: boolean }) {
  return (
    <span className={`wx-sports-team${t.winner ? ' won' : ''}`}>
      {t.logo ? <img src={t.logo} alt="" loading="lazy" /> : <span className="wx-sports-abbr">{t.abbr || t.name.slice(0, 3)}</span>}
      <span className="wx-sports-tname">
        <b>{big ? t.name : t.short || t.name}</b>
        {t.record && <small>{t.record}</small>}
      </span>
      {t.score !== undefined && t.score !== '' && <span className="wx-sports-score">{t.score}</span>}
    </span>
  )
}

function Where({ g }: { g: SportsGame }) {
  if (g.channel)
    return (
      <span className="wx-sports-where">
        <ChannelLogo name={g.channel.name} src={g.channel.logo} />
        {g.channel.name}
      </span>
    )
  if (g.streaming) return <span className="wx-sports-where">On {g.streaming}</span>
  if (g.networks?.length) return <span className="wx-sports-where">On {g.networks[0]}</span>
  return null
}

function BigGame({ g, onWatch }: { g: SportsGame; onWatch: () => void }) {
  const live = g.state === 'in'
  const tint = g.home.color ? `#${g.home.color}` : undefined
  return (
    <div className={`wx-sports-big glass${live ? ' live' : ''}`} style={tint ? { background: `linear-gradient(120deg, ${tint}66, rgba(30,30,40,0.55) 60%)` } : undefined}>
      <span className="wx-sports-tag">
        {live ? '● LIVE' : g.favorite ? '★ YOUR TEAM' : 'NEXT BIG GAME'}
        {` · ${g.leagueName}`}
        {g.note ? ` · ${g.note}` : ''}
      </span>
      <div className="wx-sports-matchup">
        <Side t={g.away} big />
        <span className="wx-sports-at">{live ? g.detail : '@'}</span>
        <Side t={g.home} big />
      </div>
      <div className="wx-sports-foot">
        {!live && <span>{countdown(g.start)}</span>}
        <Where g={g} />
        {g.channel && (
          <button className="wx-btn play small" onClick={onWatch}>
            <Icon name="play" size={16} /> {live ? 'Watch' : 'Go to channel'}
          </button>
        )}
      </div>
    </div>
  )
}

function GameCard({ g, onWatch }: { g: SportsGame; onWatch: () => void }) {
  const live = g.state === 'in'
  return (
    <button className={`wx-sports-game${live ? ' live' : ''}${g.favorite ? ' fav' : ''}`} onClick={onWatch} disabled={!g.channel} title={g.channel ? `Watch on ${g.channel.name}` : undefined}>
      <small className="wx-sports-league">
        {g.favorite ? '★ ' : ''}
        {g.leagueName}
        {g.note ? ` · ${g.note}` : ''}
      </small>
      <Side t={g.away} />
      <Side t={g.home} />
      <small className="wx-sports-when">{live ? g.detail : dayTime(g.start)}</small>
      <Where g={g} />
    </button>
  )
}

// TeamPicker: the leagues, and each league's teams to follow.
function TeamPicker({ onClose }: { onClose: () => void }) {
  const [leagues, setLeagues] = useState<{ key: string; name: string }[]>([])
  const [league, setLeague] = useState('nfl')
  const [teams, setTeams] = useState<SportsTeam[] | null>(null)
  const [mine, setMine] = useState<FavoriteTeam[]>([])
  const [error, setError] = useState('')
  useEffect(() => {
    let live = true
    setTeams(null)
    setError('')
    api
      .sportsTeams(league)
      .then((r) => {
        if (!live) return
        setLeagues(r.leagues)
        setMine(r.mine)
        setTeams(r.teams ?? [])
      })
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [league])
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])
  const followed = (t: SportsTeam) => mine.some((m) => m.league === league && m.id === t.id)
  const toggle = async (t: SportsTeam) => {
    try {
      setMine((await api.followTeam(league, t, !followed(t))).mine)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }
  return (
    <div className="wx-report-back" role="dialog" aria-modal="true" aria-label="Your teams" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className="wx-teampick glass">
        <div className="wx-teampick-head">
          <h2>Your teams</h2>
          <button className="wx-news-x" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>
        <p className="wx-dim">Their games go first in Sports, and with alerts on, your phone hears before kickoff.</p>
        <div className="wx-top-tabs">
          {leagues.map((l) => (
            <button key={l.key} className={`wx-top-tab${l.key === league ? ' on' : ''}`} onClick={() => setLeague(l.key)}>
              {l.name}
            </button>
          ))}
        </div>
        {error && <p className="wx-error">{error}</p>}
        {!teams && !error && <p className="wx-dim">Loading the teams…</p>}
        <div className="wx-teampick-grid">
          {teams?.map((t) => (
            <button key={t.id} className={`wx-teampick-team${followed(t) ? ' on' : ''}`} onClick={() => void toggle(t)} aria-pressed={followed(t)}>
              {t.logo ? <img src={t.logo} alt="" loading="lazy" /> : <span className="wx-sports-abbr">{t.abbr}</span>}
              <span>{t.name}</span>
              {followed(t) && <Icon name="check" size={16} />}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}

// GuideSports: when ESPN can't be read, the games found in the TV guide.
function GuideSports({ data }: { data: { now: GuideGame[]; soon: GuideGame[]; big?: GuideGame } }) {
  const navigate = useNavigate()
  const open = (g: GuideGame) => navigate(liveHref(g.channel.id, 'all'))
  const when = (g: GuideGame) => (!g.show ? '' : g.onNow ? `Live · ends ${hhmm(g.show.stop)}` : dayTime(g.show.start))
  const big = data.big
  return (
    <section className="wx-sports" aria-label="Sports">
      <h2>
        <Icon name="activity" size={20} /> Sports
      </h2>
      {big && big.show && (
        <button className={`wx-sports-big glass${big.onNow ? ' live' : ''}`} onClick={() => open(big)}>
          <span className="wx-sports-tag">{big.onNow ? '● LIVE NOW' : 'NEXT BIG GAME'}</span>
          <b>{big.show.title}</b>
          <span className="wx-sports-meta">
            <ChannelLogo name={big.channel.name} src={big.channel.logo} />
            {[big.league, big.channel.name, when(big)].filter(Boolean).join(' · ')}
          </span>
        </button>
      )}
      {[
        ['On now', data.now] as const,
        ['Coming up', data.soon] as const,
      ].map(([label, list]) =>
        list.length === 0 ? null : (
          <div key={label}>
            <h3>{label}</h3>
            <div className="wx-sports-row">
              {list.slice(0, 20).map((g) => (
                <button key={`${g.channel.id}-${g.show?.start}`} className={`wx-sports-game${g.onNow ? ' live' : ''}`} onClick={() => open(g)}>
                  <b>{g.show?.title}</b>
                  <small>{[g.league, g.channel.name].filter(Boolean).join(' · ')}</small>
                  <small className="wx-sports-when">{when(g)}</small>
                </button>
              ))}
            </div>
          </div>
        ),
      )}
    </section>
  )
}
