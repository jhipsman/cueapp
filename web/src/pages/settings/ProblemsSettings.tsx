import { useCallback, useEffect, useState } from 'react'
import { api } from '../../api'
import { copyText } from '../../components/CopyBox'
import Icon from '../../components/Icon'
import Switch from '../../components/Switch'
import { useToast } from '../../components/Toast'
import { appName, claudeText, seenText, splitCrashes, type Crash, type CrashList } from '../../crashReport'
import { agoText, exactTime } from '../../problemsView'

// Settings > System > Problems: crashes of Cue's own apps (the TV app on a
// Fire TV or Android TV, and this website). The same crash on any device or
// build is one line with a count. "Copy for Claude" puts everything needed to
// fix it on the clipboard, ready to paste into a Claude Code session.
export default function ProblemsSettings() {
  const toast = useToast()
  const [data, setData] = useState<CrashList | null>(null)
  const [error, setError] = useState('')
  const [open, setOpen] = useState<Set<number>>(new Set())
  const [showFixed, setShowFixed] = useState(false)

  const load = useCallback(
    () =>
      api
        .crashes()
        .then((d) => {
          setData(d)
          setError('')
        })
        .catch((e: Error) => setError(e.message)),
    [],
  )

  useEffect(() => {
    void load()
  }, [load])

  function toggle(id: number) {
    setOpen((s) => {
      const next = new Set(s)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function setFixed(c: Crash, fixed: boolean) {
    try {
      await api.setCrashFixed(c.id, fixed)
      toast.success(fixed ? 'Marked fixed. If it happens again, it comes back here and to your phone.' : 'Open again.')
      await load()
    } catch (e) {
      toast.error((e as Error).message)
    }
  }

  async function remove(c: Crash) {
    try {
      await api.deleteCrash(c.id)
      await load()
    } catch (e) {
      toast.error((e as Error).message)
    }
  }

  async function setAlerts(on: boolean) {
    try {
      await api.setCrashAlerts(on)
      setData((d) => (d ? { ...d, alerts: on } : d))
    } catch (e) {
      toast.error((e as Error).message)
    }
  }

  const { open: openCrashes, fixed } = splitCrashes(data?.items ?? [])

  return (
    <div className="pl-page">
      <p className="pl-intro">
        Crashes of the TV app and this website. The same crash on any TV or build counts on one line. Use <strong>Copy for Claude</strong> to paste one into a Claude Code
        session to get it fixed.
      </p>

      <div className="pl-bar">
        <span className="pl-bar-text">{data && <span>{data.open === 0 ? 'No open crashes' : `${data.open} open ${data.open === 1 ? 'crash' : 'crashes'}`}</span>}</span>
        <div className="pl-actions">
          {data && (
            <Switch
              checked={data.alerts}
              onChange={(on) => void setAlerts(on)}
              label="Alert my phone about new crashes"
            />
          )}
        </div>
      </div>
      {data?.alerts && (
        <p className="pl-intro" style={{ fontSize: 13 }}>
          Alerts go to the phones where an administrator turned on alerts in Watch (Settings, Notifications). Each new kind of crash alerts once, and again if it comes back after
          you mark it fixed.
        </p>
      )}

      {error && <p className="error-text">{error}</p>}

      {!data && !error && (
        <div className="pl-list" aria-busy="true">
          {[0, 1].map((i) => (
            <div key={i} className="skeleton" style={{ height: 64, borderRadius: 14 }} />
          ))}
        </div>
      )}

      {data && openCrashes.length === 0 && (
        <div className="pl-empty">
          <Icon name="check" size={34} />
          <strong>No crashes</strong>
          <span>When the TV app or a page of this website crashes, it shows up here with its stack trace.</span>
        </div>
      )}

      {openCrashes.length > 0 && (
        <ul className="pl-list">
          {openCrashes.map((c) => (
            <CrashItem key={c.id} c={c} serverVersion={data?.serverVersion ?? ''} open={open.has(c.id)} onToggle={() => toggle(c.id)} onFixed={(f) => void setFixed(c, f)} onRemove={() => void remove(c)} />
          ))}
        </ul>
      )}

      {fixed.length > 0 && (
        <>
          <div className="pl-more">
            <button type="button" onClick={() => setShowFixed((v) => !v)}>
              {showFixed ? 'Hide fixed crashes' : `Show ${fixed.length} fixed`}
            </button>
          </div>
          {showFixed && (
            <ul className="pl-list">
              {fixed.map((c) => (
                <CrashItem key={c.id} c={c} serverVersion={data?.serverVersion ?? ''} open={open.has(c.id)} onToggle={() => toggle(c.id)} onFixed={(f) => void setFixed(c, f)} onRemove={() => void remove(c)} />
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  )
}

function CrashItem({
  c,
  serverVersion,
  open,
  onToggle,
  onFixed,
  onRemove,
}: {
  c: Crash
  serverVersion: string
  open: boolean
  onToggle: () => void
  onFixed: (fixed: boolean) => void
  onRemove: () => void
}) {
  const toast = useToast()
  const bodyId = `crash-body-${c.id}`

  async function copy(text: string, what: string) {
    if (await copyText(text)) toast.success(what)
    else toast.error('Your browser blocked copying. Select the text and copy it yourself.')
  }

  return (
    <li className={`pl-row error${c.fixedAt ? ' read' : ' unread'}${open ? ' open' : ''}`}>
      <button type="button" className="pl-head" aria-expanded={open} aria-controls={bodyId} onClick={onToggle}>
        <span className="pl-level error">
          <Icon name={c.app === 'web' ? 'globe' : 'tv'} size={13} /> {appName(c.app)}
        </span>
        <span className="pl-title">{c.headline || 'Crash'}</span>
        {c.fixedAt && <span className="pl-new">Fixed</span>}
        <span className="pl-meta">
          <span className="pl-times">{seenText(c)}</span>
          <span className="pl-when" title={exactTime(c.lastAt)}>
            {agoText(c.lastAt)}
          </span>
          <Icon name="chevron-down" size={16} className="pl-chev" />
        </span>
      </button>

      <div className="pl-sub">
        <span className="pl-sub-text">
          {c.place && <code>{c.place}</code>}
          {c.device && <span>{c.device}</span>}
          {c.version && <span>Version {c.version}</span>}
        </span>
        <button type="button" className="pl-read" onClick={() => void copy(claudeText(c, serverVersion), 'Copied. Paste it into Claude Code.')}>
          <Icon name="sparkles" size={13} /> Copy for Claude
        </button>
      </div>

      {open && (
        <div className="pl-body" id={bodyId}>
          <section>
            <h4>Seen</h4>
            <p>
              {seenText(c)}. First {exactTime(c.firstAt)}, last {exactTime(c.lastAt)}.
            </p>
            {c.devices.length > 0 && <p className="pl-dim">Devices: {c.devices.join('; ')}</p>}
            {c.versions.length > 0 && <p className="pl-dim">Versions: {c.versions.join(', ')}</p>}
            {c.fixedAt && <p className="pl-dim">Marked fixed {exactTime(c.fixedAt)}.</p>}
          </section>
          <section>
            <h4>What to do</h4>
            <p>Copy it for Claude and paste it into a Claude Code session on the Cue repository. Once the fix is installed, mark it fixed: if it happens again, you'll hear.</p>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <button type="button" className="btn-with-icon" onClick={() => onFixed(!c.fixedAt)}>
                <Icon name={c.fixedAt ? 'refresh' : 'check'} size={14} /> {c.fixedAt ? 'Open again' : 'Mark fixed'}
              </button>
              <button type="button" className="btn-with-icon" onClick={onRemove}>
                <Icon name="trash" size={14} /> Remove
              </button>
            </div>
          </section>
          <section style={{ gridColumn: '1 / -1' }}>
            <div className="pl-detail-head">
              <h4>Stack trace</h4>
              <button type="button" className="pl-copy" onClick={() => void copy(c.trace, 'Copied.')}>
                <Icon name="list" size={13} /> Copy
              </button>
            </div>
            <pre className="pl-detail">{c.trace}</pre>
          </section>
        </div>
      )}
    </li>
  )
}
