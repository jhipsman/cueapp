import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import Icon, { type IconName } from '../components/Icon'
import { useProfiles } from './profiles'
import { onTV } from './tv'

// The menu that names everything: every section and tool in Cue in one list,
// so nothing hides behind an icon. A sheet from the bottom on a phone (the
// More tab), a panel under the top bar's More on a computer.

type Item = { label: string; to: string; icon: IconName; hint?: string }

export function useAllItems(): { group: string; items: Item[] }[] {
  const { active, data } = useProfiles()
  const liveTV = data?.liveTV ?? false
  return [
    {
      group: 'Watch',
      items: [
        { label: 'Home', to: '/watch', icon: 'grid' },
        { label: 'Shows', to: '/watch/shows', icon: 'tv' },
        { label: 'Movies', to: '/watch/movies', icon: 'film' },
        { label: 'My List', to: '/watch/list', icon: 'bookmark' },
        { label: 'Coming soon', to: '/watch/coming-soon', icon: 'calendar', hint: 'New episodes and films on their way' },
        { label: 'Top 10', to: '/watch/top', icon: 'star', hint: 'By service, country, genre or decade' },
        { label: 'Sports', to: '/watch/sports', icon: 'activity', hint: 'Live scores, the next big game, your teams' },
        ...(liveTV ? [{ label: 'Live TV', to: '/watch/live', icon: 'tv' as IconName }] : []),
      ],
    },
    {
      group: 'Find',
      items: [
        { label: 'Search', to: '/watch/search', icon: 'search', hint: 'Titles, people and genres' },
        { label: 'Ask Cue', to: '/watch/ask', icon: 'sparkles', hint: '“Something funny for tonight”' },
      ],
    },
    {
      group: 'You',
      items: [
        { label: 'Your year', to: '/watch/year', icon: 'clock' },
        { label: 'Reports', to: '/watch/reports', icon: 'warning', hint: 'Problems reported and the answers' },
        ...(onTV() ? [] : [{ label: 'TV remote', to: '/watch/tv-remote', icon: 'monitor' as IconName }]),
        ...((data?.profiles.length ?? 0) > 1 ? [{ label: 'Switch profile', to: '/watch/who', icon: 'users' as IconName }] : []),
        ...(active?.main ? [{ label: 'Manage profiles', to: '/watch/profiles', icon: 'user' as IconName }] : []),
        { label: 'Settings', to: '/watch/settings', icon: 'sliders' },
      ],
    },
  ]
}

export default function AllMenu({ onClose, sheet }: { onClose: () => void; sheet?: boolean }) {
  const navigate = useNavigate()
  const groups = useAllItems()
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    const onDown = (e: PointerEvent) => !box.current?.contains(e.target as Node) && onClose()
    document.addEventListener('keydown', onKey)
    const t = window.setTimeout(() => document.addEventListener('pointerdown', onDown), 0)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onDown)
      window.clearTimeout(t)
    }
  }, [onClose])
  return (
    <>
      {sheet && <div className="wx-allmenu-shade" aria-hidden="true" />}
      <div className={`wx-allmenu glass${sheet ? ' sheet' : ''}`} ref={box} role="dialog" aria-label="Everything in Cue">
        {sheet && <span className="wx-allmenu-grip" aria-hidden="true" />}
        {groups.map((g) => (
          <section key={g.group}>
            <h3>{g.group}</h3>
            <div className="wx-allmenu-grid">
              {g.items.map((it) => (
                <button
                  key={it.to}
                  onClick={() => {
                    onClose()
                    navigate(it.to)
                  }}
                >
                  <span className="wx-allmenu-ic">
                    <Icon name={it.icon} size={18} />
                  </span>
                  <span>
                    <b>{it.label}</b>
                    {it.hint && <small>{it.hint}</small>}
                  </span>
                </button>
              ))}
            </div>
          </section>
        ))}
      </div>
    </>
  )
}
