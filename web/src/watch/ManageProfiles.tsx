import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { api, setProfileToken, type WatchProfile } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { AVATAR_COLORS, Avatar, useProfiles } from './profiles'

// Manage profiles: add, rename, recolor, lock with a PIN, remove. Only the
// main profile can (the server checks too), so anyone else is sent to open it.
export default function ManageProfiles() {
  const navigate = useNavigate()
  const { data, active, refresh } = useProfiles()
  const [editing, setEditing] = useState<WatchProfile | 'new' | null>(null)
  useDocumentTitle('Manage profiles')

  if (!data) return null
  const multi = data.profiles.length > 1
  if (multi && !active?.main) return <Navigate to="/watch/who?main=1&then=/watch/profiles" replace />

  return (
    <div className="wx-who">
      <h1>Manage profiles</h1>
      {editing ? (
        <ProfileEditor
          profile={editing === 'new' ? null : editing}
          colors={data.avatars}
          onDone={async () => {
            setEditing(null)
            await refresh()
          }}
        />
      ) : (
        <>
          <div className="wx-who-grid">
            {data.profiles.map((p) => (
              <button key={p.id} className="wx-who-tile editing" onClick={() => setEditing(p)}>
                <span className="wx-who-edit">
                  <Avatar profile={p} size={120} />
                  <span className="wx-who-pencil">
                    <Icon name="sliders" size={28} />
                  </span>
                </span>
                <span>
                  {p.name} {p.main && <small className="wx-dim">(main)</small>}
                </span>
              </button>
            ))}
            {data.profiles.length < data.max && (
              <button className="wx-who-tile" onClick={() => setEditing('new')}>
                <span className="wx-avatar wx-avatar-add" style={{ width: 120, height: 120 }}>
                  <Icon name="plus" size={48} />
                </span>
                <span>Add profile</span>
              </button>
            )}
          </div>
          <button className="wx-btn play wx-who-manage" onClick={() => navigate('/watch')}>
            Done
          </button>
        </>
      )}
    </div>
  )
}

function ProfileEditor({ profile, colors, onDone }: { profile: WatchProfile | null; colors: string[]; onDone: () => Promise<void> }) {
  const [name, setName] = useState(profile?.name ?? '')
  const [avatar, setAvatar] = useState(profile?.avatar ?? colors[1] ?? 'red')
  const [pin, setPin] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function run(f: () => Promise<unknown>) {
    setBusy(true)
    setError('')
    try {
      await f()
      await onDone()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  async function save() {
    if (!profile) {
      await api.addWatchProfile(name, avatar)
      return
    }
    const body: { name: string; avatar: string; pin?: string } = { name, avatar }
    if (pin) body.pin = pin
    const res = await api.updateWatchProfile(profile.id, body)
    if (res.token && profile.main) setProfileToken(res.token) // a new PIN signed this device out of it
  }

  async function clearPin() {
    if (!profile) return
    const res = await api.updateWatchProfile(profile.id, { pin: '' })
    if (res.token && profile.main) setProfileToken(res.token)
  }

  return (
    <div className="wx-edit">
      <Avatar profile={{ name: name || '?', avatar }} size={120} />
      <label>
        Name
        <input value={name} maxLength={30} onChange={(e) => setName(e.target.value)} autoFocus placeholder="Name" />
      </label>
      <div>
        <div className="wx-dim" style={{ marginBottom: 8 }}>
          Color
        </div>
        <div className="wx-colors">
          {colors.map((c) => (
            <button
              key={c}
              className={c === avatar ? 'on' : ''}
              style={{ background: AVATAR_COLORS[c] }}
              onClick={() => setAvatar(c)}
              aria-label={c}
              aria-pressed={c === avatar}
            />
          ))}
        </div>
      </div>
      {profile && (
        <label>
          {profile.hasPin ? 'New PIN (4 digits)' : 'Lock with a PIN (4 digits, optional)'}
          <input
            value={pin}
            inputMode="numeric"
            maxLength={4}
            onChange={(e) => setPin(e.target.value.replace(/\D/g, '').slice(0, 4))}
            placeholder="••••"
            autoComplete="off"
          />
        </label>
      )}
      {error && <p className="wx-error">{error}</p>}
      <div className="wx-actions">
        <button className="wx-btn play" disabled={busy || !name.trim() || (pin !== '' && pin.length !== 4)} onClick={() => void run(save)}>
          Save
        </button>
        <button className="wx-btn" disabled={busy} onClick={() => void onDone()}>
          Cancel
        </button>
        {profile?.hasPin && (
          <button className="wx-btn" disabled={busy} onClick={() => void run(clearPin)}>
            Remove PIN
          </button>
        )}
        {profile && !profile.main && (
          <button
            className="wx-btn"
            disabled={busy}
            onClick={() => {
              if (window.confirm(`Delete ${profile.name}? Their Continue Watching and My List go too.`)) void run(() => api.removeWatchProfile(profile.id))
            }}
          >
            Delete profile
          </button>
        )}
      </div>
    </div>
  )
}
