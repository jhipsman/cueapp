import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, type ReportProblem, type WatchKind, type WatchReport } from '../api'
import Icon from '../components/Icon'
import { useDocumentTitle } from '../documentTitle'
import { epCode, shortDate, titleHref } from './parts'

// Report a problem: anyone watching says what's wrong in two presses, the
// owner sees it (and gets an alert), fixes it or says why not, and whoever
// reported it hears back.

export const PROBLEMS: { key: ReportProblem; label: string; hint: string }[] = [
  { key: 'sound', label: 'Sound', hint: 'None, out of step, or too quiet' },
  { key: 'subtitles', label: 'Subtitles', hint: 'Missing or out of step' },
  { key: 'picture', label: 'Picture', hint: 'Blurry, frozen or squashed' },
  { key: 'wrong', label: 'Wrong video', hint: 'Not this title, or the wrong episode' },
  { key: 'buffering', label: 'Keeps stopping', hint: 'Pauses to load again and again' },
  { key: 'other', label: 'Something else', hint: 'Say what in a few words' },
]

const problemLabel = (p: ReportProblem) => PROBLEMS.find((x) => x.key === p)?.label ?? p

// ReportSheet is the player's "Report a problem": pick what's wrong, add a
// word if you like, send.
export function ReportSheet({
  of,
  onClose,
}: {
  of: { kind: WatchKind; tmdbId: number; season?: number; episode?: number; title: string; release?: string }
  onClose: (sent: boolean) => void
}) {
  const [problem, setProblem] = useState<ReportProblem | null>(null)
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose(false)
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose])
  const send = async () => {
    if (!problem) return
    setBusy(true)
    setError('')
    try {
      await api.addReport({ ...of, problem, note: note.trim() })
      onClose(true)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setBusy(false)
    }
  }
  return (
    <div className="wx-report-back" role="dialog" aria-modal="true" aria-labelledby="wx-report-title" onClick={(e) => e.target === e.currentTarget && onClose(false)}>
      <div className="wx-report">
        <h2 id="wx-report-title">What’s wrong?</h2>
        <p className="wx-dim">
          {of.title}
          {of.episode ? ` · ${epCode(of.season, of.episode)}` : ''}. The owner hears about it and you’ll hear back.
        </p>
        <div className="wx-report-grid">
          {PROBLEMS.map((p) => (
            <button key={p.key} className={`wx-report-pick${problem === p.key ? ' on' : ''}`} onClick={() => setProblem(p.key)} aria-pressed={problem === p.key}>
              <strong>{p.label}</strong>
              <small>{p.hint}</small>
            </button>
          ))}
        </div>
        <input
          className="wx-report-note"
          placeholder={problem === 'other' ? 'What’s wrong?' : 'Anything else? (optional)'}
          value={note}
          maxLength={500}
          onChange={(e) => setNote(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && void send()}
        />
        {error && <p className="wx-error">{error}</p>}
        <div className="wx-actions">
          <button className="wx-btn play" disabled={!problem || busy} onClick={() => void send()}>
            {busy ? 'Sending…' : 'Send'}
          </button>
          <button className="wx-btn" onClick={() => onClose(false)}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}

// useOpenReports is how many reports are waiting on the owner (0 for
// everyone else), for the avatar's badge.
export function useOpenReports(owner: boolean): number {
  const [n, setN] = useState(0)
  useEffect(() => {
    if (!owner) return
    let live = true
    api
      .reports()
      .then((r) => live && setN(r.owner ? r.open : 0))
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [owner])
  return n
}

// ReportNews is a banner on Home when a problem you reported has been
// answered (shown once).
export function ReportNews() {
  const [news, setNews] = useState<WatchReport[]>([])
  const navigate = useNavigate()
  useEffect(() => {
    let live = true
    api
      .reportNews()
      .then((n) => live && setNews(n ?? []))
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [])
  if (news.length === 0) return null
  return (
    <div className="wx-news">
      {news.map((r) => (
        <div key={r.id} className={`wx-news-item ${r.status}`}>
          <Icon name={r.status === 'fixed' ? 'check' : 'info'} size={20} />
          <div>
            <strong>
              {r.title}: {r.status === 'fixed' ? 'fixed' : 'couldn’t be fixed'}
            </strong>
            <span>{r.reply || `About the problem you reported (${problemLabel(r.problem).toLowerCase()}).`}</span>
          </div>
          {r.status === 'fixed' && (
            <button className="wx-btn small play" onClick={() => navigate(titleHref(r))}>
              Watch
            </button>
          )}
          <button className="wx-news-x" aria-label="Dismiss" onClick={() => setNews((n) => n.filter((x) => x.id !== r.id))}>
            ×
          </button>
        </div>
      ))}
    </div>
  )
}

// ReportsPage (Watch > Reports): the owner's list, open ones first, each
// answered in a press; everyone else sees their own and the answers.
export default function ReportsPage() {
  useDocumentTitle('Reports')
  const [data, setData] = useState<{ reports: WatchReport[]; open: number; owner: boolean } | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    let live = true
    api
      .reports()
      .then((d) => live && setData(d))
      .catch((e) => live && setError(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [])
  const replace = (r: WatchReport) =>
    setData((d) => d && { ...d, reports: d.reports.map((x) => (x.id === r.id ? r : x)), open: d.reports.filter((x) => (x.id === r.id ? r : x).status === 'open').length })
  return (
    <div className="wx-page">
      <h1>Reports</h1>
      <p className="wx-dim">
        {data?.owner ? 'Problems people reported while watching. Answer one and whoever sent it hears back.' : 'Problems you reported, and what the owner said.'}
      </p>
      {error && <p className="wx-error">{error}</p>}
      {data && data.reports.length === 0 && <p className="wx-dim">Nothing reported. Everything’s playing fine.</p>}
      <div className="wx-reports">
        {data?.reports.map((r) => <ReportRow key={r.id} r={r} owner={data.owner} onChange={replace} />)}
      </div>
    </div>
  )
}

function ReportRow({ r, owner, onChange }: { r: WatchReport; owner: boolean; onChange: (r: WatchReport) => void }) {
  const navigate = useNavigate()
  const [reply, setReply] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const answer = async (status: 'fixed' | 'wontfix', wrongVideo = false) => {
    setBusy(true)
    setError('')
    try {
      onChange(await api.resolveReport(r.id, status, reply.trim() || (wrongVideo ? 'That version won’t be used again: the next one plays now.' : ''), wrongVideo))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <article className={`wx-report-row ${r.status}`}>
      <div className="wx-report-head">
        <button className="wx-linkish" onClick={() => navigate(titleHref(r))}>
          {r.title}
          {r.episode ? ` · ${epCode(r.season, r.episode)}` : ''}
        </button>
        <span className={`wx-report-tag ${r.problem}`}>{problemLabel(r.problem)}</span>
        <span className={`wx-report-status ${r.status}`}>{r.status === 'open' ? 'Open' : r.status === 'fixed' ? 'Fixed' : 'Won’t fix'}</span>
      </div>
      <div className="wx-dim">
        {owner ? `${r.who} · ` : ''}
        {shortDate(r.createdAt)}
        {r.release ? ` · ${r.release}` : ''}
      </div>
      {r.note && <p className="wx-report-quote">“{r.note}”</p>}
      {r.reply && <p className="wx-report-reply">{r.reply}</p>}
      {owner && r.status === 'open' && (
        <div className="wx-report-answer">
          <input placeholder="Reply (optional)" value={reply} maxLength={500} onChange={(e) => setReply(e.target.value)} />
          <div className="wx-actions">
            {r.release && (
              <button className="wx-btn small play" disabled={busy} onClick={() => void answer('fixed', true)} title="Never use this version for the title again">
                Wrong video: drop this version
              </button>
            )}
            <button className="wx-btn small" disabled={busy} onClick={() => void answer('fixed')}>
              Mark fixed
            </button>
            <button className="wx-btn small" disabled={busy} onClick={() => void answer('wontfix')}>
              Can’t fix
            </button>
          </div>
          {error && <p className="wx-error">{error}</p>}
        </div>
      )}
    </article>
  )
}
