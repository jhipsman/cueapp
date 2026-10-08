import { useEffect, useRef } from 'react'
import type Hls from 'hls.js'

export interface StreamSource {
  kind: 'hls' | 'ts'
  url: string
}

// How long a source gets to start playing before the next is tried.
const START_WAIT_MS = 12_000

// useLiveStream plays a live channel (or a catch-up show) in a <video>. It
// tries each source in turn: an HLS playlist through hls.js (or the
// browser's own HLS, on Safari and iPhone), or one MPEG-TS stream through
// mpegts.js, which is how many IPTV providers send catch-up. A source that
// fails, or doesn't start within a few seconds, gives way to the next. A
// new list (or none) stops the old stream first, so only one ever plays:
// IPTV plans often allow a single connection.
export function useLiveStream(video: HTMLVideoElement | null, sources: StreamSource[] | null, onError: (message: string) => void) {
  const onErrorRef = useRef(onError)
  onErrorRef.current = onError
  const key = sources ? sources.map((s) => s.url).join('|') : ''

  useEffect(() => {
    const v = video
    if (!v || !sources || sources.length === 0) return
    let stopped = false
    let stopCurrent: () => void = () => undefined
    let timer: ReturnType<typeof setTimeout> | undefined
    let lastProblem = ''

    const nativeHLS = v.canPlayType('application/vnd.apple.mpegurl') !== '' && !/Chrome|Firefox/.test(navigator.userAgent)

    const tryAt = async (i: number) => {
      stopCurrent()
      clearTimeout(timer)
      if (stopped) return
      if (i >= sources.length) {
        onErrorRef.current(
          lastProblem ||
            "This won't play here. It may be off the air, too many devices may be watching at once, or your provider doesn't offer it in browsers: try the app on your TV.",
        )
        return
      }
      const src = sources[i]
      let started = false
      const next = (problem: string) => {
        if (started || stopped) return
        lastProblem = problem
        void tryAt(i + 1)
      }
      const onPlaying = () => {
        started = true
        clearTimeout(timer)
      }
      v.addEventListener('playing', onPlaying)
      timer = setTimeout(() => next(''), START_WAIT_MS)

      if (src.kind === 'hls' && nativeHLS) {
        const onErr = () => next('')
        v.addEventListener('error', onErr)
        v.src = src.url
        void v.play().catch(() => undefined)
        stopCurrent = () => {
          v.removeEventListener('playing', onPlaying)
          v.removeEventListener('error', onErr)
          v.removeAttribute('src')
          v.load()
        }
        return
      }
      if (src.kind === 'hls') {
        const { default: HlsJs } = await import('hls.js')
        if (stopped) return
        if (!HlsJs.isSupported()) return next("This browser can't play live TV. Try Chrome, Edge, Firefox or Safari.")
        const hls: Hls = new HlsJs({ enableWorker: true, liveDurationInfinity: true, backBufferLength: 30 })
        let recovered = false
        hls.on(HlsJs.Events.ERROR, (_e, d) => {
          if (!d.fatal) return
          if (started && d.type === HlsJs.ErrorTypes.MEDIA_ERROR && !recovered) {
            recovered = true
            hls.recoverMediaError()
            return
          }
          if (started && d.type === HlsJs.ErrorTypes.NETWORK_ERROR && !recovered) {
            recovered = true
            hls.startLoad()
            return
          }
          if (started) {
            onErrorRef.current("The stream stopped. Pick the channel again to restart it.")
            return
          }
          next(d.response?.code === 502 ? "The channel didn't answer. It may be off the air, or too many devices are watching at once." : '')
        })
        hls.loadSource(src.url)
        hls.attachMedia(v)
        void v.play().catch(() => undefined)
        stopCurrent = () => {
          v.removeEventListener('playing', onPlaying)
          hls.destroy()
          v.removeAttribute('src')
          v.load()
        }
        return
      }
      // One MPEG-TS stream.
      const { default: mpegts } = await import('mpegts.js')
      if (stopped) return
      if (!mpegts.isSupported()) return next("This browser can't play this kind of stream. Try Chrome, Edge or Firefox, or the app on your TV.")
      const player = mpegts.createPlayer(
        { type: 'mpegts', isLive: true, url: new URL(src.url, window.location.href).toString() },
        { enableWorker: true, lazyLoad: false, liveBufferLatencyChasing: false },
      )
      player.on(mpegts.Events.ERROR, () => {
        if (started) onErrorRef.current('The stream stopped. Pick it again to restart it.')
        else next('')
      })
      player.attachMediaElement(v)
      player.load()
      void Promise.resolve(player.play()).catch(() => undefined)
      stopCurrent = () => {
        v.removeEventListener('playing', onPlaying)
        try {
          player.pause()
          player.unload()
          player.detachMediaElement()
          player.destroy()
        } catch {
          // Already gone.
        }
      }
    }

    void tryAt(0)
    return () => {
      stopped = true
      clearTimeout(timer)
      stopCurrent()
    }
    // The stream follows the list of addresses only.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [video, key])
}

// Recent channels, newest first, kept on this device.
const RECENT_KEY = 'cue-live-recent'

export function recentChannels(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(RECENT_KEY) ?? '[]')
    return Array.isArray(v) ? v.filter((x) => typeof x === 'string') : []
  } catch {
    return []
  }
}

export function rememberChannel(id: string) {
  try {
    const list = [id, ...recentChannels().filter((x) => x !== id)].slice(0, 15)
    localStorage.setItem(RECENT_KEY, JSON.stringify(list))
  } catch {
    // A private window: no recents, nothing else lost.
  }
}
