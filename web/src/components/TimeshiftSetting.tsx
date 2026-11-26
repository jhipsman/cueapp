import { useState } from 'react'
import { api, type IPTVSettings } from '../api'
import { useToast } from './Toast'

// Settings > Streaming > Live TV: pausing and going back in live TV. While
// a channel plays, the server keeps its last half hour (or up to two
// hours), so anyone watching can pause it, go back, and catch up.
const CHOICES = [
  { minutes: 0, label: 'Off' },
  { minutes: 15, label: '15 minutes' },
  { minutes: 30, label: '30 minutes' },
  { minutes: 60, label: '1 hour' },
  { minutes: 120, label: '2 hours' },
]

export default function TimeshiftSetting({ value }: { value: NonNullable<IPTVSettings['timeshift']> }) {
  const toast = useToast()
  const [minutes, setMinutes] = useState(value.minutes)
  const [busy, setBusy] = useState(false)
  async function pick(n: number) {
    const was = minutes
    setMinutes(n)
    setBusy(true)
    try {
      await api.saveTimeshift(n)
      toast.success(n === 0 ? 'Pausing live TV is off.' : 'Saved. Channels started from now keep that long.')
    } catch (e) {
      setMinutes(was)
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="timeshift-setting">
      <h3 style={{ margin: '18px 0 6px' }}>Pause and go back</h3>
      <p style={{ color: 'var(--text-dim)', marginTop: 0 }}>
        While a channel plays, the server keeps the last part of it, so you can pause live TV, go back, and catch up. It uses the server&apos;s disk
        (about 1 GB an hour for an HD channel) while the channel plays, and deletes it after.
        {!value.available && <span className="error-text"> This server can&apos;t: ffmpeg isn&apos;t installed.</span>}
      </p>
      <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        Keep
        <select value={minutes} disabled={busy || !value.available} onChange={(e) => void pick(Number(e.target.value))}>
          {CHOICES.map((c) => (
            <option key={c.minutes} value={c.minutes}>
              {c.label}
            </option>
          ))}
        </select>
        {value.running > 0 && <span style={{ color: 'var(--text-dim)' }}>· {value.running === 1 ? 'one channel' : `${value.running} channels`} kept now</span>}
      </label>
    </div>
  )
}
