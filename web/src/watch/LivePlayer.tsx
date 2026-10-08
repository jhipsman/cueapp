import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import type Hls from 'hls.js'
import { api, profileToken, type LiveChannel, type LiveChannels, type LivePlay } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { channelsIn, liveHref } from './LivePage'
import { Spinner } from './parts'
import { onTV, type TVPlayerResult } from './tv'

const hhmm = (s: string) => new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

// The live player: a channel full screen. Up and down (or the buttons) change
// channel within the group it was opened from. Browsers play through hls.js
// (Safari and iPhone natively); the TV app plays it in its own player.
export default function LivePlayer() {
  const { id = '' } = useParams()
  const [params] = useSearchParams()
  const cat = params.get('cat') ?? 'all'
  const navigate = useNavigate()
  const video = useRef<HTMLVideoElement>(null)
  const [play, setPlay] = useState<LivePlay | null>(null)
  const [channels, setChannels] = useState<LiveChannels | null>(null)
  const [error, setError] = useState('')
  const [idle, setIdle] = useState(false)
  useDocumentTitle(play?.name ?? 'Live TV')

  const list = channelsIn(channels, cat)
  const index = list.findIndex((c) => c.id === id)
  const channel: LiveChannel | undefined = list[index] ?? channels?.channels?.find((c) => c.id === id)

  const zap = useCallback(
    (step: number) => {
      if (list.length < 2) return
      const i = index < 0 ? 0 : (index + step + list.length) % list.length
      navigate(liveHref(list[i].id, cat), { replace: true })
    },
    [list, index, cat, navigate],
  )

  useEffect(() => {
    setPlay(null)
    setError('')
    api
      .livePlay(id)
      .then(setPlay)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [id])
  useEffect(() => {
    api
      .liveChannels()
      .then(setChannels)
      .catch(() => undefined)
  }, [])

  // In a browser: hls.js, or the browser's own HLS (Safari, iPhone).
  useEffect(() => {
    const v = video.current
    if (!play || !v || onTV()) return
    let hls: Hls | null = null
    let stopped = false
    if (v.canPlayType('application/vnd.apple.mpegurl') && !/Chrome|Firefox/.test(navigator.userAgent)) {
      v.src = play.url
      void v.play().catch(() => undefined)
    } else {
      void import('hls.js').then(({ default: HlsJs }) => {
        if (stopped) return
        if (!HlsJs.isSupported()) {
          setError("This browser can't play live TV. Try Chrome, Edge, Firefox or Safari.")
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
          const status = d.response?.code
          setError(
            status === 502
              ? "The channel didn't answer. It may be off the air, or too many devices are watching at once."
              : "This channel won't play here. Try another, or the app on your TV.",
          )
        })
        hls.loadSource(play.url)
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
  }, [play])

  // On a TV, the app's player plays it; up and down there change channel.
  useEffect(() => {
    if (!onTV() || !play) return
    window.cueTvPlayerDone = (r: TVPlayerResult) => {
      window.cueTvPlayerDone = undefined
      if (r.reason === 'channel-up') return zap(1)
      if (r.reason === 'channel-down') return zap(-1)
      if (r.reason === 'error') return setError(`This channel won't play.${r.message ? ' ' + r.message : ''}`)
      navigate(-1)
    }
    window.CueTV?.play(
      JSON.stringify({
        url: play.direct || new URL(play.url, window.location.href).toString(),
        title: `${play.num ? play.num + '  ' : ''}${play.name}`,
        subtitle: channel?.now ? `${channel.now.title} · until ${hhmm(channel.now.stop)}` : '',
        live: true,
        token: profileToken(),
      }),
    )
    // Once per channel: the guide arriving later mustn't restart the player.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [play])

  // Keys: up and down change channel, Escape leaves.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.fullscreenElement) navigate(-1)
      else if (e.key === 'ArrowUp' || e.key === 'PageUp' || e.key === 'ChannelUp') {
        e.preventDefault()
        zap(1)
      } else if (e.key === 'ArrowDown' || e.key === 'PageDown' || e.key === 'ChannelDown') {
        e.preventDefault()
        zap(-1)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate, zap])

  // The overlay fades while nothing moves.
  useEffect(() => {
    let t: ReturnType<typeof setTimeout>
    const wake = () => {
      setIdle(false)
      clearTimeout(t)
      t = setTimeout(() => setIdle(true), 4000)
    }
    wake()
    window.addEventListener('mousemove', wake)
    window.addEventListener('keydown', wake)
    window.addEventListener('touchstart', wake)
    return () => {
      clearTimeout(t)
      window.removeEventListener('mousemove', wake)
      window.removeEventListener('keydown', wake)
      window.removeEventListener('touchstart', wake)
    }
  }, [id])

  return (
    <div className={`wx-player wx-live-player${idle && play && !error ? ' idle' : ''}`}>
      <div className="wx-player-top">
        <button className="wx-round" onClick={() => navigate(-1)} aria-label="Back to the guide">
          <Icon name="x" size={20} />
        </button>
        {(play?.logo || channel?.logo) && <img className="wx-live-logo" src={play?.logo || channel?.logo} alt="" />}
        <div className="wx-player-title">
          {play ? `${play.num ? play.num + '  ' : ''}${play.name}` : (channel?.name ?? 'Tuning in…')}
          {channel?.now && (
            <small>
              Now: {channel.now.title} · until {hhmm(channel.now.stop)}
              {channel.next && ` · Next: ${channel.next.title}`}
            </small>
          )}
        </div>
        {list.length > 1 && (
          <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
            <button className="wx-btn small" onClick={() => zap(-1)} aria-label="Previous channel">
              Ch −
            </button>
            <button className="wx-btn small" onClick={() => zap(1)} aria-label="Next channel">
              Ch +
            </button>
          </div>
        )}
      </div>

      {!play && !error && <Spinner label="Tuning in…" />}
      {error && (
        <div className="wx-player-center">
          <p className="wx-error">{error}</p>
          <div className="wx-actions" style={{ justifyContent: 'center' }}>
            {list.length > 1 && (
              <button className="wx-btn" onClick={() => zap(1)}>
                Next channel
              </button>
            )}
            <button className="wx-btn" onClick={() => navigate(-1)}>
              Back to the guide
            </button>
          </div>
        </div>
      )}
      {play && !error && onTV() && <Spinner label="Starting the player…" />}
      {play && !error && !onTV() && <video ref={video} controls autoPlay playsInline />}
    </div>
  )
}
