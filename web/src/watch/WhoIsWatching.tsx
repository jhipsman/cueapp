import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import type { WatchProfile } from '../api'
import BrandMark from '../components/BrandMark'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { Avatar, useProfiles } from './profiles'

// "Who's watching?": the profile picker. A locked profile asks for its PIN.
// With ?then=… it goes there after picking (Manage profiles sends people
// here to open the main profile first).
export default function WhoIsWatching() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { data, choose } = useProfiles()
  const [asking, setAsking] = useState<WatchProfile | null>(null)
  useDocumentTitle("Who's watching?")
  const then = params.get('then') ?? '/watch'
  const onlyMain = params.get('main') === '1'

  async function pick(p: WatchProfile) {
    if (p.hasPin) {
      setAsking(p)
      return
    }
    await choose(p)
    navigate(then, { replace: true })
  }

  if (!data) return <div className="wx-who" />
  const shown = onlyMain ? data.profiles.filter((p) => p.main) : data.profiles

  return (
    <div className="wx-who">
      <div className="wx-who-brand">
        <BrandMark className="wx-brand-mark" /> Cue
      </div>
      {asking ? (
        <PinPad
          profile={asking}
          onCancel={() => setAsking(null)}
          onPin={async (pin) => {
            await choose(asking, pin)
            navigate(then, { replace: true })
          }}
        />
      ) : (
        <>
          <h1>{onlyMain ? 'Open the main profile to manage profiles' : "Who's watching?"}</h1>
          <div className="wx-who-grid">
            {shown.map((p) => (
              <button key={p.id} className="wx-who-tile" onClick={() => void pick(p)}>
                <Avatar profile={p} size={120} />
                <span>
                  {p.name} {p.hasPin && <Icon name="key" size={14} />}
                </span>
              </button>
            ))}
          </div>
          {!onlyMain && (
            <button className="wx-btn wx-who-manage" onClick={() => navigate('/watch/profiles')}>
              Manage profiles
            </button>
          )}
        </>
      )}
    </div>
  )
}

// PinPad asks for a profile's 4-digit PIN.
export function PinPad({ profile, onPin, onCancel }: { profile: WatchProfile; onPin: (pin: string) => Promise<void>; onCancel: () => void }) {
  const [pin, setPin] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(value: string) {
    setBusy(true)
    setError('')
    try {
      await onPin(value)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setPin('')
    } finally {
      setBusy(false)
    }
  }

  function press(d: string) {
    if (busy) return
    const next = (pin + d).slice(0, 4)
    setPin(next)
    if (next.length === 4) void submit(next)
  }

  return (
    <div className="wx-pin">
      <Avatar profile={profile} size={84} />
      <h1>Enter the PIN for {profile.name}</h1>
      <div className="wx-pin-dots" aria-label={`${pin.length} of 4 digits entered`}>
        {[0, 1, 2, 3].map((i) => (
          <span key={i} className={i < pin.length ? 'on' : ''} />
        ))}
      </div>
      <input
        className="wx-pin-input"
        inputMode="numeric"
        autoFocus
        value={pin}
        aria-label="PIN"
        onChange={(e) => {
          const v = e.target.value.replace(/\D/g, '').slice(0, 4)
          setPin(v)
          if (v.length === 4) void submit(v)
        }}
      />
      <div className="wx-pin-keys">
        {['1', '2', '3', '4', '5', '6', '7', '8', '9'].map((d) => (
          <button key={d} onClick={() => press(d)} disabled={busy}>
            {d}
          </button>
        ))}
        <button onClick={onCancel}>Back</button>
        <button onClick={() => press('0')} disabled={busy}>
          0
        </button>
        <button onClick={() => setPin(pin.slice(0, -1))} disabled={busy} aria-label="Delete">
          ⌫
        </button>
      </div>
      {error && <p className="wx-error">{error}</p>}
    </div>
  )
}
