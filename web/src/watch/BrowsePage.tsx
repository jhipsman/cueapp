import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, type BrowseHome, type WatchCard } from '../api'
import Icon from '../components/Icon'
import { Card, Row } from './parts'

// Browse: Coming soon and the Top 10, a page per streaming service (its
// Top 10, what's new, its originals, its best and its genres), collections
// across services, and the big film series in order.

let homeCache: BrowseHome | null = null

export default function BrowsePage() {
  const [home, setHome] = useState<BrowseHome | null>(homeCache)
  const [problem, setProblem] = useState('')
  useEffect(() => {
    api
      .browse()
      .then((h) => {
        homeCache = h
        setHome(h)
      })
      .catch((e) => setProblem(e instanceof Error ? e.message : String(e)))
  }, [])
  return (
    <div className="wx-page wx-browse">
      <h2 style={{ marginTop: 0 }}>Browse</h2>
      <div className="wx-browse-quick">
        <Link to="/watch/coming-soon" className="wx-browse-quick-tile soon">
          <Icon name="calendar" size={22} />
          <b>Coming soon</b>
          <span>New films and episodes on the way</span>
        </Link>
        <Link to="/watch/top" className="wx-browse-quick-tile top">
          <Icon name="star" size={22} />
          <b>Top 10</b>
          <span>By service, genre, decade and country</span>
        </Link>
      </div>
      {!home && <p className="wx-dim">{problem || 'Loading…'}</p>}
      {home && home.services.length > 0 && (
        <section className="wx-browse-section">
          <h3>Streaming services</h3>
          <div className="wx-network-row">
            {home.services.map((s) => (
              <Link key={s.key} to={`/watch/browse/service/${s.key}`} className={`wx-network ${s.key}${s.logo ? ' has-logo' : ''}`} title={s.name}>
                {s.logo && <img src={s.logo} alt="" />}
                <span>{s.name}</span>
              </Link>
            ))}
          </div>
        </section>
      )}
      {home && home.collections.length > 0 && (
        <section className="wx-browse-section">
          <h3>Collections</h3>
          <div className="wx-browse-cols">
            {home.collections.map((c) => (
              <Link key={c.key} to={`/watch/browse/collection/${c.key}`} className={`wx-browse-col ${c.key}`}>
                <b>{c.title}</b>
                <span>{c.blurb}</span>
              </Link>
            ))}
          </div>
        </section>
      )}
      {home && home.franchises.length > 0 && (
        <section className="wx-browse-section">
          <h3>Film series</h3>
          <div className="wx-browse-series">
            {home.franchises.map((f) => (
              <Link key={f.id} to={`/watch/browse/franchise/${f.id}`} className="wx-browse-fr" title={f.name}>
                {f.poster && <img src={f.poster} alt="" loading="lazy" />}
                <span>{f.name}</span>
              </Link>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

// A streaming service's page: its rows, as on Home.
export function BrowseServicePage() {
  const { key = '' } = useParams()
  const [page, setPage] = useState<Awaited<ReturnType<typeof api.browseService>> | null>(null)
  const [problem, setProblem] = useState('')
  useEffect(() => {
    let live = true
    api
      .browseService(key)
      .then((p) => live && setPage(p))
      .catch((e) => live && setProblem(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [key])
  return (
    <div className="wx-browse-svc-page">
      <div className="wx-page wx-browse-head">
        <Link to="/watch/browse" className="wx-back-link">
          <Icon name="chevron-left" size={16} /> Browse
        </Link>
        <h2>
          {page?.logo && <img src={page.logo} alt="" />}
          {page?.name ?? ''}
        </h2>
      </div>
      {!page && <p className="wx-page wx-dim">{problem || 'Loading…'}</p>}
      {page?.rows.length === 0 && <p className="wx-page wx-dim">Nothing to show for {page.name} right now.</p>}
      {page?.rows.map((r) => <Row key={r.key} rowKey={r.key} title={r.title} items={r.items} />)}
    </div>
  )
}

// A collection, or a film series in order: a grid of its titles.
export function BrowseListPage({ kind }: { kind: 'collection' | 'franchise' }) {
  const { key = '', id = '' } = useParams()
  const [page, setPage] = useState<{ title: string; blurb: string; items: WatchCard[] } | null>(null)
  const [problem, setProblem] = useState('')
  useEffect(() => {
    let live = true
    const load =
      kind === 'collection'
        ? api.browseCollection(key).then((c) => ({ title: c.title, blurb: c.blurb, items: c.items }))
        : api.browseFranchise(Number(id)).then((f) => ({ title: f.name, blurb: 'In the order they came out.', items: f.items }))
    load.then((p) => live && setPage(p)).catch((e) => live && setProblem(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [kind, key, id])
  return (
    <div className="wx-page">
      <Link to="/watch/browse" className="wx-back-link">
        <Icon name="chevron-left" size={16} /> Browse
      </Link>
      <h2 style={{ marginBottom: 4 }}>{page?.title ?? ''}</h2>
      {page?.blurb && <p className="wx-dim" style={{ marginTop: 0 }}>{page.blurb}</p>}
      {!page && <p className="wx-dim">{problem || 'Loading…'}</p>}
      {page?.items.length === 0 && <p className="wx-dim">Nothing here right now.</p>}
      {page && page.items.length > 0 && (
        <div className="wx-grid">
          {page.items.map((c) => (
            <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
          ))}
        </div>
      )}
    </div>
  )
}
