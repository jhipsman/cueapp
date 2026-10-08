import { useEffect, useRef } from 'react'
import Icon from '../components/Icon'

// A video found only on YouTube plays in YouTube's own player (it can't be
// played any other way). Its controls are YouTube's; Cue adds a bar on top
// with Back, Next episode and Try another version.

const ORIGIN = 'https://www.youtube-nocookie.com'

interface Props {
  id: string
  title: string
  note?: string
  startSec?: number
  onBack: () => void
  onEnded: () => void
  onError: () => void
  onNext?: () => void
  onOther?: () => void
}

export default function YouTubePlayer({ id, title, note, startSec, onBack, onEnded, onError, onNext, onOther }: Props) {
  const frame = useRef<HTMLIFrameElement | null>(null)
  const done = useRef({ onEnded, onError })
  useEffect(() => {
    done.current = { onEnded, onError }
  })

  // YouTube's player says when the video ends or can't play (removed, or
  // not allowed outside YouTube), once it's asked to.
  useEffect(() => {
    const onMsg = (e: MessageEvent) => {
      if (e.origin !== ORIGIN || e.source !== frame.current?.contentWindow || typeof e.data !== 'string') return
      let m: { event?: string; info?: unknown }
      try {
        m = JSON.parse(e.data)
      } catch {
        return
      }
      if (m.event === 'onError') done.current.onError()
      if (m.event === 'onStateChange' && m.info === 0) done.current.onEnded()
      if (m.event === 'infoDelivery' && typeof m.info === 'object' && m.info && (m.info as { playerState?: number }).playerState === 0) {
        done.current.onEnded()
      }
    }
    window.addEventListener('message', onMsg)
    return () => window.removeEventListener('message', onMsg)
  }, [])

  const listen = () => {
    const w = frame.current?.contentWindow
    if (!w) return
    // Asked a few times: the player may not be ready at the first.
    for (const ms of [0, 500, 1500, 3000]) {
      setTimeout(() => w.postMessage(JSON.stringify({ event: 'listening', id: 1, channel: 'widget' }), ORIGIN), ms)
    }
  }

  const q = new URLSearchParams({
    autoplay: '1',
    rel: '0',
    playsinline: '1',
    enablejsapi: '1',
    origin: window.location.origin,
  })
  if (startSec && startSec > 0) q.set('start', String(Math.floor(startSec)))

  return (
    <div className="wx-yt">
      <iframe
        ref={frame}
        key={id}
        src={`${ORIGIN}/embed/${encodeURIComponent(id)}?${q}`}
        title={title}
        allow="autoplay; encrypted-media; fullscreen; picture-in-picture"
        allowFullScreen
        // The site sends no referrer; YouTube's player won't start without one.
        referrerPolicy="strict-origin-when-cross-origin"
        onLoad={listen}
      />
      <div className="wx-yt-bar">
        <button className="wx-round" onClick={onBack} aria-label="Back">
          <Icon name="arrow-left" size={20} />
        </button>
        <div className="wx-yt-title">
          <b>{title}</b>
          <small>{note || 'Playing from YouTube'}</small>
        </div>
        {onOther && (
          <button className="wx-btn small" onClick={onOther}>
            <Icon name="layers" size={14} /> Another version
          </button>
        )}
        {onNext && (
          <button className="wx-btn small" onClick={onNext}>
            <Icon name="skip-forward" size={14} /> Next episode
          </button>
        )}
      </div>
    </div>
  )
}
