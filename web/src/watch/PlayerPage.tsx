import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type PlayAnswer, type WatchEpisode } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { epCode, playHref, Spinner } from './parts'

// How often where you are is saved while playing, and how long the "Next
// episode" card counts down before playing it.
const SAVE_EVERY_MS = 15_000
const NEXT_COUNTDOWN = 10

// The player: full screen, starts by itself. It asks the server for the best
// version Premiumize can stream, resumes where you stopped, saves where you
// are, quietly moves to the next version if one won't play, and offers the
// next episode at the end.
export default function PlayerPage() {
  const params = useParams()
  const navigate = useNavigate()
  const kind = params.season !== undefined ? 'tv' : 'movie'
  const tmdbId = Number(params.tmdbId)
  const season = Number(params.season ?? 0)
  const episode = Number(params.episode ?? 0)

  const video = useRef<HTMLVideoElement>(null)
  const [answer, setAnswer] = useState<PlayAnswer | null>(null)
  const [useOriginal, setUseOriginal] = useState(false)
  const [error, setError] = useState('')
  const [note, setNote] = useState('')
  const [idle, setIdle] = useState(false)
  const [next, setNext] = useState<WatchEpisode | null>(null)
  const [countdown, setCountdown] = useState<number | null>(null)
  const resumeAt = useRef(0)
  const loading = useRef(false)
  useDocumentTitle(answer?.title ?? 'Playing')

  const load = useCallback(
    async (option: number, fresh = false) => {
      if (loading.current) return
      loading.current = true
      setError('')
      try {
        const a = await api.playTMDB(kind, tmdbId, season, episode, option, fresh)
        setAnswer(a)
        setUseOriginal(!a.streamUrl)
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e))
      } finally {
        loading.current = false
      }
    },
    [kind, tmdbId, season, episode],
  )

  // A new title or episode: start over.
  useEffect(() => {
    setAnswer(null)
    setNext(null)
    setCountdown(null)
    setNote('')
    resumeAt.current = 0
    api
      .watchProgress(kind, tmdbId, season, episode)
      .then((p) => {
        if (!p.finished && p.position > 30) resumeAt.current = p.position
      })
      .catch(() => undefined)
    void load(1)
  }, [kind, tmdbId, season, episode, load])

  const save = useCallback(() => {
    const v = video.current
    if (!v || !answer || !isFinite(v.duration) || v.currentTime < 5) return
    void api
      .saveWatchProgress({ kind, tmdbId, season, episode, position: Math.floor(v.currentTime), duration: Math.floor(v.duration) })
      .catch(() => undefined)
  }, [answer, kind, tmdbId, season, episode])

  // Save every few seconds while playing, and when leaving.
  useEffect(() => {
    const t = setInterval(() => {
      if (video.current && !video.current.paused) save()
    }, SAVE_EVERY_MS)
    return () => {
      clearInterval(t)
      save()
    }
  }, [save])

  // Controls fade away while the mouse is still.
  useEffect(() => {
    let t: ReturnType<typeof setTimeout>
    const wake = () => {
      setIdle(false)
      clearTimeout(t)
      t = setTimeout(() => setIdle(true), 3000)
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
  }, [])

  // Escape leaves the player.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.fullscreenElement) navigate(-1)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate])

  // The next-episode countdown.
  useEffect(() => {
    if (countdown === null || !next) return
    if (countdown <= 0) {
      navigate(playHref('tv', tmdbId, next.season, next.episode), { replace: true })
      return
    }
    const t = setTimeout(() => setCountdown((c) => (c === null ? null : c - 1)), 1000)
    return () => clearTimeout(t)
  }, [countdown, next, navigate, tmdbId])

  function onLoaded() {
    const v = video.current
    if (v && resumeAt.current > 0 && resumeAt.current < v.duration - 60) {
      v.currentTime = resumeAt.current
    }
    resumeAt.current = 0
  }

  // Near the end of an episode, find the next one (once).
  function onTime() {
    const v = video.current
    if (!v || kind !== 'tv' || next || !isFinite(v.duration)) return
    if (v.duration - v.currentTime < 60) {
      api
        .watchNext(tmdbId, season, episode)
        .then(setNext)
        .catch(() => undefined)
    }
  }

  function onEnded() {
    save()
    if (next) setCountdown(NEXT_COUNTDOWN)
  }

  // This version won't play here: its original file, then the next version.
  function onError() {
    if (!answer) return
    if (!useOriginal && answer.url && answer.streamUrl) {
      setUseOriginal(true)
      return
    }
    if (answer.option < answer.options) {
      setNote(`Version ${answer.option} wouldn't play here, so trying the next one.`)
      void load(answer.option + 1)
      return
    }
    setError("None of the versions found will play in this browser. Try another browser, or the app on your TV.")
  }

  const src = answer ? (useOriginal ? answer.url : answer.streamUrl || answer.url) : ''
  const label = kind === 'tv' ? epCode(season, episode) : ''

  return (
    <div className={`wx-player${idle && answer && !error ? ' idle' : ''}`}>
      <div className="wx-player-top">
        <button className="wx-round" onClick={() => navigate(-1)} aria-label="Back">
          <Icon name="x" size={20} />
        </button>
        <div className="wx-player-title">
          {answer?.title ?? 'Finding the best version…'}
          {answer && (
            <small>
              {label && `${label} · `}
              {answer.quality} · version {answer.option} of {answer.options}
              {note && ` · ${note}`}
            </small>
          )}
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
          {answer && answer.option < answer.options && (
            <button className="wx-btn small" onClick={() => void load(answer.option + 1)}>
              Try another version
            </button>
          )}
          {kind === 'tv' && (
            <button
              className="wx-btn small"
              onClick={() =>
                api
                  .watchNext(tmdbId, season, episode)
                  .then((n) => navigate(playHref('tv', tmdbId, n.season, n.episode), { replace: true }))
                  .catch(() => setNote('That was the latest episode.'))
              }
            >
              Next episode
            </button>
          )}
        </div>
      </div>

      {!answer && !error && <Spinner label="Finding the best version to play…" />}
      {error && (
        <div className="wx-player-center">
          <p className="wx-error">{error}</p>
          <div className="wx-actions" style={{ justifyContent: 'center' }}>
            <button className="wx-btn" onClick={() => void load(1, true)}>
              Search again
            </button>
            <button className="wx-btn" onClick={() => navigate(-1)}>
              Go back
            </button>
          </div>
        </div>
      )}

      {answer && !error && (
        <video
          key={src}
          ref={video}
          src={src}
          controls
          autoPlay
          playsInline
          onLoadedMetadata={onLoaded}
          onTimeUpdate={onTime}
          onPause={save}
          onEnded={onEnded}
          onError={onError}
        />
      )}

      {next && countdown !== null && (
        <div className="wx-next">
          {next.stillUrl && <img src={next.stillUrl} alt="" />}
          <div className="wx-next-body">
            <small>Next episode in {countdown}s</small>
            <div style={{ fontWeight: 700 }}>
              {epCode(next.season, next.episode)} {next.title}
            </div>
            <div className="wx-next-actions">
              <button className="wx-btn play small" onClick={() => setCountdown(0)}>
                <Icon name="play" size={16} /> Play now
              </button>
              <button className="wx-btn small" onClick={() => setCountdown(null)}>
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
