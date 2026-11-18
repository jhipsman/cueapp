import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type LiveHit } from '../api'
import Icon from '../components/Icon'
import ChannelLogo from './ChannelLogo'
import { liveHref } from './LivePage'

// Sports on Live TV: the next big game on top, then games on now and coming
// up in the next day (found in the TV guide).

type Game = LiveHit & { league?: string }

const hhmm = (s: string) => new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
function when(g: Game): string {
  if (!g.show) return ''
  if (g.onNow) return `Live · ends ${hhmm(g.show.stop)}`
  const start = new Date(g.show.start)
  const today = new Date()
  const tomorrow = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 1)
  const day = start.toDateString() === today.toDateString() ? 'Today' : start.toDateString() === tomorrow.toDateString() ? 'Tomorrow' : start.toLocaleDateString([], { weekday: 'long' })
  return `${day} ${hhmm(g.show.start)}`
}

export default function SportsHub() {
  const navigate = useNavigate()
  const [data, setData] = useState<{ now: Game[]; soon: Game[]; big?: Game } | null>(null)
  useEffect(() => {
    let live = true
    const load = () =>
      api
        .liveSports()
        .then((d) => live && setData(d))
        .catch(() => undefined)
    void load()
    const t = window.setInterval(load, 60_000)
    return () => {
      live = false
      window.clearInterval(t)
    }
  }, [])
  if (!data || (data.now.length === 0 && data.soon.length === 0)) return null
  const open = (g: Game) => navigate(liveHref(g.channel.id, 'all'))
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
          <span className="wx-btn play small">
            <Icon name="play" size={16} /> {big.onNow ? 'Watch' : 'Go to channel'}
          </span>
        </button>
      )}
      {[
        ['On now', data.now.filter((g) => g !== big && g.show?.start !== big?.show?.start)] as const,
        ['Coming up', data.soon.filter((g) => !(g.show?.title === big?.show?.title && g.show?.start === big?.show?.start))] as const,
      ].map(([label, list]) =>
        list.length === 0 ? null : (
          <div key={label}>
            <h3>{label}</h3>
            <div className="wx-sports-row">
              {list.slice(0, 20).map((g) => (
                <button key={`${g.channel.id}-${g.show?.start}`} className={`wx-sports-game${g.onNow ? ' live' : ''}`} onClick={() => open(g)}>
                  <span className="wx-sports-logo">
                    <ChannelLogo name={g.channel.name} src={g.channel.logo} />
                  </span>
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
