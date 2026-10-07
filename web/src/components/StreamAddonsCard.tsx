import { useEffect, useState } from 'react'
import { api, type StreamAddon } from '../api'
import { useToast } from './Toast'

// Settings > Downloading: Stremio add-ons such as Comet on ElfHosted. Play asks
// them for streams first. Set the add-on up on its own page (Premiumize key,
// scrapers, sorting), copy its link, and paste it here.
export default function StreamAddonsCard() {
  const toast = useToast()
  const [list, setList] = useState<StreamAddon[] | null>(null)
  const [url, setUrl] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .streamAddons()
      .then(setList)
      .catch(() => setList([]))
  }, [])

  async function add() {
    setBusy(true)
    setError('')
    try {
      const l = await api.addStreamAddon(url.trim())
      setList(l)
      setUrl('')
      toast.success(`${l[l.length - 1]?.name ?? 'The add-on'} added. Play asks it first.`)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  async function remove(a: StreamAddon) {
    setBusy(true)
    try {
      setList(await api.removeStreamAddon(a.index))
      toast.success(`${a.name} removed.`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const has = (list?.length ?? 0) > 0
  return (
    <section className={`card service-card ${has ? 'is-included' : 'is-optional'}`}>
      <h2>
        Stream add-ons <span className="req-badge optional">Optional</span>{' '}
        {list !== null && <span className={`badge ${has ? 'downloaded' : 'missing'}`}>{has ? `${list.length} added` : 'none'}</span>}
      </h2>
      <p style={{ color: 'var(--text-dim)' }}>
        Stremio add-ons like <strong>Comet</strong>, Torrentio or MediaFusion find streams with their own scrapers. Play asks them first, in this order, and
        plays their best link; Cue&apos;s own search is the fallback. Set up the add-on with your Premiumize key so its links play straight away.
      </p>
      <ol style={{ margin: '8px 0 14px', paddingLeft: 20 }}>
        <li style={{ marginBottom: 4 }}>
          Open the add-on&apos;s configure page, for Comet{' '}
          <a href="https://comet.elfhosted.com/configure" target="_blank" rel="noreferrer">
            comet.elfhosted.com/configure
          </a>
          .
        </li>
        <li style={{ marginBottom: 4 }}>Pick Premiumize as the debrid service, paste your Premiumize key, and choose your scrapers, resolutions and sorting.</li>
        <li>Press its Copy link button (the link ends in /manifest.json), paste it here and press Add.</li>
      </ol>
      {has && (
        <ul style={{ listStyle: 'none', padding: 0, margin: '0 0 12px' }}>
          {list!.map((a) => (
            <li key={a.index} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '6px 0', borderBottom: '1px solid var(--border)' }}>
              <span style={{ fontWeight: 600 }}>
                {a.index + 1}. {a.name}
              </span>
              <span style={{ color: 'var(--text-dim)' }}>{a.host}</span>
              <button style={{ marginLeft: 'auto' }} disabled={busy} onClick={() => void remove(a)}>
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="grid-form">
        <label>
          Add-on link
          <input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://comet.elfhosted.com/…/manifest.json"
            autoComplete="off"
            spellCheck={false}
            disabled={busy}
          />
        </label>
        <div>
          <button className="primary" disabled={busy || url.trim() === ''} onClick={() => void add()}>
            {busy ? 'Checking…' : 'Add'}
          </button>
        </div>
        {error && <p className="error-text">{error}</p>}
      </div>
    </section>
  )
}
