import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import { useDocumentTitle } from '../documentTitle'
import { Spinner } from './parts'
import VideoControls from './VideoControls'

// A Live TV recording, in Watch's player. One the browser can't play as it
// is (an MPEG-2 channel) is converted by Cue as it plays.
export default function RecordingPlayer() {
  const id = Number(useParams().id)
  const navigate = useNavigate()
  const [info, setInfo] = useState<{ url: string; title: string; channel: string; convert?: string } | null>(null)
  const [conv, setConv] = useState<{ stream: string; duration: number; offset: number } | null>(null)
  const [error, setError] = useState('')
  const [videoEl, setVideoEl] = useState<HTMLVideoElement | null>(null)
  const ref = useCallback((el: HTMLVideoElement | null) => setVideoEl(el), [])
  useDocumentTitle(info?.title ?? 'Recording')

  useEffect(() => {
    api
      .playLiveRecording(id)
      .then(setInfo)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [id])

  function onError() {
    if (!info?.convert || conv) {
      setError("This recording won't play in this browser. Watch it on the TV app.")
      return
    }
    api
      .convertProbe(info.convert)
      .then((p) => setConv({ stream: p.stream, duration: p.duration, offset: 0 }))
      .catch(() => setError("This recording won't play in this browser. Watch it on the TV app."))
  }

  const src = conv ? `${conv.stream}&t=${conv.offset}` : info?.url
  return (
    <div className="wx-player">
      {!info && !error && <Spinner label="Opening the recording…" />}
      {error && (
        <div className="wx-player-center">
          <p className="wx-error">{error}</p>
          <button className="wx-btn" onClick={() => navigate(-1)}>
            Go back
          </button>
        </div>
      )}
      {info && !error && (
        <>
          <video key={src} ref={ref} src={src} autoPlay playsInline onError={onError} />
          <VideoControls
            video={videoEl}
            title={info.title}
            subtitle={`${info.channel} · recording`}
            onBack={() => navigate(-1)}
            timeline={
              conv && conv.duration > 0
                ? { offset: conv.offset, duration: conv.duration, onSeek: (sec) => setConv((c) => (c ? { ...c, offset: Math.max(0, Math.floor(sec)) } : c)) }
                : undefined
            }
          />
        </>
      )}
    </div>
  )
}
