import { useEffect, useState, type FormEvent } from 'react'
import { api } from '../api'
import { useAuth } from '../AuthContext'
import { useDarkPage } from '../useDarkPage'
import BrandMark from '../components/BrandMark'
import PasswordStrength from '../components/PasswordStrength'
import { titleFor } from '../documentTitle'
import { FieldError, FormProblem, useValidation } from '../useValidation'
import * as check from '../validate'

// The first run of Cue as a family streaming app: one card, like signing
// in, that creates the owner's account and opens Watch. Everything else
// (Premiumize, Comet, the movie info key, Live TV) is in Settings > Streaming,
// and Watch says what is still missing.
export default function Welcome() {
  const { refresh } = useAuth()
  useDarkPage(true)
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [setupCode, setSetupCode] = useState('')
  const [codeRequired, setCodeRequired] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    document.title = titleFor('Welcome')
    api
      .onboardingStatus()
      .then((s) => setCodeRequired(!!s.setupCodeRequired))
      .catch(() => undefined)
  }, [])

  const v = useValidation({
    name: check.maxLength(name, 100, 'Your name'),
    username: check.firstError(check.required(username, 'Choose a username to sign in with, at least 3 characters.'), check.username(username)),
    password: check.firstError(check.required(password, 'Choose a password of at least 8 characters.'), check.password(password)),
    setupCode: codeRequired && !setupCode.trim() ? 'Enter the setup code from your server.' : null,
  })

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!v.attempt()) return
    setError('')
    setBusy(true)
    try {
      await api.createAdmin({ username, password, name: name.trim() || undefined, setupCode: setupCode.trim() || undefined })
      await api.putSettings({ legalAcknowledged: true, onboardingDone: true })
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setBusy(false)
    }
  }

  return (
    <main className="auth-page">
      <form className="auth-card" onSubmit={onSubmit}>
        <h1 className="sr-only">Create your Cue account</h1>
        <div className="auth-logo">
          <BrandMark className="auth-mark" />
          <span>Cue</span>
        </div>
        <p className="auth-sub">Create your account. You&apos;ll be the owner: you add your family and change the settings.</p>
        {error && <p className="error-text">{error}</p>}
        <label>
          Your name
          <input value={name} onChange={(e) => setName(e.target.value)} autoComplete="given-name" autoFocus {...v.bind('name', name, setName)} />
          <FieldError v={v} name="name" />
        </label>
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoCapitalize="none" spellCheck={false} {...v.bind('username', username, setUsername)} />
          <FieldError v={v} name="username" />
        </label>
        <label>
          Password
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" {...v.bind('password')} />
          <FieldError v={v} name="password" />
          {password && <PasswordStrength password={password} />}
        </label>
        {codeRequired && (
          <label>
            Setup code
            <input
              value={setupCode}
              onChange={(e) => setSetupCode(e.target.value.toUpperCase())}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              placeholder="ABCD-EFGH-JKLM"
              {...v.bind('setupCode', setupCode, setSetupCode)}
            />
            <FieldError v={v} name="setupCode" />
            <small className="field-hint">
              So no one else can claim your server. The installer showed it; or on the server run{' '}
              <code>cd /opt/cue &amp;&amp; docker compose logs cue | grep setup_code</code>
            </small>
          </label>
        )}
        <button className="primary" type="submit" disabled={busy}>
          {busy ? 'Creating your account…' : 'Create account'}
        </button>
        <FormProblem v={v} verb="create your account" />
        <small className="field-hint">For your own movies, shows and channels, and the services you pay for.</small>
      </form>
    </main>
  )
}
