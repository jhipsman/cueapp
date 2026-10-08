import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, profileToken, type PlayAnswer, type WatchEpisode, type WatchSkips } from '../api'
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

// skipButton is the skip button to show at pos: during the recap or the
// intro (not in its last two seconds).
function skipButton(skips: WatchSkips | null, pos: number): { label: string; to: number } | null {
  if (!skips) return null
  if (skips.recap && pos >= skips.recap.start && pos < skips.recap.end - 2) return { label: 'Skip recap', to: skips.recap.end }
  if (skips.intro && pos >= skips.intro.start && pos < skips.intro.end - 2) return { label: 'Skip intro', to: skips.intro.end }
  return null
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
  // The file converted by Cue as it plays, for a browser that can't open it:
  // where in the video the conversion started, and how long the video is.
  const [conv, setConv] = useState<{ stream: string; duration: number; offset: number } | null>(null)
  const converting = useRef(false)
  // Asked once past the last version: Cue then looks on the Internet
  // Archive and YouTube.
  const askedBeyond = useRef(false)
  // Skip intro and the credits (TheIntroDB, or learned from skips here):
  // where the picture is (seconds into the title), and the video's length.
  const [skips, setSkips] = useState<WatchSkips | null>(null)
  const [pos, setPos] = useState(0)
  const [length, setLength] = useState(0)
  const lastPos = useRef(0)
  const ownSeek = useRef(false) // a jump Cue made (resume, a skip button), not the viewer
  const jump = useRef<{ from: number; to: number; timer: number } | null>(null)
  const creditsDone = useRef(false)
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
        setConv(null)
        setSkips(null)
        setLength(0)
        creditsDone.current = false
        converting.current = false
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
    askedBeyond.current = false
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
    const at = (conv?.offset ?? 0) + (v?.currentTime ?? 0)
    const length = conv ? conv.duration : (v?.duration ?? NaN)
    if (!v || !answer || !isFinite(length) || length <= 0 || at < 5) return
    void api
      .saveWatchProgress({ kind, tmdbId, season, episode, position: Math.floor(at), duration: Math.floor(length) })
      .catch(() => undefined)
  }, [answer, conv, kind, tmdbId, season, episode])

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
        if (answer.option < answer.options || !askedBeyond.current) {
          if (answer.option >= answer.options) askedBeyond.current = true
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
    let cancelled = false
    // The skip times go with it (waited for briefly; the player starts
    // without them if TheIntroDB is slow).
    const skipsSoon = Promise.race([
      api.watchSkips(kind, tmdbId, season, episode, 0).catch(() => ({}) as WatchSkips),
      new Promise<WatchSkips>((done) => setTimeout(() => done({}), 4500)),
    ])
    void skipsSoon.then((sk) => {
      if (cancelled) return
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
          introStart: sk.intro?.start ?? 0,
          introEnd: sk.intro?.end ?? 0,
          recapStart: sk.recap?.start ?? 0,
          recapEnd: sk.recap?.end ?? 0,
          creditsStart: sk.creditsStart ?? 0,
          creditsFromEnd: sk.creditsFromEnd ?? 0,
          introKnown: !!sk.source?.includes('theintrodb') && !!sk.intro,
        }),
      )
    })
    return () => {
      cancelled = true
    }
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
      // Cue can convert the sound into one the browser plays, from here.
      resumeAt.current = Math.floor((conv?.offset ?? 0) + v.currentTime)
      if (startConvert('No sound in this browser, so Cue is converting it…')) return
      if (answer.option < answer.options && silentSkips.current < 2) {
        silentSkips.current++
        resumeAt.current = Math.floor((conv?.offset ?? 0) + v.currentTime)
        setNote(`Version ${answer.option} had no sound in this browser, so trying the next one.`)
        void load(answer.option + 1)
      } else {
        setNote('No sound in this browser: its sound is Dolby or DTS. Pick another in Versions, or watch on the TV app.')
      }
    }, 2000)
    return () => clearInterval(t)
  }, [answer, useOriginal, conv, load])

  // The skip times, once the video's length is known (it picks the cut).
  useEffect(() => {
    if (!answer || answer.youtube || onTV() || length <= 0 || skips) return
    let live = true
    api
      .watchSkips(kind, tmdbId, season, episode, length)
      .then((s) => live && setSkips(s))
      .catch(() => live && setSkips({}))
    return () => {
      live = false
    }
  }, [answer, length, skips, kind, tmdbId, season, episode])

  // seekAbs moves to a point in the title (also in a converted video).
  function seekAbs(t: number) {
    ownSeek.current = true
    if (conv?.stream) setConv({ ...conv, offset: Math.max(0, Math.floor(t)) })
    else if (video.current) video.current.currentTime = t
  }

  // noteJump gathers the viewer's jumps forward (several presses of +10s
  // count as one); a jump over the start of an episode teaches Cue its
  // intro, unless TheIntroDB already knows it.
  function noteJump(from: number, to: number) {
    if (kind !== 'tv' || to <= from) return
    const j = jump.current
    if (j && Math.abs(from - j.to) < 4) {
      window.clearTimeout(j.timer)
      j.to = to
    } else {
      if (j) window.clearTimeout(j.timer)
      jump.current = { from, to, timer: 0 }
    }
    const cur = jump.current!
    cur.timer = window.setTimeout(() => {
      jump.current = null
      if (skips?.source?.includes('theintrodb') && skips.intro) return
      if (cur.from < 480 && cur.to - cur.from >= 15 && cur.to - cur.from <= 200) {
        void api.learnSkip({ kind, tmdbId, season, segment: 'intro', start: Math.round(cur.from), end: Math.round(cur.to) }).catch(() => undefined)
        setSkips((s) => (s && !s.intro ? { ...s, intro: { start: cur.from, end: cur.to } } : s))
      }
    }, 4000)
  }

  function onSeeking() {
    const v = video.current
    if (!v) return
    const to = (conv?.offset ?? 0) + v.currentTime
    if (ownSeek.current) {
      ownSeek.current = false
    } else {
      noteJump(lastPos.current, to)
    }
    lastPos.current = to
  }

  function onLoaded() {
    const v = video.current
    setLength(conv?.stream ? conv.duration : v && isFinite(v.duration) ? v.duration : 0)
    if (conv) {
      resumeAt.current = 0 // the conversion started there already
      return
    }
    if (v && resumeAt.current > 0 && resumeAt.current < v.duration - 60) {
      ownSeek.current = true
      v.currentTime = resumeAt.current
    }
    resumeAt.current = 0
  }

  // Near the end of an episode, find the next one (once).
  function onTime() {
    const v = video.current
    if (v) {
      const at = (conv?.offset ?? 0) + v.currentTime
      lastPos.current = at
      setPos(at)
      // The credits: the next episode's card, counting down.
      const creditsAt = skips?.creditsStart || (skips?.creditsFromEnd && length ? length - skips.creditsFromEnd : 0)
      if (kind === 'tv' && creditsAt > 0 && at >= creditsAt && !creditsDone.current) {
        creditsDone.current = true
        api
          .watchNext(tmdbId, season, episode)
          .then((n) => {
            setNext(n)
            setCountdown(NEXT_COUNTDOWN)
          })
          .catch(() => undefined)
      }
    }
    const total = conv ? conv.duration : v?.duration
    if (!v || kind !== 'tv' || next || !total || !isFinite(total)) return
    if (total - ((conv?.offset ?? 0) + v.currentTime) < 60) {
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
    // The browser can't open this file: Cue converts it as it plays.
    if (!startConvert('Converting this version so this browser can play it…')) moveOn()
  }

  // startConvert has Cue convert this version as it plays (from
  // resumeAt), once; false when it can't or already did.
  function startConvert(why: string): boolean {
    if (!answer?.convert || conv || converting.current) return false
    converting.current = true
    setNote(why)
    api
      .convertProbe(answer.convert)
      .then((p) => setConv({ stream: p.stream, duration: p.duration, offset: Math.max(0, Math.floor(resumeAt.current)) }))
      .catch(() => moveOn())
    return true
  }

  // The next version, or nothing more to try.
  function moveOn() {
    if (!answer) return
    if (answer.option < answer.options || !askedBeyond.current) {
      if (answer.option >= answer.options) askedBeyond.current = true
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

  const src = answer
    ? conv?.stream
      ? `${conv.stream}&t=${conv.offset}`
      : useOriginal
        ? answer.url
        : answer.streamUrl || answer.url
    : ''
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
            onSeeking={onSeeking}
            onPause={save}
            onEnded={onEnded}
            onError={onError}
          />
          <VideoControls
            video={videoEl}
            title={answer.title}
            subtitle={[answer.quality, conv?.stream && 'converted for this browser'].filter(Boolean).join(' · ')}
            timeline={
              conv?.stream && conv.duration > 0
                ? {
                    offset: conv.offset,
                    duration: conv.duration,
                    onSeek: (sec) => {
                      if (ownSeek.current) ownSeek.current = false
                      else noteJump(lastPos.current, sec)
                      lastPos.current = sec
                      ownSeek.current = true // the reload that follows isn't the viewer's
                      setConv((c) => (c ? { ...c, offset: Math.max(0, Math.floor(sec)) } : c))
                    },
                  }
                : undefined
            }
            onBack={() => navigate(-1)}
            note={note}
            actions={
              kind === 'tv' ? (
                <button
                  className="vx-text-btn"
                  onClick={() => {
                    // Next pressed near the end: where the credits start.
                    if (length > 0 && !skips?.creditsStart && length - pos >= 15 && length - pos <= 900 && pos > length * 0.7) {
                      void api.learnSkip({ kind, tmdbId, season, segment: 'credits', start: Math.round(pos), duration: Math.round(length) }).catch(() => undefined)
                    }
                    void nextEpisode()
                  }}
                  aria-label="Next episode"
                >
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
                ...(kind === 'tv'
                  ? [
                      {
                        label: skips?.intro || skips?.recap ? 'Skip intro is on' : 'Skip intro: no times yet',
                        hint: !skips
                          ? 'Looking them up…'
                          : skips.source?.includes('theintrodb')
                            ? 'Times from TheIntroDB'
                            : skips.source === 'learned'
                              ? 'Learned from a skip in this show'
                              : 'Jump over the intro once and the rest of the show gets the button',
                        disabled: true,
                      },
                    ]
                  : []),
                {
                  label: 'Try another version',
                  hint: answer.option < answer.options ? `Version ${answer.option + 1} of ${answer.options}` : 'This is the last one found',
                  disabled: answer.option >= answer.options,
                  onClick: () => {
                    resumeAt.current = Math.floor((conv?.offset ?? 0) + (video.current?.currentTime ?? 0))
                    void load(answer.option + 1)
                  },
                },
                ...(answer.convert && !conv?.stream
                  ? [
                      {
                        label: 'No sound? Fix it',
                        hint: 'Cue converts the sound so this browser can play it',
                        onClick: () => {
                          resumeAt.current = Math.floor(video.current?.currentTime ?? 0)
                          startConvert('Converting the sound for this browser…')
                        },
                      },
                    ]
                  : []),
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

      {playing && skipButton(skips, pos) && (
        <button
          className="wx-skip"
          onClick={() => {
            const b = skipButton(skips, pos)
            if (b) seekAbs(b.to)
          }}
        >
          {skipButton(skips, pos)?.label} <Icon name="skip-forward" size={18} fill="currentColor" />
        </button>
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
