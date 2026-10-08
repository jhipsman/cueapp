import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, profileToken, type PlayAnswer, type WatchEpisode } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { epCode, playHref, Spinner } from './parts'
import { savedTheme } from './theme'
import { onTV, type TVPlayerResult } from './tv'
import VideoControls from './VideoControls'
import FreeElsewhere from './FreeElsewhere'
import YouTubePlayer from './YouTubePlayer'

// How often where you are is saved while playing, and how long the "Next
// episode" card counts down before playing it.
const SAVE_EVERY_MS = 15_000
const NEXT_COUNTDOWN = 10

// The player: full screen, starts by itself. It asks the server for the best
// version Premiumize can stream, resumes where you stopped, saves where you
// are, quietly moves to the next version if one won't play, and offers the
// next episode at the end.
// Phones and tablets can hand a video to a player app, which plays what the
// browser can't. iPhone: Infuse or VLC; Android: VLC.
const UA = typeof navigator === 'undefined' ? '' : navigator.userAgent
const IOS = /iPhone|iPad|iPod/.test(UA) || (/Macintosh/.test(UA) && typeof navigator !== 'undefined' && navigator.maxTouchPoints > 1)
const ANDROID = /Android/.test(UA)

function playerApps(url: string): { name: string; href: string }[] {
  if (!url || onTV()) return []
  const u = encodeURIComponent(url)
  if (IOS) {
    return [
      { name: 'Infuse', href: `infuse://x-callback-url/play?url=${u}` },
      { name: 'VLC', href: `vlc-x-callback://x-callback-url/stream?url=${u}` },
    ]
  }
  if (ANDROID) {
    const [scheme, rest] = url.split('://')
    return [{ name: 'VLC', href: `intent://${rest}#Intent;scheme=${scheme};package=org.videolan.vlc;type=video/*;end` }]
  }
  return []
}

function OpenIn({ url, onOpen }: { url: string; onOpen: () => void }) {
  const apps = playerApps(url)
  if (apps.length === 0) return null
  return (
    <div className="wx-open-in">
      {apps.map((a) => (
        <a key={a.name} className="wx-btn small" href={a.href} onClick={onOpen}>
          Open in {a.name}
        </a>
      ))}
    </div>
  )
}

// CopyLink copies the video's address, to open in VLC on a computer (Media
// > Open Network Stream), which plays every format.
function CopyLink({ url }: { url: string }) {
  const [done, setDone] = useState(false)
  if (!url) return null
  return (
    <div className="wx-open-in">
      <button
        className="wx-btn small"
        onClick={() => {
          void navigator.clipboard
            ?.writeText(url)
            .then(() => setDone(true))
            .catch(() => window.prompt('Copy this link into VLC (Media > Open Network Stream):', url))
        }}
      >
        <Icon name={done ? 'check' : 'external'} size={14} /> {done ? 'Copied: paste it in VLC' : 'Copy link for VLC'}
      </button>
    </div>
  )
}

export default function PlayerPage() {
  const params = useParams()
  const navigate = useNavigate()
  const kind = params.season !== undefined ? 'tv' : 'movie'
  const tmdbId = Number(params.tmdbId)
  const season = Number(params.season ?? 0)
  const episode = Number(params.episode ?? 0)

  const video = useRef<HTMLVideoElement | null>(null)
  const [videoEl, setVideoEl] = useState<HTMLVideoElement | null>(null)
  const videoRef = useCallback((el: HTMLVideoElement | null) => {
    video.current = el
    setVideoEl(el)
  }, [])
  const [answer, setAnswer] = useState<PlayAnswer | null>(null)
  const [useOriginal, setUseOriginal] = useState(false)
  const [error, setError] = useState('')
  const [note, setNote] = useState('')
  const [next, setNext] = useState<WatchEpisode | null>(null)
  const [countdown, setCountdown] = useState<number | null>(null)
  const resumeAt = useRef(0)
  // How many times this title moved on by itself because a version had no
  // sound; it stops after a couple, so it never runs through every version.
  const silentSkips = useRef(0)
  const [resumeReady, setResumeReady] = useState(false)
  const loading = useRef(false)
  useDocumentTitle(answer?.title ?? 'Playing')

  const load = useCallback(
    async (option: number, fresh = false) => {
      if (loading.current) return
      loading.current = true
      setError('')
      try {
        const a = await api.playTMDB(kind, tmdbId, season, episode, option, fresh, !onTV())
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
    silentSkips.current = 0
    setResumeReady(false)
    api
      .watchProgress(kind, tmdbId, season, episode)
      .then((p) => {
        if (!p.finished && p.position > 30) resumeAt.current = p.position
      })
      .catch(() => undefined)
      .finally(() => setResumeReady(true))
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

  // A minute in, look the next episode up, so it starts at once too.
  useEffect(() => {
    if (kind !== 'tv' || !answer) return
    const t = setTimeout(() => {
      api
        .watchNext(tmdbId, season, episode)
        .then((n) => api.prefetchPlay('tv', tmdbId, n.season, n.episode))
        .catch(() => undefined)
    }, 60_000)
    return () => clearTimeout(t)
  }, [kind, tmdbId, season, episode, answer])

  // On a TV, the app's own player plays it (every format, Dolby and DTS
  // sound) and saves where you are; this page waits and acts on how it ended.
  useEffect(() => {
    if (!onTV() || !answer || answer.youtube || !resumeReady || error) return
    window.cueTvPlayerDone = (r: TVPlayerResult) => {
      window.cueTvPlayerDone = undefined
      if (r.reason === 'ended' || r.reason === 'next') {
        if (kind !== 'tv') return navigate(-1)
        api
          .watchNext(tmdbId, season, episode)
          .then((n) => navigate(playHref('tv', tmdbId, n.season, n.episode), { replace: true }))
          .catch(() => navigate(-1))
        return
      }
      if (r.reason === 'other') {
        resumeAt.current = r.position
        setNote(`Trying version ${answer.option + 1} of ${answer.options}.`)
        void load(answer.option + 1)
        return
      }
      if (r.reason === 'nosound') {
        // This version's sound can't play on this TV: the next one, from the
        // same point, a couple of times at most.
        resumeAt.current = r.position
        if (answer.option < answer.options && silentSkips.current < 2) {
          silentSkips.current++
          setNote(`Version ${answer.option} had no sound on this TV, so trying the next one.`)
          void load(answer.option + 1)
        } else {
          setError('The versions found have sound this TV can\'t play. Try again later for others.')
        }
        return
      }
      if (r.reason === 'error') {
        if (answer.option < answer.options) {
          setNote(`Version ${answer.option} wouldn't play, so trying the next one.`)
          void load(answer.option + 1)
        } else {
          setError(`None of the versions found would play.${r.message ? ' ' + r.message : ''}`)
        }
        return
      }
      navigate(-1)
    }
    const label = kind === 'tv' ? epCode(season, episode) : ''
    window.CueTV?.play(
      JSON.stringify({
        url: answer.url,
        title: answer.title,
        subtitle: [label, answer.quality, answer.source && `via ${answer.source}`].filter(Boolean).join(' · '),
        startSec: resumeAt.current,
        hasOther: answer.option < answer.options,
        kind,
        tmdbId,
        season,
        episode,
        token: profileToken(),
        accent: savedTheme().accent, // the profile's color, for the player
      }),
    )
  }, [answer, resumeReady, error, kind, tmdbId, season, episode, load, navigate])

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

  // Some versions carry sound a browser can't play (Dolby, DTS, TrueHD):
  // the picture plays in silence. A few seconds in, if no sound has been
  // decoded, move on to the next version from the same point.
  useEffect(() => {
    if (!answer || onTV()) return
    // Checked once the video has played a few seconds (it may buffer first).
    const t = setInterval(() => {
      const v = video.current as (HTMLVideoElement & { webkitAudioDecodedByteCount?: number; mozHasAudio?: boolean; audioTracks?: { length: number } }) | null
      if (!v || v.paused || v.currentTime < 4) return
      clearInterval(t)
      const silent =
        v.webkitAudioDecodedByteCount === 0 || v.mozHasAudio === false || (v.audioTracks !== undefined && v.audioTracks.length === 0)
      if (!silent) return
      if (answer.option < answer.options && silentSkips.current < 2) {
        silentSkips.current++
        resumeAt.current = Math.floor(v.currentTime)
        setNote(`Version ${answer.option} had no sound in this browser, so trying the next one.`)
        void load(answer.option + 1)
      } else {
        setNote('No sound in this browser: its sound is Dolby or DTS. Pick another in Versions, or watch on the TV app.')
      }
    }, 2000)
    return () => clearInterval(t)
  }, [answer, useOriginal, load])

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
    setError(
      playerApps(answer.url).length > 0
        ? 'None of the versions found will play in this browser. Open it in a player app instead:'
        : 'None of the versions found will play in this browser (their video or sound is in a format browsers don’t open). Watch it on the TV app, or copy the link into VLC:',
    )
  }

  const src = answer ? (useOriginal ? answer.url : answer.streamUrl || answer.url) : ''
  const nextEpisode = () =>
    api
      .watchNext(tmdbId, season, episode)
      .then((n) => navigate(playHref('tv', tmdbId, n.season, n.episode), { replace: true }))
      .catch(() => setNote('That was the latest episode.'))

  const yt = answer && !error ? answer.youtube : undefined
  const playing = answer && !error && !onTV() && !yt
  return (
    <div className="wx-player">
      {!playing && !yt && (
        <div className="wx-player-top">
          <button className="wx-round" onClick={() => navigate(-1)} aria-label="Back">
            <Icon name="x" size={20} />
          </button>
          <div className="wx-player-title">
            {answer?.title ?? 'Finding the best version…'}
            {note && <small>{note}</small>}
          </div>
        </div>
      )}

      {!answer && !error && <Spinner label="Finding the best version to play…" />}
      {error && (
        <div className="wx-player-center">
          <p className="wx-error">{error}</p>
          <FreeElsewhere kind={kind} tmdbId={tmdbId} />
          {answer && <OpenIn url={answer.url} onOpen={() => undefined} />}
          {answer && !answer.youtube && playerApps(answer.url).length === 0 && !onTV() && <CopyLink url={answer.url} />}
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

      {answer && !error && onTV() && !yt && <Spinner label="Starting the player…" />}
      {answer && yt && (
        <YouTubePlayer
          id={yt}
          title={answer.title}
          note={note || `From YouTube · ${answer.release}`}
          startSec={resumeAt.current}
          onBack={() => navigate(-1)}
          onEnded={() => {
            if (kind === 'tv') void nextEpisode()
          }}
          onError={() => onError()}
          onNext={kind === 'tv' ? () => void nextEpisode() : undefined}
          onOther={answer.option < answer.options ? () => void load(answer.option + 1) : undefined}
        />
      )}
      {playing && (
        <>
          <video
            key={src}
            ref={videoRef}
            src={src}
            autoPlay
            playsInline
            onLoadedMetadata={onLoaded}
            onTimeUpdate={onTime}
            onPause={save}
            onEnded={onEnded}
            onError={onError}
          />
          <VideoControls
            video={videoEl}
            title={answer.title}
            subtitle={answer.quality}
            onBack={() => navigate(-1)}
            note={note}
            actions={
              kind === 'tv' ? (
                <button className="vx-text-btn" onClick={() => void nextEpisode()} aria-label="Next episode">
                  <Icon name="skip-forward" size={22} fill="currentColor" />
                  <span>Next episode</span>
                </button>
              ) : undefined
            }
            menu={{
              label: 'Versions',
              icon: 'layers',
              heading: `Version ${answer.option} of ${answer.options}`,
              items: [
                {
                  label: [answer.quality, answer.source && `via ${answer.source}`].filter(Boolean).join(' · ') || 'This version',
                  hint: answer.release,
                  disabled: true,
                },
                {
                  label: 'Try another version',
                  hint: answer.option < answer.options ? `Version ${answer.option + 1} of ${answer.options}` : 'This is the last one found',
                  disabled: answer.option >= answer.options,
                  onClick: () => {
                    resumeAt.current = Math.floor(video.current?.currentTime ?? 0)
                    void load(answer.option + 1)
                  },
                },
                ...playerApps(answer.url).map((a) => ({
                  label: `Open in ${a.name}`,
                  hint: 'Plays every kind of sound',
                  href: a.href,
                  onClick: () => video.current?.pause(),
                })),
                { label: 'Search again', hint: 'Look for versions afresh', onClick: () => void load(1, true) },
              ],
            }}
          />
        </>
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
