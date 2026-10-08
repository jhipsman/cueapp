import { useEffect, useState } from 'react'
import { api, type IPTVSettings } from '../api'
import { useToast } from './Toast'

// Settings > Streaming: the IPTV provider for Live TV, an Xtream Codes login
// (the server address, username and password the provider sent). Cue reads
// its channels, logos and TV guide from it.
export default function IPTVCard() {
  const toast = useToast()
  const [st, setSt] = useState<IPTVSettings | null>(null)
  const [server, setServer] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .getIPTV()
      .then((s) => {
        setSt(s)
        setServer(s.server ?? '')
        setUsername(s.username ?? '')
      })
      .catch(() => setSt({ set: false }))
  }, [])

  async function save(remove = false) {
    setBusy(true)
    setError('')
    try {
      const s = await api.putIPTV(remove ? { server: '' } : { server, username, password })
      setSt(s)
      setPassword('')
      if (remove) {
        setServer('')
        setUsername('')
      }
      toast.success(s.set ? 'Connected. Live TV is in Watch; the guide fills in over the next few minutes.' : 'Live TV removed.')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const expires = st?.expires ? new Date(st.expires).toLocaleDateString() : ''

  return (
    <section className={`card service-card ${st?.set ? 'is-included' : 'is-optional'}`}>
      <h2>
        Live TV (IPTV) <span className="req-badge optional">Optional</span>{' '}
        {st && <span className={`badge ${st.set ? 'downloaded' : 'missing'}`}>{st.set ? 'connected' : 'not set up'}</span>}
      </h2>
      <p style={{ color: 'var(--text-dim)' }}>
        Your IPTV provider&apos;s channels, with their logos and a TV guide, under Live TV in Watch. Use the &quot;Xtream Codes&quot; login your
        provider sent: a server address (often with a port, like <code>http://line.example.com:8080</code>), a username and a password.
      </p>
      {st?.set && (
        <p style={{ margin: '0 0 12px' }}>
          {st.channels !== undefined && <>{st.channels.toLocaleString()} channels</>}
          {st.guideChannels !== undefined && st.guideChannels > 0 && <>, guide for {st.guideChannels.toLocaleString()}</>}
          {expires && <> · account runs until {expires}</>}
          {st.guideProblem && <span className="error-text"> · Guide: {st.guideProblem}</span>}
        </p>
      )}
      <div className="grid-form">
        <label>
          Server address
          <input value={server} onChange={(e) => setServer(e.target.value)} placeholder="http://line.example.com:8080" autoComplete="off" spellCheck={false} disabled={busy} />
        </label>
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="off" spellCheck={false} disabled={busy} />
        </label>
        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={st?.set ? 'Saved (type to change it)' : ''}
            autoComplete="new-password"
            disabled={busy}
          />
        </label>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
          <button className="primary" disabled={busy || !server.trim() || !username.trim() || (!st?.set && !password)} onClick={() => void save()}>
            {busy ? 'Checking…' : 'Save'}
          </button>
          {st?.set && (
            <button disabled={busy} onClick={() => void save(true)}>
              Remove
            </button>
          )}
        </div>
        {error && <p className="error-text">{error}</p>}
      </div>
    </section>
  )
}
