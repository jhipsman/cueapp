import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api } from '../api'
import { setPreviewsOn } from './living'

// The account's own settings (Settings > Playback and the tour), kept on
// the server so they follow it to every device. A copy in this browser
// makes them ready at once; the server's answer wins when it comes.

export type PrefKey = 'tourSeen' | 'autoplayNext' | 'previews' | 'previewSound'

const DEFAULTS: Record<PrefKey, string> = { tourSeen: '0', autoplayNext: '1', previews: '1', previewSound: '1' }
const CACHE = 'cue.prefs.v1'

function cached(): Partial<Record<PrefKey, string>> {
  try {
    return JSON.parse(localStorage.getItem(CACHE) ?? '{}')
  } catch {
    return {}
  }
}

interface Prefs {
  loaded: boolean
  get: (k: PrefKey) => string
  on: (k: PrefKey) => boolean
  set: (k: PrefKey, v: string) => Promise<void>
}

const Ctx = createContext<Prefs | null>(null)

export function AccountPrefsProvider({ children }: { children: ReactNode }) {
  const [values, setValues] = useState<Partial<Record<PrefKey, string>>>(cached)
  const [loaded, setLoaded] = useState(false)
  useEffect(() => {
    api
      .accountPrefs()
      .then((p) => {
        setValues(p as Partial<Record<PrefKey, string>>)
        try {
          localStorage.setItem(CACHE, JSON.stringify(p))
        } catch {
          /* no storage */
        }
      })
      .catch(() => undefined)
      .finally(() => setLoaded(true))
  }, [])
  const get = useCallback((k: PrefKey) => values[k] ?? DEFAULTS[k], [values])
  // Previews are read in many places (living.tsx): keep its switch in step.
  useEffect(() => setPreviewsOn(get('previews') === '1'), [get])
  const set = useCallback(async (k: PrefKey, v: string) => {
    setValues((old) => {
      const next = { ...old, [k]: v }
      try {
        localStorage.setItem(CACHE, JSON.stringify(next))
      } catch {
        /* no storage */
      }
      return next
    })
    await api.setAccountPrefs({ [k]: v })
  }, [])
  const value = useMemo<Prefs>(() => ({ loaded, get, on: (k) => get(k) === '1', set }), [loaded, get, set])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function usePrefs(): Prefs {
  const v = useContext(Ctx)
  if (!v) throw new Error('usePrefs outside AccountPrefsProvider')
  return v
}
