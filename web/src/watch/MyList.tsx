import { useEffect, useState } from 'react'
import { api, type WatchCard } from '../api'
import { useDocumentTitle } from '../documentTitle'
import { Card } from './parts'

// My List: the titles saved with "My List" on their page.
export default function MyList() {
  const [items, setItems] = useState<WatchCard[] | null>(null)
  useDocumentTitle('My List')
  useEffect(() => {
    api
      .watchHome()
      .then((h) => setItems(h.rows.find((r) => r.key === 'my-list')?.items ?? []))
      .catch(() => setItems([]))
  }, [])
  return (
    <div className="wx-page">
      <h2 style={{ marginTop: 0 }}>My List</h2>
      {items === null && <div className="wx-skeleton" style={{ height: 260 }} />}
      {items?.length === 0 && <p className="wx-dim">Nothing saved yet. Open a title and press + My List.</p>}
      {items && items.length > 0 && (
        <div className="wx-grid">
          {items.map((c) => (
            <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
          ))}
        </div>
      )}
    </div>
  )
}
