import { useEffect, useState } from 'react'
import { api, type GuideSetup } from '../api'
import { useToast } from './Toast'

// Settings > Streaming > Live TV: extra TV guides (XMLTV addresses, plain or
// .gz) beside the provider's. A channel the provider has no guide for takes
// an extra guide's channel with the same name; any channel can be matched
// to a guide channel by hand from Live TV (the channel's Guide button).
export default function GuideSources() {
  const toast = useToast()
  const [setup, setSetup] = useState<GuideSetup | null>(null)
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .guideSetup()
      .then((g) => {
        setSetup(g)
        setText(g.sources.join('\n'))
      })
      .catch(() => undefined)
  }, [])
  // While a new guide loads, how it went shows when it's done.
  useEffect(() => {
    if (!setup?.loading) return
    const t = window.setTimeout(() => api.guideSetup().then(setSetup).catch(() => undefined), 4000)
    return () => window.clearTimeout(t)
  }, [setup])

  async function save() {
    setBusy(true)
    setError('')
    try {
      const g = await api.saveGuideSources(text.split('\n').map((l) => l.trim()).filter(Boolean))
      setSetup(g)
      toast.success('Saved. The guide loads again now; it takes a few minutes with big guides.')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  if (!setup) return null
  const matched = Object.keys(setup.matches ?? {}).length
  return (
    <div className="guide-sources">
      <h3 style={{ margin: '18px 0 6px' }}>Extra TV guides</h3>
      <p style={{ color: 'var(--text-dim)', marginTop: 0 }}>
        For channels your provider&apos;s guide leaves out or gets wrong: the address of an XMLTV guide (plain or .gz), one a line, up to five. A
        channel with no guide takes one with the same name; you can match any channel by hand from Live TV (its Guide button).
        {matched > 0 && ` ${matched} matched by hand.`}
      </p>
      <textarea
        rows={3}
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="https://example.com/guide.xml.gz"
        spellCheck={false}
        disabled={busy}
        style={{ width: '100%', fontFamily: 'monospace' }}
      />
      {(setup.status ?? []).map((s, i) => (
        <p key={i} style={{ margin: '4px 0', fontSize: '0.9em' }}>
          Guide {i + 1}: {s.problem ? <span className="error-text">{s.problem}</span> : `${s.channels.toLocaleString()} channels, ${s.matched} matched by name`}
        </p>
      ))}
      {setup.loading && <p style={{ color: 'var(--text-dim)', margin: '4px 0' }}>Loading the guides…</p>}
      <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
        <button className="primary" disabled={busy} onClick={() => void save()}>
          {busy ? 'Saving…' : 'Save guides'}
        </button>
      </div>
      {error && <p className="error-text">{error}</p>}
    </div>
  )
}
