import { useEffect } from 'react'
import type Hls from 'hls.js'

// useLiveStream plays a live channel's HLS address in a <video>: through
// hls.js, or the browser's own HLS (Safari, iPhone). A new address (or none)
// stops the old stream first, so only one channel ever plays: IPTV plans
// often allow a single connection.
export function useLiveStream(video: HTMLVideoElement | null, url: string | null, onError: (message: string) => void) {
  useEffect(() => {
    const v = video
    if (!v || !url) return
    let hls: Hls | null = null
    let stopped = false
    if (v.canPlayType('application/vnd.apple.mpegurl') && !/Chrome|Firefox/.test(navigator.userAgent)) {
      v.src = url
      void v.play().catch(() => undefined)
    } else {
      void import('hls.js').then(({ default: HlsJs }) => {
        if (stopped) return
        if (!HlsJs.isSupported()) {
          onError("This browser can't play live TV. Try Chrome, Edge, Firefox or Safari.")
          return
        }
        hls = new HlsJs({ enableWorker: true, liveDurationInfinity: true, backBufferLength: 30 })
        let recovered = false
        hls.on(HlsJs.Events.ERROR, (_e, d) => {
          if (!d.fatal || !hls) return
          if (d.type === HlsJs.ErrorTypes.MEDIA_ERROR && !recovered) {
            recovered = true
            hls.recoverMediaError()
            return
          }
          if (d.type === HlsJs.ErrorTypes.NETWORK_ERROR && !recovered) {
            recovered = true
            hls.startLoad()
            return
          }
          onError(
            d.response?.code === 502
              ? "The channel didn't answer. It may be off the air, or too many devices are watching at once."
              : "This channel won't play here. Try another, or the app on your TV.",
          )
        })
        hls.loadSource(url)
        hls.attachMedia(v)
        void v.play().catch(() => undefined)
      })
    }
    return () => {
      stopped = true
      hls?.destroy()
      v.removeAttribute('src')
      v.load()
    }
    // onError is the caller's setter; the stream follows the address only.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [video, url])
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
