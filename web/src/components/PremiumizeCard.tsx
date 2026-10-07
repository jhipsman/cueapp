import { useEffect, useState } from 'react'
import { api, type PremiumizeState, type PremiumizeUseFor } from '../api'
import { useToast } from './Toast'

const useForChoices: [PremiumizeUseFor, string][] = [
  ['torrents', 'Torrents'],
  ['usenet', 'Usenet'],
  ['both', 'Both'],
]

// Settings > Downloading: an optional Premiumize.me account. With its API key
// saved, the releases chosen below are downloaded by Premiumize on its own
// servers, then fetched from there over HTTPS, instead of by the built-in
// torrent client or Usenet downloader.
export default function PremiumizeCard({ onChange }: { onChange?: () => void }) {
  const toast = useToast()
  const [state, setState] = useState<PremiumizeState | null>(null)
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .getPremiumize()
      .then(setState)
      .catch(() => setState({ set: false, useFor: 'torrents' }))
  }, [])

  async function save(body: { apiKey?: string; useFor?: PremiumizeUseFor }, done: (s: PremiumizeState) => string) {
    setBusy(true)
    setError('')
    try {
      const s = await api.putPremiumize(body)
      setState(s)
      setKey('')
      toast.success(done(s))
      onChange?.()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const set = state?.set ?? false
  const until = state?.premiumUntil ? new Date(state.premiumUntil * 1000).toLocaleDateString() : ''
  const what = (u: PremiumizeUseFor) => (u === 'both' ? 'torrents and Usenet' : u === 'usenet' ? 'Usenet' : 'torrents')

  return (
    <section className={`card service-card ${set ? 'is-included' : 'is-optional'}`}>
      <h2>
        Premiumize <span className="req-badge optional">Optional</span>{' '}
        {state && (
          <span className={`badge ${set && !state.error ? 'downloaded' : 'missing'}`}>{set ? (state.error ? 'check the key' : 'connected') : 'not set up'}</span>
        )}
      </h2>
      <p style={{ color: 'var(--text-dim)' }}>
        Premiumize.me downloads torrents and NZBs on its own servers, often instantly when it already has them. Cue then fetches the finished files from
        Premiumize over HTTPS. For torrents that means no peers connect to your server, no seeding and no VPN needed.
      </p>
      {set && state && (
        <p>
          {state.error ? (
            <span className="error-text">{state.error}</span>
          ) : (
            <>
              Connected{state.customerId ? ` (customer ${state.customerId})` : ''}.{until && ` Premium until ${until}.`}
              {state.limitUsed !== undefined && ` Fair-use limit used: ${Math.round(state.limitUsed * 100)}%.`}
            </>
          )}
        </p>
      )}
      {!set && (
        <ol style={{ margin: '8px 0 14px', paddingLeft: 20 }}>
          <li style={{ marginBottom: 4 }}>
            Sign in at{' '}
            <a href="https://www.premiumize.me/account" target="_blank" rel="noreferrer">
              premiumize.me/account
            </a>
            .
          </li>
          <li style={{ marginBottom: 4 }}>Copy the API key shown there.</li>
          <li>Paste it here and press Save. Cue checks it with Premiumize first.</li>
        </ol>
      )}
      <div className="grid-form">
        <label>
          {set ? 'Replace the API key' : 'Premiumize API key'}
          <input value={key} onChange={(e) => setKey(e.target.value)} placeholder="API key" autoComplete="off" spellCheck={false} disabled={busy} />
        </label>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
          <button
            className="primary"
            disabled={busy || key.trim() === ''}
            onClick={() => void save({ apiKey: key.trim(), useFor: state?.useFor }, (s) => `Premiumize connected. It now downloads your ${what(s.useFor)}.`)}
          >
            {busy ? 'Checking…' : 'Save'}
          </button>
          {set && (
            <button disabled={busy} onClick={() => void save({ apiKey: '' }, () => 'Premiumize removed. The built-in downloaders are used again.')}>
              Remove the key
            </button>
          )}
        </div>
        {set && state && (
          <div>
            <p style={{ color: 'var(--text-dim)', margin: '8px 0 6px' }}>Use Premiumize for</p>
            <div className="seg" role="radiogroup" aria-label="Use Premiumize for">
              {useForChoices.map(([value, label]) => (
                <button
                  key={value}
                  role="radio"
                  aria-checked={state.useFor === value}
                  className={state.useFor === value ? 'active' : ''}
                  disabled={busy}
                  onClick={() => void save({ useFor: value }, (s) => `Saved: Premiumize downloads your ${what(s.useFor)}.`)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
        )}
        {error && <p className="error-text">{error}</p>}
      </div>
    </section>
  )
}
