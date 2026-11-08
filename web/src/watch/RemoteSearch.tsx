import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'

// Searching the TV from a phone: the TV's Search shows a QR code that opens
// this page. What's typed here (or said: the mic button where the browser can
// listen, or the keyboard's own dictation) shows up as the TV's search as you
// go. A Fire TV keeps its remote's microphone for Alexa, so this is how Cue
// hears you there.

interface Recognition {
  lang: string
  interimResults: boolean
  continuous: boolean
  onresult: ((e: { results: ArrayLike<ArrayLike<{ transcript: string }>> }) => void) | null
  onend: (() => void) | null
  onerror: (() => void) | null
  start: () => void
  stop: () => void
}

function recognizer(): Recognition | null {
  const w = window as unknown as { SpeechRecognition?: new () => Recognition; webkitSpeechRecognition?: new () => Recognition }
  const R = w.SpeechRecognition ?? w.webkitSpeechRecognition
  return R ? new R() : null
}

export default function RemoteSearch() {
  const { code = '' } = useParams()
  const [q, setQ] = useState('')
  const [state, setState] = useState<'idle' | 'sending' | 'sent' | 'error'>('idle')
  const [listening, setListening] = useState(false)
  const rec = useRef<Recognition | null>(null)
  const canListen = useRef(recognizer() !== null)
  useDocumentTitle('Search on the TV')

  // Send as you type, a beat after the last key.
  useEffect(() => {
    if (!code) return
    const t = window.setTimeout(() => {
      setState('sending')
      api
        .sendRemoteSearch(code, q)
        .then(() => setState('sent'))
        .catch(() => setState('error'))
    }, 280)
    return () => window.clearTimeout(t)
  }, [code, q])

  const listen = () => {
    if (listening) {
      rec.current?.stop()
      return
    }
    const r = recognizer()
    if (!r) return
    r.lang = navigator.language || 'en-US'
    r.interimResults = true
    r.continuous = false
    r.onresult = (e) => {
      let text = ''
      for (let i = 0; i < e.results.length; i++) text += e.results[i][0].transcript
      setQ(text)
    }
    r.onend = () => setListening(false)
    r.onerror = () => setListening(false)
    rec.current = r
    setListening(true)
    try {
      r.start()
    } catch {
      setListening(false)
    }
  }

  return (
    <div className="wx-remote">
      <p className="wx-remote-kicker">
        <Icon name="tv" size={16} /> Searching on the TV
      </p>
      <h1>What do you want to watch?</h1>
      <div className="wx-remote-field">
        <Icon name="search" size={20} />
        <input
          id="remote-q"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Type a movie or show"
          autoFocus
          autoComplete="off"
          enterKeyHint="search"
          aria-label="Search on the TV"
        />
        {q && (
          <button type="button" className="wx-remote-clear" onClick={() => setQ('')} aria-label="Clear">
            <Icon name="x" size={18} />
          </button>
        )}
      </div>
      {canListen.current && (
        <button type="button" className={`wx-remote-mic${listening ? ' on' : ''}`} onClick={listen}>
          <span className="wx-remote-mic-dot" aria-hidden="true" />
          {listening ? 'Listening… tap to stop' : 'Tap and say it'}
        </button>
      )}
      <p className="wx-remote-note" aria-live="polite">
        {state === 'error'
          ? "Couldn't reach the TV's search. Check you're signed in, then try again."
          : q && state === 'sent'
            ? 'On the TV now. Pick it with the remote.'
            : canListen.current
              ? 'Or tap the mic on your keyboard.'
              : 'Tap the mic on your keyboard to say it.'}
      </p>
    </div>
  )
}
