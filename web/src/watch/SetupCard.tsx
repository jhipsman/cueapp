import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import Icon from '../components/Icon'
import { useProfiles } from './profiles'

// "Finish setting up", for the owner only, on Watch's home until Cue has
// what it needs: movie info (a TMDB key, for posters and search) and
// somewhere to play from (Premiumize or a stream add-on like Comet). Live
// TV is offered alongside, and the card goes once the two needed ones are in.
export default function SetupCard() {
  const { active, main } = useProfiles()
  const owner = !!active && active.id === main?.id
  const [st, setSt] = useState<{ movieInfo: boolean; streams: boolean; liveTV: boolean } | null>(null)

  useEffect(() => {
    if (!owner) return
    api
      .streamingSetup()
      .then(setSt)
      .catch(() => undefined)
  }, [owner])

  if (!owner || !st || (st.movieInfo && st.streams)) return null
  const steps: { done: boolean; title: string; hint: string; to: string; optional?: boolean }[] = [
    { done: st.movieInfo, title: 'Movie info', hint: 'A free TMDB key: posters, descriptions and search.', to: '/settings/metadata' },
    { done: st.streams, title: 'Where streams come from', hint: 'Your Premiumize key, and Comet for the best streams.', to: '/settings/streaming' },
    { done: st.liveTV, title: 'Live TV', hint: "Your IPTV provider's login, for channels and a TV guide.", to: '/settings/streaming', optional: true },
  ]
  return (
    <section className="wx-setup" aria-label="Finish setting up Cue">
      <h2>Finish setting up</h2>
      <p className="wx-dim">Only you see this. Connect these and everyone can browse and press Play.</p>
      <ol>
        {steps.map((s) => (
          <li key={s.title} className={s.done ? 'done' : ''}>
            <span className="wx-setup-mark">{s.done ? <Icon name="check" size={16} /> : null}</span>
            <div>
              <b>
                {s.title}
                {s.optional && <small> · optional</small>}
              </b>
              <span>{s.hint}</span>
            </div>
            {!s.done && (
              <Link className="wx-btn small" to={s.to}>
                Set up
              </Link>
            )}
          </li>
        ))}
      </ol>
    </section>
  )
}
