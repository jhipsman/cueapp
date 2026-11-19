import { useCallback, useEffect, useState } from 'react'
import { api, type Settings as SettingsData } from '../../api'
import HardcoverCard from '../../components/HardcoverCard'
import OMDbCard from '../../components/OMDbCard'
import ServiceKeyCard from '../../components/ServiceKeyCard'
import { ANTHROPIC_COPY, TMDB_COPY, TRAKT_COPY } from '../../components/serviceCopy'

export default function MetadataSettings() {
  const [s, setS] = useState<SettingsData | null>(null)
  const [error, setError] = useState('')
  const load = useCallback(() => {
    api
      .getSettings()
      .then((data) => {
        setS(data)
        setError('')
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [])
  useEffect(load, [load])

  if (!s) {
    return error ? (
      <div>
        <p className="error-text">{error}</p>
        <button onClick={load}>Try again</button>
      </div>
    ) : (
      <p>Loading…</p>
    )
  }
  return (
    <div className="settings-stack">
      <ServiceKeyCard service="tmdb" {...TMDB_COPY} builtIn={!!s.tmdbKeyBuiltIn} configured={s.hasTmdbApiKey} onSaved={load} />
      <ServiceKeyCard service="trakt" {...TRAKT_COPY} builtIn={!!s.traktClientIdBuiltIn} configured={s.hasTraktClientId} onSaved={load} allowOverride usingOwnKey={!!s.traktUsingOwnKey} />
      <ServiceKeyCard service="anthropic" {...ANTHROPIC_COPY} builtIn={false} configured={!!s.hasAnthropicApiKey} onSaved={load}>
        <fieldset className="ask-model">
          <legend>Which Claude answers</legend>
          {(
            [
              ['haiku', 'Haiku (recommended)', 'Quick and plenty smart for picking something to watch. About a fifth of a cent a question.'],
              ['opus', 'Opus', 'The smartest answers, about 40 times the cost (around 7¢ a question).'],
            ] as const
          ).map(([value, label, hint]) => (
            <label key={value} className="ask-model-choice">
              <input
                type="radio"
                name="askModel"
                checked={(s.askModel ?? 'haiku') === value}
                onChange={() => void api.putSettings({ askModel: value }).then(load)}
              />
              <span>
                <b>{label}</b>
                <small>{hint}</small>
              </span>
            </label>
          ))}
        </fieldset>
      </ServiceKeyCard>
      <OMDbCard />
      <HardcoverCard />
    </div>
  )
}
