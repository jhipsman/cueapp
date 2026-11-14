import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useLocation } from 'react-router-dom'
import { api, type AlbumSound, type ListenTrack } from '../../api'

// The music player: one <audio> for the whole of Watch, so a song keeps
// playing while you browse; a queue with shuffle and repeat; the phone's
// lock screen and headphone buttons (Media Session). A song's address comes
// from its album's sound (listen.go), asked for when it's needed and, for
// the songs coming up next, a little ahead.

export type Repeat = 'off' | 'all' | 'one'

export interface MusicPlayer {
  queue: ListenTrack[]
  index: number
  current: ListenTrack | null
  playing: boolean
  // What the current song is waiting for, in words ("" once it plays).
  note: string
  waiting: boolean
  fetchProgress: number // 0 to 1 while Premiumize fetches the album
  pos: number
  length: number
  shuffle: boolean
  repeat: Repeat
  playTracks: (tracks: ListenTrack[], start?: number, opts?: { shuffle?: boolean }) => void
  toggle: () => void
  next: () => void
  prev: () => void
  seek: (sec: number) => void
  jump: (index: number) => void
  setShuffle: (on: boolean) => void
  cycleRepeat: () => void
  addNext: (t: ListenTrack) => void
  soundOf: (albumId: number) => AlbumSound | undefined
  close: () => void
}

const Ctx = createContext<MusicPlayer | null>(null)

export function useMusic(): MusicPlayer {
  const v = useContext(Ctx)
  if (!v) throw new Error('useMusic outside MusicProvider')
  return v
}

// Optional: components that may show outside the provider.
export function useMaybeMusic(): MusicPlayer | null {
  return useContext(Ctx)
}

function shuffled<T>(list: T[], keepFirst?: T): T[] {
  const rest = list.filter((x) => x !== keepFirst)
  for (let i = rest.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[rest[i], rest[j]] = [rest[j], rest[i]]
  }
  return keepFirst ? [keepFirst, ...rest] : rest
}

export function MusicProvider({ children }: { children: ReactNode }) {
  const [audioEl] = useState<HTMLAudioElement | null>(() => {
    if (typeof Audio === 'undefined') return null
    const a = new Audio()
    a.preload = 'auto'
    return a
  })
  const audio = useRef(audioEl)
  const [queue, setQueue] = useState<ListenTrack[]>([])
  const [original, setOriginal] = useState<ListenTrack[]>([]) // the order before shuffling
  const [index, setIndex] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [pos, setPos] = useState(0)
  const [length, setLength] = useState(0)
  const [shuffle, setShuffleState] = useState(false)
  const [repeat, setRepeat] = useState<Repeat>('off')
  const [sounds, setSounds] = useState<Record<number, AlbumSound>>({})
  const asking = useRef(new Set<number>())
  const wantPlay = useRef(false) // play as soon as the current song has an address
  const loadedSrc = useRef('')

  const current = queue[index] ?? null

  // ---- Album sound: asked for, then asked again every 2 s until settled.
  const ask = useCallback((albumId: number, again = false) => {
    if (asking.current.has(albumId) && !again) return
    asking.current.add(albumId)
    let tries = 0
    const go = (first: boolean) => {
      api
        .albumSound(albumId, first && again)
        .then((s) => {
          setSounds((m) => ({ ...m, [albumId]: s }))
          if ((s.status === 'looking' || s.status === 'fetching') && tries++ < 900) {
            window.setTimeout(() => go(false), s.status === 'fetching' ? 4000 : 1500)
          } else {
            asking.current.delete(albumId)
          }
        })
        .catch(() => {
          if (tries++ < 5) window.setTimeout(() => go(false), 3000)
          else asking.current.delete(albumId)
        })
    }
    go(true)
  }, [])

  // The current song's album, and the next two songs' albums, a little ahead.
  useEffect(() => {
    if (!current) return
    for (const t of [queue[index], queue[index + 1], queue[index + 2]]) {
      if (t && !sounds[t.albumId]) ask(t.albumId)
    }
  }, [current, queue, index, sounds, ask])

  const sound = current ? sounds[current.albumId] : undefined
  const url = current && sound?.status === 'ready' ? sound.tracks?.find((t) => t.id === current.id)?.url : undefined

  const next = useCallback(() => {
    setIndex((i) => {
      if (i + 1 < queue.length) return i + 1
      if (repeat === 'all' && queue.length > 0) return 0
      wantPlay.current = false
      setPlaying(false)
      audio.current?.pause()
      return i
    })
  }, [queue.length, repeat])

  // A song with no address (its album has nothing, or this song isn't in the
  // release found) is skipped after a moment, when more are queued.
  const missing = current && sound && (sound.status === 'none' || (sound.status === 'ready' && !url))
  useEffect(() => {
    if (!missing || !wantPlay.current || index + 1 >= queue.length) return
    const t = window.setTimeout(next, 2500)
    return () => window.clearTimeout(t)
  }, [missing, index, queue.length, next])

  // Load the current song's address into the player.
  useEffect(() => {
    const a = audio.current
    if (!a) return
    if (!url) {
      if (loadedSrc.current) {
        a.pause()
        a.removeAttribute('src')
        a.load()
        loadedSrc.current = ''
      }
      setPos(0)
      setLength(current?.length ?? 0)
      return
    }
    if (loadedSrc.current !== url) {
      loadedSrc.current = url
      a.src = url
      setPos(0)
      setLength(current?.length ?? 0)
    }
    if (wantPlay.current) void a.play().catch(() => setPlaying(false))
  }, [url, current])

  // The audio element's own events.
  useEffect(() => {
    const a = audio.current
    if (!a) return
    const onTime = () => setPos(a.currentTime)
    const onDur = () => isFinite(a.duration) && a.duration > 0 && setLength(a.duration)
    const onPlay = () => setPlaying(true)
    // The moment of silence ending isn't a pause: the song is still coming.
    const onPause = () => loadedSrc.current && setPlaying(false)
    const onEnded = () => {
      if (!loadedSrc.current) return // the moment of silence, not a song
      if (repeat === 'one') {
        a.currentTime = 0
        void a.play()
        return
      }
      next()
    }
    const onError = () => {
      // The address stopped working (old link): ask for the album again.
      const t = queue[index]
      if (t && loadedSrc.current) {
        loadedSrc.current = ''
        setSounds((m) => {
          const copy = { ...m }
          delete copy[t.albumId]
          return copy
        })
        ask(t.albumId, true)
      }
    }
    a.addEventListener('timeupdate', onTime)
    a.addEventListener('durationchange', onDur)
    a.addEventListener('play', onPlay)
    a.addEventListener('pause', onPause)
    a.addEventListener('ended', onEnded)
    a.addEventListener('error', onError)
    return () => {
      a.removeEventListener('timeupdate', onTime)
      a.removeEventListener('durationchange', onDur)
      a.removeEventListener('play', onPlay)
      a.removeEventListener('pause', onPause)
      a.removeEventListener('ended', onEnded)
      a.removeEventListener('error', onError)
    }
  }, [repeat, next, queue, index, ask])

  const playTracks = useCallback((tracks: ListenTrack[], start = 0, opts?: { shuffle?: boolean }) => {
    if (tracks.length === 0) return
    const first = tracks[Math.min(start, tracks.length - 1)]
    const on = opts?.shuffle ?? false
    setOriginal(tracks)
    setShuffleState(on)
    const q = on ? shuffled(tracks, opts?.shuffle && start === 0 ? undefined : first) : tracks
    setQueue(q)
    setIndex(on ? 0 : Math.min(start, tracks.length - 1))
    wantPlay.current = true
    setPlaying(true)
    // A tap is what lets a phone's browser start sound: start now if this
    // song's address is already known.
    const a = audio.current
    const s = sounds[(on ? q[0] : first).albumId]
    const u = s?.status === 'ready' ? s.tracks?.find((t) => t.id === (on ? q[0] : first).id)?.url : undefined
    if (a && u) {
      loadedSrc.current = u
      a.src = u
      void a.play().catch(() => setPlaying(false))
    } else if (a) {
      // Phones only let sound start from a tap: play a moment of silence
      // now, so the song can start by itself once its address arrives.
      loadedSrc.current = ''
      a.src = silence()
      void a.play().catch(() => undefined)
    }
  }, [sounds])

  const toggle = useCallback(() => {
    const a = audio.current
    if (!a || !current) return
    if (!loadedSrc.current) {
      // Still waiting for the song's address: play when it comes, or not.
      wantPlay.current = !wantPlay.current
      setPlaying(wantPlay.current)
      return
    }
    if (a.paused) {
      wantPlay.current = true
      void a.play().catch(() => undefined)
    } else {
      wantPlay.current = false
      a.pause()
    }
  }, [current])

  const prev = useCallback(() => {
    const a = audio.current
    if (a && a.currentTime > 3) {
      a.currentTime = 0
      return
    }
    wantPlay.current = true
    setIndex((i) => (i > 0 ? i - 1 : repeat === 'all' ? Math.max(0, queue.length - 1) : 0))
  }, [queue.length, repeat])

  const seek = useCallback((sec: number) => {
    const a = audio.current
    if (a && loadedSrc.current) a.currentTime = Math.max(0, sec)
  }, [])

  const jump = useCallback((i: number) => {
    wantPlay.current = true
    setPlaying(true)
    setIndex(i)
  }, [])

  const setShuffle = useCallback(
    (on: boolean) => {
      setShuffleState(on)
      if (!current) return
      if (on) {
        const q = shuffled(queue, current)
        setQueue(q)
        setIndex(0)
      } else {
        const i = original.findIndex((t) => t.id === current.id)
        setQueue(original.length ? original : queue)
        setIndex(Math.max(0, i))
      }
    },
    [current, queue, original],
  )

  const cycleRepeat = useCallback(() => setRepeat((r) => (r === 'off' ? 'all' : r === 'all' ? 'one' : 'off')), [])

  const addNext = useCallback(
    (t: ListenTrack) => {
      if (!current) {
        playTracks([t])
        return
      }
      setQueue((q) => [...q.slice(0, index + 1), t, ...q.slice(index + 1)])
      setOriginal((q) => [...q, t])
    },
    [current, index, playTracks],
  )

  const close = useCallback(() => {
    wantPlay.current = false
    audio.current?.pause()
    setQueue([])
    setOriginal([])
    setIndex(0)
  }, [])

  // A video starting pauses the music.
  const { pathname } = useLocation()
  useEffect(() => {
    if (/\/watch\/(play|live\/play|recording)\//.test(pathname) && audio.current && !audio.current.paused) {
      wantPlay.current = false
      audio.current.pause()
    }
  }, [pathname])

  // The lock screen, the notification shade and headphone buttons.
  useEffect(() => {
    const ms = typeof navigator !== 'undefined' ? navigator.mediaSession : undefined
    if (!ms || !current) return
    try {
      ms.metadata = new MediaMetadata({
        title: current.title,
        artist: current.artist.name,
        album: current.albumTitle,
        artwork: current.cover ? [{ src: current.cover.replace(/\/\d+x\d+-/, '/500x500-'), sizes: '500x500', type: 'image/jpeg' }] : [],
      })
      ms.setActionHandler('play', toggle)
      ms.setActionHandler('pause', toggle)
      ms.setActionHandler('nexttrack', next)
      ms.setActionHandler('previoustrack', prev)
      ms.setActionHandler('seekto', (d) => d.seekTime !== undefined && seek(d.seekTime))
    } catch {
      // Older browsers: no lock-screen controls.
    }
  }, [current, toggle, next, prev, seek])
  useEffect(() => {
    const ms = typeof navigator !== 'undefined' ? navigator.mediaSession : undefined
    if (ms) ms.playbackState = playing ? 'playing' : current ? 'paused' : 'none'
  }, [playing, current])

  const waiting = !!current && !url && !missing
  let note = ''
  if (current && !url) {
    if (!sound || sound.status === 'looking') note = 'Finding this album…'
    else if (sound.status === 'fetching') note = `Premiumize is getting this album${sound.progress ? ` (${Math.round(sound.progress * 100)}%)` : ''}…`
    else if (sound.status === 'none') note = sound.message || 'This album isn’t available.'
    else if (sound.status === 'ready') note = 'This song isn’t in the release found.'
  }

  const value = useMemo<MusicPlayer>(
    () => ({
      queue,
      index,
      current,
      playing,
      note,
      waiting,
      fetchProgress: sound?.status === 'fetching' ? (sound.progress ?? 0) : 0,
      pos,
      length,
      shuffle,
      repeat,
      playTracks,
      toggle,
      next,
      prev,
      seek,
      jump,
      setShuffle,
      cycleRepeat,
      addNext,
      soundOf: (id: number) => sounds[id],
      close,
    }),
    [queue, index, current, playing, waiting, note, sound, pos, length, shuffle, repeat, playTracks, toggle, next, prev, seek, jump, setShuffle, cycleRepeat, addNext, sounds, close],
  )
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

// silence is a tenth of a second of silent WAV, made here (as a blob, which
// the page's security policy allows).
let silentURL = ''
function silence(): string {
  if (silentURL) return silentURL
  const rate = 8000
  const samples = rate / 10
  const buf = new ArrayBuffer(44 + samples)
  const v = new DataView(buf)
  const text = (o: number, t: string) => [...t].forEach((c, i) => v.setUint8(o + i, c.charCodeAt(0)))
  text(0, 'RIFF')
  v.setUint32(4, 36 + samples, true)
  text(8, 'WAVEfmt ')
  v.setUint32(16, 16, true)
  v.setUint16(20, 1, true) // PCM
  v.setUint16(22, 1, true) // mono
  v.setUint32(24, rate, true)
  v.setUint32(28, rate, true)
  v.setUint16(32, 1, true)
  v.setUint16(34, 8, true) // 8 bits
  text(36, 'data')
  v.setUint32(40, samples, true)
  for (let i = 0; i < samples; i++) v.setUint8(44 + i, 128)
  silentURL = URL.createObjectURL(new Blob([buf], { type: 'audio/wav' }))
  return silentURL
}

export function clock(sec: number): string {
  if (!isFinite(sec) || sec < 0) sec = 0
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

// Deezer's covers come in any size: ask for the one that's needed.
export function sized(url: string | undefined, px: number): string {
  if (!url) return ''
  return url.replace(/\/\d+x\d+-/, `/${px}x${px}-`)
}
