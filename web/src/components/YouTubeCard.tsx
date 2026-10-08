import { useEffect, useState } from 'react'
import { api } from '../api'
import { useToast } from './Toast'

// Settings > Streaming: an optional YouTube Data API key. When the add-ons
// and torrent sites have nothing (old children's TV, say), Play also looks
// on the Internet Archive (no key needed) and on YouTube, whose videos play
// in YouTube's own player.
export default function YouTubeCard() {
  const toast = useToast()
  const [set, setSet] = useState<boolean | null>(null)
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .getYouTube()
      .then((s) => setSet(s.set))
      .catch(() => setSet(false))
  }, [])

  async function save(value: string) {
    setBusy(true)
    setError('')
    try {
      const s = await api.putYouTube(value.trim())
      setSet(s.set)
      setKey('')
      toast.success(s.set ? 'YouTube connected: Play looks there when nothing else is found.' : 'YouTube key removed.')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className={`card service-card ${set ? 'is-included' : 'is-optional'}`}>
      <h2>
        Hard-to-find shows (YouTube) <span className="req-badge optional">Optional</span>{' '}
        {set !== null && <span className={`badge ${set ? 'downloaded' : 'missing'}`}>{set ? 'connected' : 'not set up'}</span>}
      </h2>
      <p style={{ color: 'var(--text-dim)' }}>
        When nothing else is found, Play looks on the Internet Archive (always on) and, with a free YouTube key, for full
        episodes and films on YouTube. Those play in YouTube&apos;s own player.
      </p>
      <ol style={{ margin: '8px 0 14px', paddingLeft: 20 }}>
        <li style={{ marginBottom: 4 }}>
          Open{' '}
          <a href="https://console.cloud.google.com/apis/library/youtube.googleapis.com" target="_blank" rel="noreferrer">
            YouTube Data API v3 in Google Cloud
          </a>
          , sign in, make a project if asked, and press Enable.
        </li>
        <li style={{ marginBottom: 4 }}>
          Go to{' '}
          <a href="https://console.cloud.google.com/apis/credentials" target="_blank" rel="noreferrer">
            Credentials
          </a>
          , press Create credentials, then API key.
        </li>
        <li>Paste the key (it starts with AIza) here and press Save. It&apos;s free: about 100 searches a day, and each title is searched once.</li>
      </ol>
      <div className="grid-form">
        <label>
          {set ? 'Replace the key' : 'YouTube API key'}
          <input value={key} onChange={(e) => setKey(e.target.value)} placeholder="AIza…" autoComplete="off" spellCheck={false} disabled={busy} />
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
