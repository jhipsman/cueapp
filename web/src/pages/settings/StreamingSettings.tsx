import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type StreamingSettings as Streaming } from '../../api'
import Icon from '../../components/Icon'
import PremiumizeCard from '../../components/PremiumizeCard'
import StreamAddonsCard from '../../components/StreamAddonsCard'
import Switch from '../../components/Switch'
import { useToast } from '../../components/Toast'
import { useModules } from '../../ModulesContext'

// Settings > Streaming: everything Cue needs as a family streaming app, on
// one page. Premiumize, the stream add-ons (Comet...), how good a picture to
// prefer, and the switch between streaming only and the full library manager.
export default function StreamingSettings() {
  const toast = useToast()
  const { refresh: refreshModules } = useModules()
  const [st, setSt] = useState<Streaming | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api
      .streamingSettings()
      .then(setSt)
      .catch(() => setSt({ streamingOnly: true, maxResolution: '' }))
  }, [])

  async function save(body: Partial<Streaming>, done: string) {
    setBusy(true)
    try {
      setSt(await api.putStreamingSettings(body))
      toast.success(done)
      if (body.streamingOnly !== undefined) await refreshModules()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const qualities: [Streaming['maxResolution'], string, string][] = [
    ['', 'Best there is', '4K when there is one'],
    ['1080', 'Up to 1080p', 'Full HD; smaller files'],
    ['720', 'Up to 720p', 'Data saver, for hotspots'],
  ]

  return (
    <div className="settings-stack">
      <fieldset className="group folders span-all">
        <legend>
          <Icon name="play" size={14} /> Where streams come from
        </legend>
        <div className="stack-cols">
          <PremiumizeCard />
          <StreamAddonsCard />
        </div>
        <p style={{ color: 'var(--text-dim)', margin: '12px 0 0' }}>
          When no add-on has a stream, Cue searches your own <Link to="/settings/indexers">torrent sites</Link> and checks them with Premiumize.
        </p>
      </fieldset>

      <div className="half-cols span-all">
        <fieldset className="group folders">
          <legend>
            <Icon name="star" size={14} /> Picture quality
          </legend>
          <p style={{ color: 'var(--text-dim)', marginTop: 0 }}>
            Which versions Play starts with. Bigger ones are still there, at the end of &quot;Try another version&quot;.
          </p>
          <div className="seg" role="radiogroup" aria-label="Picture quality">
            {qualities.map(([value, label, hint]) => (
              <button
                key={value || 'best'}
                role="radio"
                aria-checked={st?.maxResolution === value}
                className={st?.maxResolution === value ? 'active' : ''}
                disabled={!st || busy}
                title={hint}
                onClick={() => void save({ maxResolution: value }, `Saved: ${label.toLowerCase()}.`)}
              >
                {label}
              </button>
            ))}
          </div>
        </fieldset>

        <fieldset className="group folders">
          <legend>
            <Icon name="user" size={14} /> Family
          </legend>
          <p style={{ color: 'var(--text-dim)', marginTop: 0 }}>
            Profiles, PINs and who&apos;s watching are under your avatar in Watch.
          </p>
          <Link className="btn-with-icon" to="/watch/profiles" style={{ display: 'inline-flex' }}>
            <Icon name="user" size={16} /> Manage profiles
          </Link>
        </fieldset>
      </div>

      <fieldset className="group folders span-all">
        <legend>
          <Icon name="sliders" size={14} /> Streaming only
        </legend>
        <Switch
          checked={st?.streamingOnly ?? true}
          disabled={!st || busy}
          onChange={(v) =>
            void save(
              { streamingOnly: v },
              v ? 'Streaming only: Watch is the app.' : 'The full library manager is back: downloads, Usenet, music and books.',
            )
          }
          label="Streaming only"
          description="Cue opens on Watch, and the library manager (downloading to this computer, Usenet, the torrent client, music, books, media servers) is hidden and its automatic searches rest. Nothing is deleted: switch this off to bring it all back."
          showState
        />
      </fieldset>
    </div>
  )
}
