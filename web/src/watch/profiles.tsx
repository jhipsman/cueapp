import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { api, setProfileToken, type WatchProfile, type WatchProfiles } from '../api'
import { applyTheme } from './theme'

// The household's profiles and the one this device is on, for every Watch page.

export const AVATAR_COLORS: Record<string, string> = {
  teal: '#14b8a6',
  red: '#e2453c',
  blue: '#3b82f6',
  purple: '#8b5cf6',
  orange: '#f97316',
  green: '#22c55e',
  pink: '#ec4899',
  yellow: '#eab308',
}

interface ProfilesState {
  data: WatchProfiles | null
  active: WatchProfile | null
  main: WatchProfile | null
  refresh: () => Promise<WatchProfiles | null>
  // choose signs this device in to a profile (pin for a locked one).
  choose: (p: WatchProfile, pin?: string) => Promise<void>
}

const Ctx = createContext<ProfilesState | null>(null)

export function ProfilesProvider({ children }: { children: ReactNode }) {
  const [data, setData] = useState<WatchProfiles | null>(null)

  const refresh = useCallback(async () => {
    try {
      const d = await api.watchProfiles()
      setData(d)
      return d
    } catch {
      return null
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const choose = useCallback(
    async (p: WatchProfile, pin = '') => {
      const res = await api.selectWatchProfile(p.id, pin)
      setProfileToken(res.token)
      await refresh()
    },
    [refresh],
  )

  const active = data?.profiles.find((p) => p.id === data.active) ?? null
  const main = data?.profiles.find((p) => p.main) ?? null
  // The profile's own colors (theme.ts).
  const themeKey = JSON.stringify(active?.theme ?? null)
  useEffect(() => {
    if (active) applyTheme(active.theme)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [themeKey])
  return <Ctx.Provider value={{ data, active, main, refresh, choose }}>{children}</Ctx.Provider>
}

export function useProfiles(): ProfilesState {
  const v = useContext(Ctx)
  if (!v) throw new Error('useProfiles outside ProfilesProvider')
  return v
}

// Avatar is a profile's colored tile with its initial.
export function Avatar({ profile, size = 40 }: { profile: Pick<WatchProfile, 'name' | 'avatar'>; size?: number }) {
  return (
    <span
      className="wx-avatar"
      style={{ width: size, height: size, fontSize: size * 0.48, background: AVATAR_COLORS[profile.avatar] ?? AVATAR_COLORS.teal }}
      aria-hidden="true"
    >
      {(profile.name.trim()[0] ?? '?').toUpperCase()}
    </span>
  )
}

// MainProfileOnly keeps the library manager and settings to the main
// profile: on any other (in a household with several), it goes to Watch.
export function MainProfileOnly({ children }: { children: ReactNode }) {
  const [state, setState] = useState<'checking' | 'ok' | 'watch'>('checking')
  useEffect(() => {
    api
      .watchProfiles()
      .then((d) => {
        const active = d.profiles.find((p) => p.id === d.active)
        setState(d.profiles.length > 1 && !active?.main ? 'watch' : 'ok')
      })
      .catch(() => setState('ok')) // an older server, or a hiccup: the server still checks
  }, [])
  if (state === 'checking') return null
  if (state === 'watch') return <Navigate to="/watch" replace />
  return <>{children}</>
}
