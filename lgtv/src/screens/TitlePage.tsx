import { useEffect, useState } from 'react'
import { api, backdrop, type Card, type Episode, type Title } from '../api'
import { Btn, Icon, PosterRow, Spinner, useFocusOnShow } from '../ui'

export interface PlayTarget {
  kind: Card['kind']
  tmdbId: number
  season: number
  episode: number
  title: string
  label?: string // "S2:E5 · The One Where…"
}

// A title's page: its picture behind everything, its name, facts and story,
// Play (or Resume) and My List, a show's seasons and episodes, and More like
// this. Down from the buttons moves through them; the page follows.
export function TitleScreen({ card, onPlay, onPick }: { card: Card; onPlay: (t: PlayTarget) => void; onPick: (c: Card) => void }) {
  const [t, setT] = useState<Title | null>(null)
  const [problem, setProblem] = useState('')
  const [season, setSeason] = useState(0)
  const [episodes, setEpisodes] = useState<Episode[] | null>(null)
  const [inList, setInList] = useState(false)
  const ref = useFocusOnShow(t ? 1 : 0)

  useEffect(() => {
    let live = true
    api
      .title(card.kind, card.tmdbId)
      .then((a) => {
        if (!live) return
        setT(a)
        setInList(a.inList)
        const seasons = (a.seasons ?? []).filter((s) => s.number > 0)
        setSeason(a.resumeSeason || seasons[0]?.number || a.seasons?.[0]?.number || 0)
      })
      .catch((e) => live && setProblem(e instanceof Error ? e.message : String(e)))
    return () => {
      live = false
    }
  }, [card.kind, card.tmdbId])

  useEffect(() => {
    if (card.kind !== 'tv' || !season) return
    let live = true
    setEpisodes(null)
    api
      .season(card.tmdbId, season)
      .then((a) => live && setEpisodes(a ?? []))
      .catch(() => live && setEpisodes([]))
    return () => {
      live = false
    }
  }, [card.kind, card.tmdbId, season])

  const shown = t ?? card
  const art = backdrop(shown.backdropUrl)
  const facts = t
    ? [t.year, t.runtime ? `${Math.floor(t.runtime / 60) ? `${Math.floor(t.runtime / 60)}h ` : ''}${t.runtime % 60}m` : '', t.certification, t.genres.slice(0, 3).join(', '), t.rating ? `★ ${t.rating.toFixed(1)}` : '']
    : []
  const isShow = card.kind === 'tv'
  const rs = t?.resumeSeason ?? 0
  const re = t?.resumeEpisode ?? 0
  const resuming = isShow ? rs > 0 && re > 0 : (t?.resumePosition ?? 0) > 30
  const firstSeason = (t?.seasons ?? []).find((s) => s.number > 0)?.number ?? 1
  const playLabel = isShow ? `${resuming ? 'Resume' : 'Play'} S${resuming ? rs : firstSeason}:E${resuming ? re : 1}` : resuming ? 'Resume' : 'Play'

  function play(s = 0, e = 0, label = '') {
    if (!t) return
    if (isShow && !s) {
      s = resuming ? rs : firstSeason
      e = resuming ? re : 1
    }
    onPlay({ kind: card.kind, tmdbId: card.tmdbId, season: s, episode: e, title: t.title, label: isShow ? label || `S${s}:E${e}` : '' })
  }
  async function toggleList() {
    const on = !inList
    setInList(on)
    try {
      await api.setInList(card.kind, card.tmdbId, on)
    } catch {
      setInList(!on)
    }
  }

  return (
    <div className="screen title-page">
      <div className="ambient strong">{art && <img src={art} alt="" />}</div>
      <div className="title-scroll" data-vs="" ref={ref}>
        <div className="title-top" data-row="">
          {shown.logoUrl ? <img className="title-logo" src={shown.logoUrl} alt={shown.title} /> : <h1>{shown.title}</h1>}
          <p className="facts">{facts.filter(Boolean).join('  ·  ')}</p>
          {t?.tagline && <p className="tagline">{t.tagline}</p>}
          {shown.overview && <p className="story long">{shown.overview}</p>}
          {problem && <p className="problem">{problem}</p>}
          {!t && !problem && <Spinner />}
          {t && (
            <div className="btns" data-g="title-buttons">
              {t.notOut ? (
                <p className="note">{t.notOut}</p>
              ) : (
                <Btn primary first k="play" icon={<Icon name="play" />} onClick={() => play()}>
                  {playLabel}
                </Btn>
              )}
              <Btn round k="list" label={inList ? 'In My List' : 'My List'} icon={<Icon name={inList ? 'check' : 'plus'} />} onClick={() => void toggleList()} />
            </div>
          )}
          {t?.theaterNote && <p className="note">{t.theaterNote}</p>}
        </div>

        {isShow && t && (t.seasons ?? []).length > 0 && (
          <section className="row seasons" data-row="" data-g="seasons">
            <div className="row-track" data-hs="0">
              <div className="row-items">
                {(t.seasons ?? [])
                  .filter((s) => s.number > 0 || (t.seasons ?? []).length === 1)
                  .map((s) => (
                    <button key={s.number} className={`chip${s.number === season ? ' on' : ''}`} data-f="" data-k={String(s.number)} data-first={s.number === season ? '' : undefined} onClick={() => setSeason(s.number)} onFocus={() => setSeason(s.number)}>
                      {s.name || `Season ${s.number}`}
                    </button>
                  ))}
              </div>
            </div>
          </section>
        )}
        {isShow && t && (
          <section className="row episodes" data-row="" data-g={`episodes-${season}`}>
            <div className="row-track" data-hs="0">
              <div className="row-items">
                {episodes === null && <Spinner />}
                {episodes?.map((ep) => (
                  <button
                    key={ep.episode}
                    className={`episode${ep.aired ? '' : ' later'}`}
                    data-f=""
                    data-k={String(ep.episode)}
                    data-first={ep.season === rs && ep.episode === re ? '' : undefined}
                    onClick={() => ep.aired && play(ep.season, ep.episode, `S${ep.season}:E${ep.episode} · ${ep.title}`)}
                  >
                    <span className="episode-art">
                      {ep.stillUrl ? <img src={ep.stillUrl} alt="" loading="lazy" /> : null}
                      {ep.progress ? (
                        <i className="poster-bar">
                          <i style={{ width: `${Math.round((ep.finished ? 1 : ep.progress) * 100)}%` }} />
                        </i>
                      ) : null}
                    </span>
                    <b>
                      {ep.episode}. {ep.title}
                    </b>
                    <small>{ep.aired ? (ep.runtime ? `${ep.runtime}m` : '') : ep.airDate ? `Out ${ep.airDate}` : 'Coming'}</small>
                    {ep.overview && <span className="episode-story">{ep.overview}</span>}
                  </button>
                ))}
              </div>
            </div>
          </section>
        )}
        {t && t.more.length > 0 && <PosterRow id={`more-${card.kind}-${card.tmdbId}`} title="More like this" items={t.more.slice(0, 18)} onPick={onPick} />}
        <div className="rows-end" />
      </div>
    </div>
  )
}
