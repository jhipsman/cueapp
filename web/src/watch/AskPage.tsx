import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type WatchCard } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { Card, playHref, titleHref } from './parts'
import { useProfiles } from './profiles'

// Ask Cue: say what you're in the mood for, or ask about a film, a show or
// a person. The answer comes with posters you can play; "play Dune" plays it.

type Turn = { role: 'user' | 'assistant'; text: string; heading?: string; cards?: WatchCard[] }

const IDEAS = ['Something funny for tonight', 'A film under 90 minutes', 'Best sci-fi of the 90s', 'What should I watch next?', 'Something the whole family will like', 'Who directed Dune?']

// The browser's own speech recognition, where there is one.
type Recog = { start: () => void; stop: () => void; onresult: ((e: { results: { 0: { transcript: string } }[] }) => void) | null; onend: (() => void) | null; lang: string }
function speech(): (new () => Recog) | null {
  const w = window as unknown as { SpeechRecognition?: new () => Recog; webkitSpeechRecognition?: new () => Recog }
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null
}

export default function AskPage() {
  useDocumentTitle('Ask Cue')
  const navigate = useNavigate()
  const name = useProfiles().active?.name
  const [ready, setReady] = useState<boolean | null>(null)
  const [turns, setTurns] = useState<Turn[]>([])
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [listening, setListening] = useState(false)
  const end = useRef<HTMLDivElement>(null)
  const recog = useRef<Recog | null>(null)
  useEffect(() => {
    api
      .askStatus()
      .then((s) => setReady(s.ready))
      .catch(() => setReady(false))
  }, [])
  useEffect(() => {
    end.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
  }, [turns, busy])

  const send = async (q: string) => {
    const ask = q.trim()
    if (!ask || busy) return
    const next: Turn[] = [...turns, { role: 'user', text: ask }]
    setTurns(next)
    setText('')
    setBusy(true)
    setError('')
    try {
      const a = await api.ask(next.map((t) => ({ role: t.role, text: t.text })))
      setTurns([...next, { role: 'assistant', text: a.reply, heading: a.heading, cards: a.cards }])
      if (a.play) navigate(playHref(a.play.kind, a.play.tmdbId, a.play.season || undefined, a.play.episode || undefined))
      else if (a.open) navigate(titleHref(a.open))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setTurns(turns)
      setText(ask)
    } finally {
      setBusy(false)
    }
  }

  const Speech = speech()
  const listen = () => {
    if (!Speech) return
    if (listening) {
      recog.current?.stop()
      return
    }
    const r = new Speech()
    r.lang = 'en-US'
    r.onresult = (e) => {
      const said = e.results[0]?.[0]?.transcript ?? ''
      setText(said)
      void send(said)
    }
    r.onend = () => setListening(false)
    recog.current = r
    setListening(true)
    r.start()
  }

  return (
    <div className="wx-page wx-ask">
      <div className="wx-ask-head">
        <span className="wx-ask-orb" aria-hidden="true">
          <Icon name="sparkles" size={22} />
        </span>
        <div>
          <h1>Ask Cue</h1>
          <p className="wx-dim">Tell me what you're in the mood for, or ask about any film, show or person.</p>
        </div>
      </div>

      {ready === false && (
        <div className="wx-ask-off glass">
          Ask Cue isn't set up yet. The owner can turn it on in <b>Settings › Movie info</b> with a Claude API key.
        </div>
      )}

      <div className="wx-ask-chat">
        {turns.length === 0 && ready && <p className="wx-ask-hi">Hi{name ? ` ${name}` : ''}! What are we watching?</p>}
        {turns.map((t, i) => (
          <div key={i} className={`wx-ask-turn ${t.role}`}>
            <div className={`wx-ask-bubble${t.role === 'assistant' ? ' glass' : ''}`}>{t.text}</div>
            {t.cards && t.cards.length > 0 && (
              <div className="wx-ask-results">
                {t.heading && <h3>{t.heading}</h3>}
                <div className="wx-grid">
                  {t.cards.map((c) => (
                    <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
                  ))}
                </div>
              </div>
            )}
          </div>
        ))}
        {busy && (
          <div className="wx-ask-turn assistant">
            <div className="wx-ask-bubble glass wx-ask-typing" aria-label="Thinking">
              <i />
              <i />
              <i />
            </div>
          </div>
        )}
        {error && <p className="wx-error">{error}</p>}
        <div ref={end} />
      </div>

      <div className="wx-ask-bar glass">
        <div className="wx-ask-ideas">
          {IDEAS.map((idea) => (
            <button key={idea} disabled={!ready || busy} onClick={() => void send(idea)}>
              {idea}
            </button>
          ))}
        </div>
        <form
          className="wx-ask-input"
          onSubmit={(e) => {
            e.preventDefault()
            void send(text)
          }}
        >
          <input value={text} onChange={(e) => setText(e.target.value)} placeholder="Ask Cue, or tell it what to play" disabled={!ready} maxLength={500} aria-label="Ask Cue" />
          {Speech && (
            <button type="button" className={`wx-ask-mic${listening ? ' on' : ''}`} onClick={listen} disabled={!ready} aria-label={listening ? 'Stop listening' : 'Speak'}>
              <Icon name="mic" size={18} />
            </button>
          )}
          <button type="submit" className="wx-ask-send" disabled={!ready || busy || !text.trim()} aria-label="Send">
            <Icon name="send" size={18} />
          </button>
        </form>
      </div>
    </div>
  )
}
