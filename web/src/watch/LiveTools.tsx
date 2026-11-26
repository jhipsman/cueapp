import { useEffect, useState } from 'react'
import { api, type LiveChannel, type LiveHit, type LiveList, type LiveProgramme } from '../api'
import Icon from '../components/Icon'
import ChannelLogo from './ChannelLogo'

// Live TV's own-yours parts: For you and Tonight at the top of All
// channels, a channel's tools (rename it, put it in a list, match its
// guide), and editing a list (its order, what's in it, its name).

const hhmm = (d: Date | string) => new Date(d).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

// For you (what's on the channels you watch most) and Tonight (the big
// shows this evening), as two rows of cards.
export function LivePicks({ onWatch, onShow }: { onWatch: (id: string) => void; onShow: (c: LiveChannel, show: LiveProgramme) => void }) {
  const [picks, setPicks] = useState<{ tonight: LiveHit[]; forYou: LiveHit[] } | null>(null)
  useEffect(() => {
    let live = true
    const load = () => api.livePicks().then((p) => live && setPicks(p)).catch(() => undefined)
    load()
    const t = window.setInterval(load, 5 * 60_000)
    return () => {
      live = false
      window.clearInterval(t)
    }
  }, [])
  if (!picks || (picks.forYou.length === 0 && picks.tonight.length === 0)) return null
  return (
    <div className="wx-picks">
      {picks.forYou.length > 0 && (
        <section aria-label="For you">
          <h3>For you</h3>
          <div className="wx-picks-row">
            {picks.forYou.map((h) => (
              <button key={h.channel.id} className="wx-pick" onClick={() => onWatch(h.channel.id)} title={`Watch ${h.channel.name}`}>
                <span className="wx-pick-logo">
                  <ChannelLogo name={h.channel.name} src={h.channel.logo} />
                </span>
                <span className="wx-pick-text">
                  <small>
                    <i className="wx-chan-live" aria-hidden="true" /> {h.channel.name}
                  </small>
                  <b>{h.show?.title ?? 'On now'}</b>
                </span>
              </button>
            ))}
          </div>
        </section>
      )}
      {picks.tonight.length > 0 && (
        <section aria-label="Tonight">
          <h3>Tonight</h3>
          <div className="wx-picks-row">
            {picks.tonight.map((h) => (
              <button
                key={h.channel.id + (h.show?.start ?? '')}
                className="wx-pick"
                onClick={() => h.show && onShow(h.channel, h.show)}
                title={h.show ? `${h.show.title}, ${hhmm(h.show.start)} on ${h.channel.name}` : h.channel.name}
              >
                <span className="wx-pick-logo">
                  <ChannelLogo name={h.channel.name} src={h.channel.logo} />
                </span>
                <span className="wx-pick-text">
                  <small>
                    {h.show ? hhmm(h.show.start) : ''} · {h.channel.name}
                  </small>
                  <b>{h.show?.title}</b>
                </span>
              </button>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

// A channel's tools: rename it (for this profile), put it in one of your
// lists (or a new one), and for the owner, match it to a guide channel.
export function ChannelTools({ channel, lists, owner, onChanged }: { channel: LiveChannel; lists: LiveList[]; owner: boolean; onChanged: () => void }) {
  const [mode, setMode] = useState<'' | 'rename' | 'list' | 'guide'>('')
  const [name, setName] = useState('')
  const [newList, setNewList] = useState('')
  const [q, setQ] = useState('')
  const [options, setOptions] = useState<{ key: string; name: string; source: string }[]>([])
  const [problem, setProblem] = useState('')
  useEffect(() => {
    setMode('')
    setProblem('')
  }, [channel.id])
  useEffect(() => {
    if (mode !== 'guide') return
    const t = window.setTimeout(() => api.guideChannels(q).then(setOptions).catch(() => setOptions([])), 250)
    return () => window.clearTimeout(t)
  }, [mode, q])

  async function run(f: () => Promise<unknown>) {
    setProblem('')
    try {
      await f()
      setMode('')
      onChanged()
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e))
    }
  }
  const inList = (l: LiveList) => l.items.includes(channel.id)

  return (
    <div className="wx-chan-tools">
      <div className="wx-chan-tools-row">
        <button className="wx-btn small" aria-pressed={mode === 'rename'} onClick={() => (setName(channel.renamed ? channel.name : ''), setMode(mode === 'rename' ? '' : 'rename'))}>
          <Icon name="sliders" size={14} /> Rename
        </button>
        <button className="wx-btn small" aria-pressed={mode === 'list'} onClick={() => setMode(mode === 'list' ? '' : 'list')}>
          <Icon name="list" size={14} /> Lists
        </button>
        {owner && (
          <button className="wx-btn small" aria-pressed={mode === 'guide'} onClick={() => (setQ(channel.name), setMode(mode === 'guide' ? '' : 'guide'))}>
            <Icon name="calendar" size={14} /> Guide
          </button>
        )}
      </div>
      {mode === 'rename' && (
        <form className="wx-chan-tools-panel" onSubmit={(e) => (e.preventDefault(), void run(() => api.renameChannel(channel.id, name.trim())))}>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder={channel.raw ?? channel.name} maxLength={60} autoFocus />
          <button className="wx-btn small play" type="submit">
            Save
          </button>
          {channel.renamed && (
            <button className="wx-btn small" type="button" onClick={() => void run(() => api.renameChannel(channel.id, ''))}>
              Its own name
            </button>
          )}
        </form>
      )}
      {mode === 'list' && (
        <div className="wx-chan-tools-panel">
          {lists.map((l) => (
            <button
              key={l.id}
              className="wx-btn small"
              aria-pressed={inList(l)}
              onClick={() => void run(() => api.updateLiveList(l.id, { items: inList(l) ? l.items.filter((x) => x !== channel.id) : [...l.items, channel.id] }))}
            >
              <Icon name={inList(l) ? 'check' : 'plus'} size={14} /> {l.name}
            </button>
          ))}
          <form onSubmit={(e) => (e.preventDefault(), newList.trim() && void run(() => api.createLiveList(newList.trim(), [channel.id])))}>
            <input value={newList} onChange={(e) => setNewList(e.target.value)} placeholder="A new list…" maxLength={60} />
            <button className="wx-btn small" type="submit" disabled={!newList.trim()}>
              Make it
            </button>
          </form>
        </div>
      )}
      {mode === 'guide' && (
        <div className="wx-chan-tools-panel column">
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Find the channel in the guides" autoFocus />
          <div className="wx-guide-options">
            <button className="wx-guide-option" onClick={() => void run(() => api.matchGuide(channel.id, ''))}>
              <b>Automatic</b>
              <small>The provider&apos;s guide, or an extra guide&apos;s channel with the same name</small>
            </button>
            {options.map((o) => (
              <button key={o.key} className="wx-guide-option" onClick={() => void run(() => api.matchGuide(channel.id, o.key))}>
                <b>{o.name || o.key}</b>
                <small>{o.source}</small>
              </button>
            ))}
            {options.length === 0 && <small className="wx-dim">No guide channels named like that.</small>}
          </div>
        </div>
      )}
      {problem && <p className="wx-error">{problem}</p>}
    </div>
  )
}

// Editing a list: its channels in order (up, down, out), its name, or
// removing it.
export function ListEditor({ list, channels, onChanged, onRemoved }: { list: LiveList; channels: LiveChannel[]; onChanged: () => void; onRemoved: () => void }) {
  const [name, setName] = useState(list.name)
  const [items, setItems] = useState(list.items)
  useEffect(() => {
    setName(list.name)
    setItems(list.items)
  }, [list])
  const byId = new Map(channels.map((c) => [c.id, c]))
  const save = (next: string[]) => {
    setItems(next)
    void api.updateLiveList(list.id, { items: next }).then(onChanged)
  }
  const move = (i: number, d: number) => {
    const j = i + d
    if (j < 0 || j >= items.length) return
    const next = [...items]
    ;[next[i], next[j]] = [next[j], next[i]]
    save(next)
  }
  return (
    <div className="wx-list-editor">
      <form
        className="wx-list-editor-head"
        onSubmit={(e) => {
          e.preventDefault()
          if (name.trim() && name.trim() !== list.name) void api.updateLiveList(list.id, { name: name.trim() }).then(onChanged)
        }}
      >
        <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} aria-label="The list's name" />
        <button className="wx-btn small" type="submit" disabled={!name.trim() || name.trim() === list.name}>
          Rename
        </button>
        <button
          className="wx-btn small"
          type="button"
          onClick={() => {
            if (window.confirm(`Remove the list “${list.name}”? The channels stay.`)) void api.deleteLiveList(list.id).then(onRemoved)
          }}
        >
          <Icon name="trash" size={14} /> Remove the list
        </button>
      </form>
      {items.length === 0 && <p className="wx-dim">No channels yet: a channel&apos;s Lists button puts it here.</p>}
      <ol>
        {items.map((id, i) => {
          const c = byId.get(id)
          return (
            <li key={id}>
              <span className="wx-list-editor-logo">{c && <ChannelLogo name={c.name} src={c.logo} />}</span>
              <span className="wx-list-editor-name">{c?.name ?? 'Not on your provider any more'}</span>
              <button className="wx-btn small" onClick={() => move(i, -1)} disabled={i === 0} aria-label="Up">
                <Icon name="chevron-up" size={14} />
              </button>
              <button className="wx-btn small" onClick={() => move(i, 1)} disabled={i === items.length - 1} aria-label="Down">
                <Icon name="chevron-down" size={14} />
              </button>
              <button className="wx-btn small" onClick={() => save(items.filter((x) => x !== id))} aria-label="Take it out">
                <Icon name="x" size={14} />
              </button>
            </li>
          )
        })}
      </ol>
    </div>
  )
}
