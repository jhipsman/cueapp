import { useEffect, useState } from 'react'
import { api } from '../api'
import { useToast } from './Toast'

// Settings > Info, lists and subtitles: an optional OMDb key, for IMDb and
// Rotten Tomatoes ratings in Watch. TMDB has no IMDb ratings; without a key
// Watch shows TMDB's own score.
export default function OMDbCard() {
  const toast = useToast()
  const [set, setSet] = useState<boolean | null>(null)
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .getOMDb()
      .then((s) => setSet(s.set))
      .catch(() => setSet(false))
  }, [])

  async function save(value: string) {
    setBusy(true)
    setError('')
    try {
      const s = await api.putOMDb(value.trim())
      setSet(s.set)
      setKey('')
      toast.success(s.set ? 'OMDb connected: Watch now shows IMDb ratings.' : 'OMDb key removed. Watch shows TMDB scores.')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className={`card service-card ${set ? 'is-included' : 'is-optional'}`}>
      <h2>
        IMDb ratings (OMDb) <span className="req-badge optional">Optional</span>{' '}
        {set !== null && <span className={`badge ${set ? 'downloaded' : 'missing'}`}>{set ? 'connected' : 'not set up'}</span>}
      </h2>
      <p style={{ color: 'var(--text-dim)' }}>
        Watch shows IMDb and Rotten Tomatoes ratings when you add a free OMDb key. Without one it shows TMDB&apos;s own score.
      </p>
      <ol style={{ margin: '8px 0 14px', paddingLeft: 20 }}>
        <li style={{ marginBottom: 4 }}>
          Get a free key at{' '}
          <a href="https://www.omdbapi.com/apikey.aspx" target="_blank" rel="noreferrer">
            omdbapi.com
          </a>{' '}
          (1,000 lookups a day is plenty: each title is looked up once a day).
        </li>
        <li style={{ marginBottom: 4 }}>Click the activation link in the email OMDb sends.</li>
        <li>Paste the key here and press Save.</li>
      </ol>
      <div className="grid-form">
        <label>
          {set ? 'Replace the key' : 'OMDb API key'}
          <input value={key} onChange={(e) => setKey(e.target.value)} placeholder="a1b2c3d4" autoComplete="off" spellCheck={false} disabled={busy} />
        </label>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
          <button className="primary" disabled={busy || key.trim() === ''} onClick={() => void save(key)}>
            {busy ? 'Checking…' : 'Save'}
          </button>
          {set && (
            <button disabled={busy} onClick={() => void save('')}>
              Remove the key
            </button>
          )}
        </div>
        {error && <p className="error-text">{error}</p>}
      </div>
    </section>
  )
}
