import { useEffect, useRef, useState } from 'react'
import { api, type Episode, type PlayAnswer } from '../api'
import { KEY, onKey } from '../focus'
import { Icon, Spinner } from '../ui'
import type { PlayTarget } from './TitlePage'

// The player: the video fills the screen, on the TV's own player (it plays
// MKV, HEVC and Dolby sound). OK pauses and plays, left and right skip back
// 10 seconds and on 10 (fast forward and rewind 30), Back leaves. The bar
// shows on any press and hides itself. It starts where you left off, keeps
// your place as it goes, and on a version the TV can't play tries Cue's
// converted copy, then the next version. At a show's end: the next episode.

function clock(s: number): string {
  if (!isFinite(s) || s < 0) s = 0
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = Math.floor(s % 60)
  const two = (n: number) => (n < 10 ? `0${n}` : String(n))
  return `${h ? `${h}:` : ''}${h ? two(m) : m}:${two(sec)}`
}

export function Player({ target, onClose, onPlay }: { target: PlayTarget; onClose: () => void; onPlay: (t: PlayTarget) => void }) {
  const video = useRef<HTMLVideoElement>(null)
  const [option, setOption] = useState(1)
  const [answer, setAnswer] = useState<PlayAnswer | null>(null)
  const [useCopy, setUseCopy] = useState(false)
  const [problem, setProblem] = useState('')
  const [note, setNote] = useState('')
  const [paused, setPaused] = useState(false)
  const [waiting, setWaiting] = useState(true)
  const [bar, setBar] = useState(true)
  const [time, setTime] = useState({ at: 0, length: 0 })
  const [upNext, setUpNext] = useState<{ ep: Episode; in: number } | null>(null)
  const resumeAt = useRef(0)
  // Where it's got to, kept here: the video is gone by the time the last save runs.
  const place = useRef({ at: 0, length: 0 })
  const hideTimer = useRef(0)

  // Where you were.
  useEffect(() => {
    api
      .progress(target.kind, target.tmdbId, target.season, target.episode)
      .then((p) => {
        if (p && !p.finished && p.position > 30 && (!p.duration || p.position < p.duration - 60)) resumeAt.current = p.position
      })
      .catch(() => undefined)
  }, [target.kind, target.tmdbId, target.season, target.episode])

  // A version to play (the best first; the next when one won't).
  useEffect(() => {
    let live = true
    setAnswer(null)
    setUseCopy(false)
    setWaiting(true)
    api
      .play(target.kind, target.tmdbId, target.season, target.episode, option)
      .then((a) => {
        if (!live) return
        if (a.youtube) {
          setProblem("This one only plays on YouTube, which the LG app can't open yet.")
          return
        }
        setAnswer(a)
      })
      .catch((e) => live && setProblem(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [target.kind, target.tmdbId, target.season, target.episode, option])

  const src = answer ? (useCopy && answer.streamUrl ? answer.streamUrl : answer.url) : ''

  // Keeping your place: every 15 seconds, and on leaving.
  useEffect(() => {
    const save = () => {
      const { at, length } = place.current
      if (!length || at < 5) return
      void api
        .saveProgress({ kind: target.kind, tmdbId: target.tmdbId, season: target.season, episode: target.episode, position: Math.floor(at), duration: Math.floor(length) })
        .catch(() => undefined)
    }
    const t = window.setInterval(save, 15000)
    return () => {
      window.clearInterval(t)
      save()
    }
  }, [target.kind, target.tmdbId, target.season, target.episode])

  function showBar() {
    setBar(true)
    window.clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => {
      if (video.current && !video.current.paused) setBar(false)
    }, 4000)
  }

  function failed() {
    if (!answer) return
    if (!useCopy && answer.streamUrl && answer.streamUrl !== answer.url) {
      setNote('Trying Cue’s converted copy…')
      setUseCopy(true)
      return
    }
    if (option < answer.options) {
      setNote(`That version didn’t play on this TV. Trying another (${option + 1} of ${answer.options})…`)
      setOption(option + 1)
      return
    }
    setProblem('None of the versions found would play on this TV.')
  }

  // The next episode, at a show's end.
  async function ended() {
    if (target.kind !== 'tv') return onClose()
    try {
      const ep = await api.next(target.tmdbId, target.season, target.episode)
      if (ep && ep.aired && ep.episode) {
        setUpNext({ ep, in: 8 })
        return
      }
    } catch {
      // No next one.
    }
    onClose()
  }
  useEffect(() => {
    if (!upNext) return
    if (upNext.in <= 0) {
      playNext(upNext.ep)
      return
    }
    const t = window.setTimeout(() => setUpNext((u) => u && { ...u, in: u.in - 1 }), 1000)
    return () => window.clearTimeout(t)
  })
  function playNext(ep: Episode) {
    setUpNext(null)
    onPlay({ ...target, season: ep.season, episode: ep.episode, label: `S${ep.season}:E${ep.episode} · ${ep.title}` })
  }

  // The remote.
  useEffect(() =>
    onKey((code) => {
      const v = video.current
      if (code === KEY.back || code === KEY.stop) {
        onClose()
        return true
      }
      if (upNext) {
        if (code === KEY.ok) playNext(upNext.ep)
        return true
      }
      if (!v || problem) return code === KEY.ok ? (onClose(), true) : false
      showBar()
      const skip = (s: number) => {
        if (v.duration) v.currentTime = Math.max(0, Math.min(v.duration - 1, v.currentTime + s))
      }
      switch (code) {
        case KEY.ok:
        case KEY.playPause:
          if (v.paused) void v.play()
          else v.pause()
          return true
        case KEY.play:
          void v.play()
          return true
        case KEY.pause:
          v.pause()
          return true
        case KEY.left:
          skip(-10)
          return true
        case KEY.right:
          skip(10)
          return true
        case KEY.rewind:
          skip(-30)
          return true
        case KEY.forward:
          skip(30)
          return true
        case KEY.up:
        case KEY.down:
          return true
      }
      return false
    }),
  )

  const pct = time.length ? (time.at / time.length) * 100 : 0
  return (
    <div className="player" data-layer="">
      {src && (
        <video
          ref={video}
          key={src}
          src={src}
          autoPlay
          onLoadedMetadata={(e) => {
            const v = e.currentTarget
            if (resumeAt.current && v.duration > resumeAt.current) v.currentTime = resumeAt.current
            resumeAt.current = 0
            setNote('')
          }}
          onTimeUpdate={(e) => {
            const t = { at: e.currentTarget.currentTime, length: e.currentTarget.duration || 0 }
            place.current = t
            setTime(t)
          }}
          onPlaying={() => {
            setWaiting(false)
            setPaused(false)
            showBar()
          }}
          onPause={() => {
            setPaused(true)
            setBar(true)
          }}
          onWaiting={() => setWaiting(true)}
          onError={failed}
          onEnded={() => void ended()}
        />
      )}
      {!problem && (waiting || !answer) && (
        <div className="player-wait">
          <Spinner label={note || (answer ? 'Starting…' : 'Finding the best version…')} />
        </div>
      )}
      {problem && (
        <div className="player-problem">
          <h2>Couldn’t play {target.title}</h2>
          <p>{problem}</p>
          <p className="dim">Press OK or Back to go back.</p>
        </div>
      )}
      {upNext && (
        <div className="up-next">
          <small>Up next in {upNext.in}</small>
          <b>
            S{upNext.ep.season}:E{upNext.ep.episode} · {upNext.ep.title}
          </b>
          <span className="dim">OK to play now · Back to stop</span>
        </div>
      )}
      <div className={`player-bar${bar && !problem && !upNext ? ' on' : ''}`}>
        <div className="player-title">
          <b>{target.title}</b>
          {target.label && <span>{target.label}</span>}
          {answer && <small>{answer.quality}</small>}
        </div>
        <div className="player-time">
          <span className="player-state">
            <Icon name={paused ? 'play' : 'pause'} />
          </span>
          <span>{clock(time.at)}</span>
          <i className="player-track">
            <i style={{ width: `${pct}%` }} />
          </i>
          <span>{clock(time.length)}</span>
        </div>
      </div>
    </div>
  )
}
