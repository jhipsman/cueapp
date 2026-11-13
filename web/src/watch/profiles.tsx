import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { api, setProfileToken, type WatchProfile, type WatchProfiles } from '../api'
import { applyTheme } from './theme'

// The household's profiles and the one this device is on, for every Watch page.

// Each is dark enough for the white initial on it to read (4.5:1 or more).
// The pictures a profile can wear on its color (the server's AvatarIcons),
// drawn as emoji so they look the same on every screen.
export const AVATAR_ICONS: Record<string, string> = {
  fox: '🦊', cat: '🐱', dog: '🐶', panda: '🐼', koala: '🐨', lion: '🦁', frog: '🐸', monkey: '🐵', unicorn: '🦄',
  octopus: '🐙', penguin: '🐧', owl: '🦉', rocket: '🚀', star: '⭐', game: '🎮', ball: '⚽', pizza: '🍕', alien: '👾',
}

// avatarParts splits "teal/fox" into its color and picture ("" for none).
export function avatarParts(avatar: string): { color: string; icon: string } {
  const [color, icon = ''] = avatar.split('/')
  return { color: color || 'teal', icon: AVATAR_ICONS[icon] ? icon : '' }
}

export const AVATAR_COLORS: Record<string, string> = {
  teal: '#0f766e',
  red: '#dc2626',
  blue: '#2563eb',
  purple: '#7c3aed',
  orange: '#c2410c',
  green: '#15803d',
  pink: '#be185d',
  yellow: '#a16207',
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

// Avatar is a profile's colored tile with its picture, or its initial.
export function Avatar({ profile, size = 40 }: { profile: Pick<WatchProfile, 'name' | 'avatar'>; size?: number }) {
  const { color, icon } = avatarParts(profile.avatar)
  return (
    <span
      className={`wx-avatar${icon ? ' pic' : ''}`}
      style={{ width: size, height: size, fontSize: size * (icon ? 0.62 : 0.48), background: AVATAR_COLORS[color] ?? AVATAR_COLORS.teal }}
      aria-hidden="true"
    >
      {icon ? AVATAR_ICONS[icon] : (profile.name.trim()[0] ?? '?').toUpperCase()}
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
