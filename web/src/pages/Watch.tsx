import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, type PlayAnswer } from '../api'
import Icon from '../components/Icon'
import { qualityText } from '../format'
import { useDocumentTitle } from '../documentTitle'

// Watch: one-click play. The server picks the best release Premiumize can
// stream right away; this page plays it straight from Premiumize. When a
// version won't play in this browser, the next one is tried by itself, so
// there is no list of streams to choose from.
//
// Routes: /watch/movie/:id and /watch/series/:id/:season/:episode
export default function Watch() {
  const params = useParams()
  const navigate = useNavigate()
  const isEpisode = params.season !== undefined
  const id = Number(params.id)
  const season = Number(params.season)
  const episode = Number(params.episode)

  const [answer, setAnswer] = useState<PlayAnswer | null>(null)
  // Which link of the answer is playing: Premiumize's MP4 copy first (it
  // plays in every browser), then the original file.
  const [useOriginal, setUseOriginal] = useState(false)
  const [error, setError] = useState('')
  const [note, setNote] = useState('')
  const loading = useRef(false)
  useDocumentTitle(answer?.title ?? 'Watch')

  const load = useCallback(
    async (option: number, fresh = false) => {
      if (loading.current) return
      loading.current = true
      setError('')
      try {
        const a = isEpisode ? await api.playEpisode(id, season, episode, option, fresh) : await api.playMovie(id, option, fresh)
        setAnswer(a)
        setUseOriginal(!a.streamUrl)
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e))
      } finally {
        loading.current = false
      }
    },
    [isEpisode, id, season, episode],
  )

  useEffect(() => {
    void load(1)
  }, [load])

  // This version won't play here: its original file, then the next version.
  function failed() {
    if (!answer) return
    if (!useOriginal && answer.url) {
      setUseOriginal(true)
      return
    }
    if (answer.option < answer.options) {
      setNote(`Version ${answer.option} wouldn't play in this browser, so the next one is playing.`)
      void load(answer.option + 1)
      return
    }
    setError("None of the versions found will play in this browser. Try the app on your TV, or another browser.")
  }

  const src = answer ? (useOriginal ? answer.url : answer.streamUrl || answer.url) : ''

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
        <button className="btn-with-icon" onClick={() => navigate(-1)}>
          <Icon name="x" size={16} /> Close
        </button>
        <h1 style={{ margin: 0, fontSize: '1.3rem' }}>{answer?.title ?? 'Finding something to play…'}</h1>
      </div>

      {error && <p className="error-text">{error}</p>}
      {!answer && !error && <div className="skeleton" style={{ aspectRatio: '16 / 9', borderRadius: 12 }} />}

      {answer && (
        <>
          <video
            key={src}
            src={src}
            controls
            autoPlay
            playsInline
            onError={failed}
            style={{ width: '100%', maxHeight: '80vh', background: '#000', borderRadius: 12 }}
          />
          {note && <p style={{ color: 'var(--text-dim)', margin: 0 }}>{note}</p>}
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', color: 'var(--text-dim)' }}>
            <span>
              {qualityText(answer.quality)} · version {answer.option} of {answer.options}
            </span>
            <code className="filepath" title={answer.release}>
              {answer.fileName}
            </code>
            {answer.option < answer.options && (
              <button onClick={() => void load(answer.option + 1)}>Try another version</button>
            )}
            <button onClick={() => void load(1, true)}>Search again</button>
          </div>
        </>
      )}
    </div>
  )
}
