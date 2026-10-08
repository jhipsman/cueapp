import { useNavigate } from 'react-router-dom'
import { api, profileToken, type LiveRecording } from '../api'
import Icon from '../components/Icon'
import ChannelLogo from './ChannelLogo'
import { recordingSize } from './recordings'
import { savedTheme } from './theme'
import { onTV } from './tv'

function when(iso: string): string {
  const d = new Date(iso)
  const today = new Date()
  const day =
    d.toDateString() === today.toDateString()
      ? 'Today'
      : d.toDateString() === new Date(today.getTime() + 86_400_000).toDateString()
        ? 'Tomorrow'
        : d.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' })
  return `${day} ${d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })}`
}

const STATUS: Record<LiveRecording['status'], string> = {
  scheduled: 'Scheduled',
  recording: 'Recording now',
  done: 'Recorded',
  failed: "Didn't record",
}

// playRecording opens a finished recording: the TV app's own player on a
// TV, Watch's player anywhere else.
export async function playRecording(r: LiveRecording, navigate: (to: string) => void) {
  if (!onTV()) {
    navigate(`/watch/recording/${r.id}`)
    return
  }
  const p = await api.playLiveRecording(r.id)
  window.CueTV?.play(
    JSON.stringify({
      url: new URL(p.url, window.location.href).toString(),
      title: r.title,
      subtitle: `${r.channel} · recorded ${when(r.start)}`,
      startSec: 0,
      kind: 'recording',
      token: profileToken(),
      accent: savedTheme().accent,
    }),
  )
}

// RecordingsList is the Recordings group of Live TV: what's recorded, being
// recorded and coming up, each to play or delete.
export default function RecordingsList({ list, onRemove }: { list: LiveRecording[]; onRemove: (id: number) => void }) {
  const navigate = useNavigate()
  if (list.length === 0) {
    return (
      <div className="wx-recs-empty">
        <Icon name="disc" size={30} />
        <p>
          Nothing recorded yet. Pick a show in the guide and press <b>Record</b>: Cue records it on the server, even when nothing here is open, and keeps it
          for 30 days.
        </p>
        <p className="wx-dim">Recording uses one of your provider's connections while the show is on.</p>
      </div>
    )
  }
  return (
    <div className="wx-recs">
      {list.map((r) => {
        const mins = Math.round((Date.parse(r.stop) - Date.parse(r.start)) / 60_000)
        return (
          <div key={r.id} className={`wx-rec ${r.status}`}>
            <span className="wx-rec-logo">
              <ChannelLogo name={r.channel} src={r.logo} />
            </span>
            <div className="wx-rec-body">
              <b>{r.title}</b>
              <small>
                {r.channel} · {when(r.start)} · {mins} min{r.status === 'done' && r.size ? ` · ${recordingSize(r.size)}` : ''}
              </small>
              <span className={`wx-rec-status ${r.status}`}>
                {r.status === 'recording' && <i />}
                {STATUS[r.status]}
                {r.problem ? `: ${r.problem}` : ''}
              </span>
            </div>
            <div className="wx-rec-actions">
              {r.status === 'done' && (
                <button className="wx-btn play small" onClick={() => void playRecording(r, navigate)}>
                  <Icon name="play" size={14} /> Play
                </button>
              )}
              <button
                className="wx-btn small"
                onClick={() => {
                  const what = r.status === 'scheduled' || r.status === 'recording' ? 'Cancel this recording?' : 'Delete this recording?'
                  if (onTV() || window.confirm(what)) onRemove(r.id)
                }}
              >
                <Icon name={r.status === 'done' || r.status === 'failed' ? 'trash' : 'x'} size={14} />
                {r.status === 'done' || r.status === 'failed' ? 'Delete' : 'Cancel'}
              </button>
            </div>
          </div>
        )
      })}
    </div>
  )
}
