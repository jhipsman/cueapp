import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../api'
import { useAuth } from '../AuthContext'
import { useDarkPage } from '../useDarkPage'
import CueWordmark from '../components/CueWordmark'
import { DOCS_URL } from '../docs'
import { titleFor } from '../documentTitle'
import { required } from '../validate'
import { FieldError, FormProblem, useValidation } from '../useValidation'

export default function Login() {
  const { refresh, streamingOnly } = useAuth()
  useDarkPage(streamingOnly)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    document.title = titleFor('Sign in')
  }, [])

  // Sign-in only checks that both boxes are filled in. The username format is
  // never checked here, so an older account with an unusual name can still get in.
  const v = useValidation({
    username: required(username, 'Enter your username.'),
    password: required(password, 'Enter your password.'),
  })

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!v.attempt()) return
    setError('')
    setBusy(true)
    try {
      await api.login(username, password)
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="auth-page">
      <form className="auth-card" onSubmit={onSubmit}>
        <h1 className="sr-only">Sign in to Cue</h1>
        <div className="auth-logo">
          <CueWordmark className="auth-wordmark" />
        </div>
        <p className="auth-sub">Sign in to continue</p>
        {error && <p className="error-text">{error}</p>}
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoFocus autoComplete="username" {...v.bind('username', username, setUsername)} />
          <FieldError v={v} name="username" />
        </label>
        <label>
          Password
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" {...v.bind('password')} />
          <FieldError v={v} name="password" />
        </label>
        <button className="primary" type="submit" disabled={busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
        <FormProblem v={v} verb="sign in" />
        <details className="forgot">
          <summary>Forgot your password?</summary>
          <p>
            Cue can&apos;t send reset emails. Ask an administrator to set a new password in Settings &gt; Accounts. If you&apos;re the owner, run this on the server that runs Cue:
          </p>
          <code className="forgot-cmd">cd /opt/cue &amp;&amp; docker compose exec -u 1000:1000 cue /app/app reset-password &lt;username&gt;</code>
          <p>
            It asks for a new password and signs that account out everywhere. Not using Docker? See{' '}
            <a href={`${DOCS_URL}/accounts.md`} target="_blank" rel="noreferrer">
              the accounts page
            </a>
            .
          </p>
        </details>
      </form>
    </main>
  )
}
