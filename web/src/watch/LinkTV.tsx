import { useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import { useAuth } from '../AuthContext'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'

// Signing a TV in from a phone: the TV's sign-in shows a code and a QR
// code for this page. Signed in here, one tap signs the TV in to the same
// account; no typing a password on a remote.
export default function LinkTV() {
  const params = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const [code, setCode] = useState((params.code ?? '').toUpperCase())
  const [state, setState] = useState<'idle' | 'busy' | 'done' | 'error'>('idle')
  const [problem, setProblem] = useState('')
  useDocumentTitle('Sign in the TV')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setState('busy')
    setProblem('')
    try {
      await api.approveTVLink(code)
      setState('done')
    } catch (err) {
      setProblem(err instanceof Error ? err.message : String(err))
      setState('error')
    }
  }

  return (
    <div className="wx-remote">
      <p className="wx-remote-kicker">
        <Icon name="tv" size={16} /> Sign in the TV
      </p>
      {state === 'done' ? (
        <>
          <h1>The TV is signed in</h1>
          <p className="wx-remote-note">Pick your profile on the TV and start watching.</p>
          <button type="button" className="wx-remote-mic" onClick={() => navigate('/watch', { replace: true })}>
            Done
          </button>
        </>
      ) : (
        <form onSubmit={submit} style={{ display: 'grid', gap: 18 }}>
          <h1>Sign the TV in as {user?.name || user?.username}?</h1>
          <div className="wx-remote-field">
            <input
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 6))}
              placeholder="Code on the TV"
              autoFocus={!params.code}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              aria-label="The code on the TV"
              style={{ letterSpacing: '0.2em', fontWeight: 700 }}
            />
          </div>
          <button type="submit" className="wx-remote-mic" disabled={code.length !== 6 || state === 'busy'}>
            {state === 'busy' ? 'Signing in…' : 'Sign in the TV'}
          </button>
          <p className="wx-remote-note" aria-live="polite">
            {problem || 'Only do this for a TV in front of you.'}
          </p>
        </form>
      )}
    </div>
  )
}
