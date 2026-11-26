import { useEffect, useState } from 'react'
import { api, type Card } from '../api'
import { focus, KEY } from '../focus'
import { cardKey, Icon, Poster, Spinner } from '../ui'
import { useHome } from './Home'

// Search: a box at the top (OK opens the TV's keyboard), what it finds as
// posters below, and before anything's typed, what's trending.
export function SearchScreen({ onPick }: { onPick: (c: Card) => void }) {
  const [q, setQ] = useState('')
  const [found, setFound] = useState<Card[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [problem, setProblem] = useState('')
  const { home } = useHome()

  useEffect(() => {
    const text = q.trim()
    if (text.length < 2) {
      setFound(null)
      setProblem('')
      return
    }
    let live = true
    setBusy(true)
    const t = window.setTimeout(() => {
      api
        .search(text)
        .then((a) => live && (setFound(a ?? []), setProblem('')))
        .catch((e) => live && setProblem(e instanceof Error ? e.message : String(e)))
        .finally(() => live && setBusy(false))
    }, 450)
    return () => {
      live = false
      window.clearTimeout(t)
    }
  }, [q])

  const trending = (home?.rows ?? []).filter((r) => r.key.startsWith('trending')).flatMap((r) => r.items)
  const seen = new Set<string>()
  const items = (found ?? trending)
    .filter((c) => {
      const k = cardKey(c)
      if (seen.has(k)) return false
      seen.add(k)
      return true
    })
    .slice(0, 42)
  const heading = found ? (found.length ? `Results for “${q.trim()}”` : `Nothing found for “${q.trim()}”`) : 'Trending now'

  return (
    <div className="screen search" data-vs="">
      <label className="search-box">
        <Icon name="search" />
        <input
          data-f=""
          data-first=""
          value={q}
          placeholder="Titles, people, genres"
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            // OK on the TV's keyboard: down to the results.
            if (e.keyCode === KEY.ok && items.length) {
              e.preventDefault()
              focus(document.querySelector<HTMLElement>('.grid [data-f]'))
            }
          }}
        />
        {busy && <Spinner />}
      </label>
      <h3>{heading}</h3>
      {problem && <p className="problem">{problem}</p>}
      <div className="grid" data-g="search">
        {items.map((c) => (
          <div className="grid-cell" data-row="" key={cardKey(c)}>
            <Poster c={c} onPick={onPick} />
            <span className="grid-name">{c.title}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
