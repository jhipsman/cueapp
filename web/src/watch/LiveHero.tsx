import { useEffect, useMemo, useState } from 'react'
import { api, type SportsAnswer, type SportsGame, type SportsTeam } from '../api'
import Icon from '../components/Icon'

// Live TV's hero: the game that matters most right now, as an event. Both
// teams large (logo, city, name, record) over their colors, a countdown to
// the start (or the score, live), where it's played, and the channel it's on.
// Beside it, Featured: the next few games; the hero moves through them
// every few seconds (each card's bar shows how long until the next), and
// picking one shows it.

const ROTATE_MS = 8000

// leagueLogo is ESPN's mark for the big leagues (none for the rest).
function leagueLogo(league: string): string | undefined {
  const known = ['nfl', 'nba', 'nhl', 'mlb', 'wnba', 'mls']
  return known.includes(league) ? `https://a.espncdn.com/i/teamlogos/leagues/500/${league}.png` : undefined
}

// split is a team's city and nickname: "Los Angeles" and "Kings".
function split(t: SportsTeam): [string, string] {
  const short = t.short || t.name
  const city = t.name.endsWith(short) ? t.name.slice(0, t.name.length - short.length).trim() : ''
  return [city, short]
}

// color is a team's color as CSS, darkened for a background (a neutral gray
// when ESPN gives none).
function color(t: SportsTeam, alpha: number): string {
  const hex = /^[0-9a-f]{6}$/i.test(t.color ?? '') ? t.color! : '3a3a3e'
  const n = parseInt(hex, 16)
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`
}

// until is "47m", "2h 14m", or "" when it has started or is more than a day off.
function until(iso: string, now: number): string {
  const m = Math.round((Date.parse(iso) - now) / 60000)
  if (m <= 0 || m > 24 * 60) return ''
  return m < 60 ? `${m}m` : `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, '0')}m`
}

function dayTime(iso: string): string {
  const d = new Date(iso)
  const today = new Date()
  const tomorrow = new Date(Date.now() + 86_400_000)
  const day =
    d.toDateString() === today.toDateString()
      ? 'Today'
      : d.toDateString() === tomorrow.toDateString()
        ? 'Tomorrow'
        : d.toLocaleDateString(undefined, { weekday: 'short' })
  return `${day} · ${d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })}`
}

function shortTime(iso: string): string {
  return new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

function Side({ team, align }: { team: SportsTeam; align: 'left' | 'right' }) {
  const [city, name] = split(team)
  return (
    <div className={`wx-lh-team ${align}`}>
      {team.logo ? <img src={team.logo} alt="" /> : <span className="wx-lh-abbr">{team.abbr}</span>}
      {city && <small>{city}</small>}
      <b>{name}</b>
      {team.record && <span>{team.record}</span>}
    </div>
  )
}

// onFind searches Live TV for a game no channel was matched to (lists
// carry most games somewhere: an event channel, a backup).
export default function LiveHero({ onWatch, onFind }: { onWatch: (channelId: string) => void; onFind: (query: string) => void }) {
  const [data, setData] = useState<SportsAnswer | null>(null)
  const [pickedId, setPickedId] = useState<string | null>(null)
  const [paused, setPaused] = useState(false)
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    let live = true
    const load = () =>
      api
        .liveSports()
        .then((d) => live && setData(d))
        .catch(() => undefined)
    void load()
    const t = window.setInterval(load, 60_000)
    const c = window.setInterval(() => setNow(Date.now()), 30_000)
    return () => {
      live = false
      window.clearInterval(t)
      window.clearInterval(c)
    }
  }, [])

  // The biggest game first, then what's on now, then what's next: four.
  const games = useMemo(() => {
    if (!data || data.source !== 'espn') return []
    const all = [data.big, ...data.now, ...data.soon].filter((g): g is SportsGame => !!g)
    const seen = new Set<string>()
    return all.filter((g) => (seen.has(g.league + g.id) ? false : (seen.add(g.league + g.id), true))).slice(0, 4)
  }, [data])

  // Move through them on its own, unless the pointer rests on the hero.
  const index = Math.max(0, games.findIndex((g) => g.league + g.id === pickedId))
  useEffect(() => {
    if (games.length < 2 || paused) return
    const t = window.setTimeout(() => {
      const next = games[(index + 1) % games.length]
      setPickedId(next.league + next.id)
    }, ROTATE_MS)
    return () => window.clearTimeout(t)
  }, [games, index, paused])

  if (games.length === 0) return null
  const g = games[index]
  const live = g.state === 'in'
  const countdown = until(g.start, now)
  const where = g.channel?.name || g.streaming || g.networks?.[0]
  const mark = leagueLogo(g.league)

  return (
    <section className="wx-lh" aria-label="Sports" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
      <button
        className="wx-lh-hero"
        key={g.league + g.id}
        style={{
          background: `radial-gradient(120% 140% at 0% 50%, ${color(g.away, 0.55)} 0%, transparent 55%), radial-gradient(120% 140% at 100% 50%, ${color(g.home, 0.55)} 0%, transparent 55%), linear-gradient(180deg, #1a1a1c, #0e0e10)`,
        }}
        onClick={() => (g.channel ? onWatch(g.channel.id) : onFind(g.home.short || g.home.name))}
        title={g.channel ? `Watch on ${g.channel.name}` : `Find ${g.away.short} at ${g.home.short} on Live TV`}
      >
        <div className="wx-lh-match">
          <Side team={g.away} align="left" />
          <div className="wx-lh-mid">
            {mark ? <img className="wx-lh-league" src={mark} alt={g.leagueName} /> : <span className="wx-lh-league-name">{g.leagueName}</span>}
            {live ? (
              <>
                <span className="wx-lh-kicker live">
                  <i /> LIVE
                </span>
                <b className="wx-lh-big">
                  {g.away.score ?? 0} – {g.home.score ?? 0}
                </b>
                <span className="wx-lh-when">{g.detail}</span>
              </>
            ) : countdown ? (
              <>
                <span className="wx-lh-kicker">STARTS IN</span>
                <b className="wx-lh-big">{countdown}</b>
                <span className="wx-lh-when">{dayTime(g.start)}</span>
              </>
            ) : (
              <>
                <span className="wx-lh-kicker">{g.note || g.leagueName}</span>
                <b className="wx-lh-big small">{dayTime(g.start)}</b>
              </>
            )}
          </div>
          <Side team={g.home} align="right" />
        </div>
        <div className="wx-lh-foot">
          {g.venue && <span>{g.venue}</span>}
          {(where || !g.channel) && (
            <span className="wx-lh-on">
              <Icon name={g.channel ? 'play' : 'search'} size={12} />
              {g.channel ? `Watch on ${g.channel.name}` : `Find it on Live TV${where ? ` · on ${where}` : ''}`}
            </span>
          )}
        </div>
      </button>

      {games.length > 1 && (
        <aside className="wx-lh-featured" aria-label="Featured games">
          <div className="wx-lh-featured-head">
            <b>Featured</b>
            <small>
              {index + 1} / {games.length}
            </small>
          </div>
          {games.map((f, i) => (
            <button
              key={f.league + f.id}
              className={`wx-lh-card${i === index ? ' on' : ''}`}
              onClick={() => setPickedId(f.league + f.id)}
              aria-pressed={i === index}
            >
              <small className="wx-lh-card-league">{f.leagueName}</small>
              <span className="wx-lh-card-row">
                <span className="wx-lh-card-team">
                  {f.away.logo && <img src={f.away.logo} alt="" />}
                  {f.away.abbr || f.away.short}
                </span>
                <span className="wx-lh-card-time">
                  {f.state === 'in' ? (
                    <>
                      <b>
                        {f.away.score} – {f.home.score}
                      </b>
                      <small className="live">{f.detail}</small>
                    </>
                  ) : (
                    <>
                      <b>{shortTime(f.start)}</b>
                      <small>{dayTime(f.start).split(' · ')[0]}</small>
                    </>
                  )}
                </span>
                <span className="wx-lh-card-team right">
                  {f.home.abbr || f.home.short}
                  {f.home.logo && <img src={f.home.logo} alt="" />}
                </span>
              </span>
              {i === index && games.length > 1 && <span className={`wx-lh-card-bar${paused ? ' paused' : ''}`} key={g.id + String(paused)} />}
            </button>
          ))}
        </aside>
      )}
    </section>
  )
}
