import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchTheme } from '../api'
import CueWordmark from '../components/CueWordmark'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { Spinner } from './parts'
import { useProfiles } from './profiles'
import { ACCENTS, applyTheme, BACKGROUNDS, DEFAULT_THEME, GLOWS, themeVars } from './theme'
import { onTV } from './tv'

// Colors: the profile on this device picks its accent, glow and background.
// Watch changes as you pick; Save keeps it for the profile on every device.
export default function ThemePage() {
  const { active } = useProfiles()
  useDocumentTitle('Colors')
  // Starts from the profile's own colors, so it waits for the profile.
  if (!active) return <Spinner label="Loading…" />
  return <ThemeEditor key={active.id} />
}

function ThemeEditor() {
  const navigate = useNavigate()
  const { active, refresh } = useProfiles()
  const [theme, setTheme] = useState<WatchTheme>(() => ({ ...DEFAULT_THEME, ...(active?.theme ?? {}) }))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const saved = useRef(false)

  // Show it live; leaving without saving puts the profile's own back.
  useEffect(() => applyTheme(theme), [theme])
  const original = active?.theme
  useEffect(
    () => () => {
      if (!saved.current) applyTheme(original)
    },
    [original],
  )

  const set = (p: Partial<WatchTheme>) => setTheme((t) => ({ ...t, ...p }))

  async function save() {
    setBusy(true)
    setError('')
    try {
      const t = await api.saveProfileTheme(theme)
      saved.current = true
      applyTheme(t)
      await refresh()
      navigate(-1)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const custom = !ACCENTS.some((a) => a.color === theme.accent)
  return (
    <div className="wx-theme">
      <div className="wx-theme-head">
        <button className="wx-round" onClick={() => navigate(-1)} aria-label="Back">
          <Icon name="arrow-left" size={20} />
        </button>
        <h1>Colors{active ? <small> for {active.name}</small> : null}</h1>
      </div>

      <div className="wx-theme-preview" aria-hidden="true">
        <CueWordmark className="wx-theme-mark" />
        <span className="wx-btn play small">
          <Icon name="play" size={16} /> Play
        </span>
        <span className="wx-theme-card" />
        <span className="wx-theme-chip">Live</span>
      </div>

      <section>
        <h2>Color</h2>
        <div className="wx-theme-swatches">
          {ACCENTS.map((a) => (
            <button
              key={a.color}
              className={`wx-swatch ${theme.accent === a.color ? 'on' : ''}`}
              style={{ background: a.color }}
              onClick={() => set({ accent: a.color })}
              aria-label={a.name}
              aria-pressed={theme.accent === a.color}
              title={a.name}
            >
              {theme.accent === a.color && <Icon name="check" size={18} strokeWidth={3} />}
            </button>
          ))}
          {!onTV() && (
            <label className={`wx-swatch wx-swatch-custom ${custom ? 'on' : ''}`} title="Any color" style={custom ? { background: theme.accent } : undefined}>
              <input type="color" value={theme.accent} onChange={(e) => set({ accent: e.target.value })} aria-label="Any color" />
              {!custom && <Icon name="plus" size={18} />}
            </label>
          )}
        </div>
      </section>

      <section>
        <h2>Glow</h2>
        <div className="wx-theme-options">
          {GLOWS.map((g) => (
            <button key={g.value} className={`wx-theme-option ${theme.glow === g.value ? 'on' : ''}`} onClick={() => set({ glow: g.value })} aria-pressed={theme.glow === g.value}>
              <span className={`wx-glow-sample glow-${g.value}`} />
              {g.label}
            </button>
          ))}
        </div>
      </section>

      <section>
        <h2>Background</h2>
        <div className="wx-theme-options">
          {BACKGROUNDS.map((b) => (
            <button
              key={b.value}
              className={`wx-theme-option ${theme.background === b.value ? 'on' : ''}`}
              onClick={() => set({ background: b.value })}
              aria-pressed={theme.background === b.value}
            >
              <span className="wx-bg-sample" style={{ background: themeVars({ ...theme, background: b.value })['--wx-bg'] }} />
              <span>
                {b.label}
                <small>{b.hint}</small>
              </span>
            </button>
          ))}
        </div>
      </section>

      {error && <p className="wx-error">{error}</p>}
      <div className="wx-actions">
        <button className="wx-btn play" disabled={busy} onClick={() => void save()}>
          {busy ? 'Saving…' : 'Save'}
        </button>
        <button className="wx-btn" onClick={() => setTheme(DEFAULT_THEME)}>
          Cue’s own colors
        </button>
      </div>
    </div>
  )
}
