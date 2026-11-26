// Talking to Cue from the LG app. The app's pages are files on the TV, so
// it can't use Cue's sign-in cookie: it keeps its session itself and sends
// it in a header (X-Cue-Session), saying it's the LG app (X-Cue-App), with
// the profile it's on (X-Cue-Profile), as the Fire TV app does.

const KEYS = { server: 'cue.server', session: 'cue.session', profile: 'cue.profile', profileId: 'cue.profileId' }

function load(key: string): string {
  try {
    return localStorage.getItem(key) || ''
  } catch {
    return ''
  }
}
function save(key: string, value: string) {
  try {
    if (value) localStorage.setItem(key, value)
    else localStorage.removeItem(key)
  } catch {
    // Not kept: asked again next time.
  }
}

export const saved = {
  get server() {
    return load(KEYS.server)
  },
  set server(v: string) {
    save(KEYS.server, v)
  },
  get session() {
    return load(KEYS.session)
  },
  set session(v: string) {
    save(KEYS.session, v)
  },
  get profile() {
    return load(KEYS.profile)
  },
  set profile(v: string) {
    save(KEYS.profile, v)
  },
  get profileId() {
    return Number(load(KEYS.profileId)) || 0
  },
  set profileId(v: number) {
    save(KEYS.profileId, v ? String(v) : '')
  },
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// An address as typed: a scheme added when missing, the trailing slash dropped.
export function tidyServer(s: string): string {
  s = s.trim()
  if (!s) return ''
  if (!/^https?:\/\//i.test(s)) s = 'http://' + s
  return s.replace(/\/+$/, '')
}

export async function request<T>(method: string, path: string, body?: unknown, server = saved.server): Promise<T> {
  const headers: Record<string, string> = { 'X-Cue-App': 'webos' }
  if (saved.session) headers['X-Cue-Session'] = saved.session
  if (saved.profile) headers['X-Cue-Profile'] = saved.profile
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  let r: Response
  try {
    r = await fetch(server + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), credentials: 'omit' })
  } catch {
    throw new ApiError(0, `Couldn't reach ${server.replace(/^https?:\/\//, '')}. Check the TV is online and Cue is running.`)
  }
  // Signing in hands the session over in this header.
  const session = r.headers.get('X-Cue-Session')
  if (session) saved.session = session
  const text = await r.text()
  let data: unknown = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = null
  }
  if (!r.ok) {
    const err = data && typeof data === 'object' ? (data as { error?: unknown }).error : undefined
    const msg = err ? String(err) : `Cue answered ${r.status}.`
    throw new ApiError(r.status, msg)
  }
  return data as T
}

const get = <T>(path: string) => request<T>('GET', path)

// ---- What Cue answers ----

export type Kind = 'movie' | 'tv'

export interface Card {
  kind: Kind
  tmdbId: number
  title: string
  year?: number
  overview?: string
  posterUrl?: string
  backdropUrl?: string
  logoUrl?: string
  rating?: number
  progress?: number
  season?: number
  episode?: number
  episodeTitle?: string
  new?: boolean
  quiet?: boolean
}
export interface Row {
  key: string
  title: string
  items: Card[]
}
export interface TopList {
  key: string
  name: string
  items: Card[]
  movies?: Card[]
  shows?: Card[]
}
export interface Home {
  hero?: Card
  rows: Row[]
  top10?: TopList[]
}
export interface Title extends Card {
  tagline?: string
  runtime?: number
  genres: string[]
  certification?: string
  cast: string[]
  seasons?: { number: number; name: string; episodes: number }[]
  inList: boolean
  resumeSeason?: number
  resumeEpisode?: number
  resumePosition?: number
  more: Card[]
  notOut?: string
  theaterNote?: string
}
export interface Episode {
  season: number
  episode: number
  title: string
  overview?: string
  airDate?: string
  runtime?: number
  stillUrl?: string
  aired: boolean
  progress?: number
  finished?: boolean
}
export interface PlayAnswer {
  title: string
  url: string
  streamUrl?: string
  fileName: string
  release: string
  quality: string
  option: number
  options: number
  youtube?: string
}
export interface Profile {
  id: number
  name: string
  avatar: string
  hasPin: boolean
  main: boolean
}

// ---- Calls ----

export const api = {
  // Whether there's a Cue at this address (any answer from its API).
  async reachable(server: string): Promise<boolean> {
    try {
      await request('GET', '/api/auth/me', undefined, server)
      return true
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) return true
      throw e
    }
  },
  me: () => get<{ id: number; username: string }>('/api/auth/me'),
  login: (username: string, password: string) => request<{ twoStep?: boolean; ticket?: string }>('POST', '/api/auth/login', { username, password }),
  loginCode: (ticket: string, code: string) => request<unknown>('POST', '/api/auth/login/code', { ticket, code }),
  tvLinkStart: () => request<{ code: string; token: string; expiresIn: number }>('POST', '/api/auth/tv-link'),
  tvLinkPoll: (token: string) => get<{ ok?: boolean; pending?: boolean }>(`/api/auth/tv-link/${token}`),
  tvLinkQR: (code: string) => `${saved.server}/api/auth/tv-link/qr/${encodeURIComponent(code)}`,
  async logout() {
    try {
      await request('POST', '/api/auth/logout')
    } catch {
      // Signed out here either way.
    }
    saved.session = ''
    saved.profile = ''
    saved.profileId = 0
  },
  profiles: () => get<{ profiles: Profile[] }>('/api/profiles'),
  async choose(p: Profile, pin = '') {
    const a = await request<{ token: string }>('POST', `/api/profiles/${p.id}/select`, { pin })
    saved.profile = a.token
    saved.profileId = p.id
  },
  home: () => get<Home>('/api/watch/home'),
  search: (q: string) => get<Card[]>(`/api/watch/search?q=${encodeURIComponent(q)}`),
  title: (kind: Kind, id: number) => get<Title>(`/api/watch/${kind}/${id}`),
  season: (id: number, n: number) => get<Episode[]>(`/api/watch/tv/${id}/season/${n}`),
  next: (id: number, season: number, episode: number) => get<Episode>(`/api/watch/tv/${id}/next?season=${season}&episode=${episode}`),
  setInList: (kind: Kind, id: number, on: boolean) => request<unknown>(on ? 'PUT' : 'DELETE', `/api/watch/list/${kind}/${id}`),
  play: (kind: Kind, id: number, season: number, episode: number, option: number) =>
    get<PlayAnswer>(`/api/play/tmdb/${kind}/${id}${kind === 'tv' ? `/${season}/${episode}` : ''}?option=${option}&client=webos`),
  progress: (kind: Kind, id: number, season: number, episode: number) =>
    get<{ position: number; duration: number; finished: boolean }>(`/api/watch/progress/${kind}/${id}?season=${season}&episode=${episode}`),
  saveProgress: (b: { kind: Kind; tmdbId: number; season: number; episode: number; position: number; duration: number }) =>
    request<unknown>('PUT', '/api/watch/progress', b),
}

// Pictures from TMDB at the size a TV shows them.
export function poster(url?: string): string {
  return url ? url.replace('/t/p/w500/', '/t/p/w342/') : ''
}
export function backdrop(url?: string): string {
  return url ? url.replace(/\/t\/p\/w\d+\//, '/t/p/w1280/') : ''
}
// A picture from Cue itself (a logo it serves): the full address.
export function fromCue(url?: string): string {
  if (!url) return ''
  return url.startsWith('/') ? saved.server + url : url
}
