import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, type WatchCard } from '../api'
import { useDocumentTitle } from '../documentTitle'
import { Card } from './parts'

// Search: every movie and show TMDB knows, best match first. The search box
// is in the top bar.
export default function SearchPage() {
  const [params] = useSearchParams()
  const q = params.get('q') ?? ''
  const [results, setResults] = useState<{ q: string; items: WatchCard[] } | null>(null)
  const [error, setError] = useState('')
  useDocumentTitle(q ? `Search: ${q}` : 'Search')

  useEffect(() => {
    if (!q.trim()) return
    let live = true
    api
      .watchSearch(q)
      .then((items) => {
        if (live) {
          setResults({ q, items })
          setError('')
        }
      })
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [q])

  const items = results?.items ?? []
  return (
    <div className="wx-page">
      {error && <p className="wx-error">{error}</p>}
      {results && results.q === q && items.length === 0 && <p className="wx-dim">Nothing found for “{q}”.</p>}
      {items.length > 0 && (
        <div className="wx-grid">
          {items.map((c) => (
            <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
          ))}
        </div>
      )}
    </div>
  )
}
