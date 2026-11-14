import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, type ListenAlbum, type ListenArtist, type ListenArtistPage, type ListenHome, type ListenResults, type ListenTrack } from '../../api'
import Icon from '../../components/Icon'
import { clock, sized, useMusic } from './player'
import './music.css'

// Watch > Music, laid out as the music apps are: a search box, new releases,
// the top songs and albums, popular artists; an album with its tracklist;
// an artist with their popular songs and albums.

function useLoad<T>(load: () => Promise<T>, deps: unknown[]): { data: T | null; error: string } {
  const [state, setState] = useState<{ data: T | null; error: string }>({ data: null, error: '' })
  useEffect(() => {
    let live = true
    setState({ data: null, error: '' })
    load()
      .then((d) => live && setState({ data: d, error: '' }))
      .catch((e) => live && setState({ data: null, error: e instanceof Error ? e.message : String(e) }))
    return () => {
      live = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
  return state
}

function Loading() {
  return (
    <div className="mu-loading" aria-label="Loading">
      <span />
      <span />
      <span />
    </div>
  )
}

// ---- Pieces ----

function AlbumCard({ a, sub }: { a: ListenAlbum; sub?: string }) {
  return (
    <Link to={`/watch/music/album/${a.id}`} className="mu-card">
      <span className="mu-cover">
        <img src={sized(a.cover || a.coverSmall, 500)} alt="" loading="lazy" />
        <PlayAlbumButton id={a.id} />
      </span>
      <strong>{a.title}</strong>
      <small>{sub ?? a.artist.name}</small>
    </Link>
  )
}

// The round green button on a cover, as the music apps have: plays the
// album without opening it.
function PlayAlbumButton({ id }: { id: number }) {
  const m = useMusic()
  const playingThis = m.current?.albumId === id && m.playing
  return (
    <button
      className="mu-cover-play"
      aria-label={playingThis ? 'Pause' : 'Play'}
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
        if (m.current?.albumId === id) return m.toggle()
        void api.listenAlbum(id).then((a) => m.playTracks(a.tracks ?? []))
      }}
    >
      <Icon name={playingThis ? 'pause' : 'play'} size={20} fill="currentColor" />
    </button>
  )
}

function ArtistCircle({ a }: { a: ListenArtist }) {
  return (
    <Link to={`/watch/music/artist/${a.id}`} className="mu-card mu-artist">
      <span className="mu-cover round">
        <img src={sized(a.picture, 500)} alt="" loading="lazy" />
      </span>
      <strong>{a.name}</strong>
      <small>Artist</small>
    </Link>
  )
}

function Shelf({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mu-shelf">
      <h2>{title}</h2>
      <div className="mu-row">{children}</div>
    </section>
  )
}

// A song in a list: number (or the playing bars), cover, title and artist,
// length. Clicking plays the list from here.
function SongRow({ t, n, list, showCover = true, showArtist = true }: { t: ListenTrack; n: number; list: ListenTrack[]; showCover?: boolean; showArtist?: boolean }) {
  const m = useMusic()
  const isCurrent = m.current?.id === t.id
  return (
    <div className={`mu-song${isCurrent ? ' current' : ''}`}>
      <button className="mu-song-main" onClick={() => (isCurrent ? m.toggle() : m.playTracks(list, n))}>
        <span className="mu-song-n">{isCurrent && m.playing ? <Bars /> : n + 1}</span>
        {showCover && <img src={sized(t.cover, 120)} alt="" loading="lazy" />}
        <span className="mu-song-words">
          <strong>{t.title}</strong>
          <small>
            {t.explicit && <i className="mu-e">E</i>}
            {showArtist ? t.artist.name : ''}
          </small>
        </span>
        <span className="mu-song-len">{clock(t.length)}</span>
      </button>
      <button className="mu-song-more" aria-label={`Play ${t.title} next`} title="Play next" onClick={() => m.addNext(t)}>
        <Icon name="plus" size={16} />
      </button>
    </div>
  )
}

export function Bars() {
  return (
    <span className="mu-bars" aria-label="Playing">
      <i />
      <i />
      <i />
    </span>
  )
}

// ---- Home and search ----

export function MusicHome() {
  const [params, setParams] = useSearchParams()
  const [q, setQ] = useState(params.get('q') ?? '')
  const term = q.trim()
  useEffect(() => {
    const t = window.setTimeout(() => setParams(term ? { q: term } : {}, { replace: true }), 300)
    return () => window.clearTimeout(t)
  }, [term, setParams])
  const asked = params.get('q') ?? ''
  return (
    <div className="mu-page">
      <label className="mu-search">
        <Icon name="search" size={18} />
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Songs, albums or artists" aria-label="Search music" enterKeyHint="search" />
        {q && (
          <button onClick={() => setQ('')} aria-label="Clear">
            <Icon name="x" size={16} />
          </button>
        )}
      </label>
      {asked ? <SearchResults q={asked} /> : <Browse />}
    </div>
  )
}

function Browse() {
  const { data, error } = useLoad<ListenHome>(() => api.listenHome(), [])
  if (error) return <p className="mu-note">{error}</p>
  if (!data) return <Loading />
  return (
    <>
      {data.newReleases.length > 0 && (
        <Shelf title="New releases">
          {data.newReleases.map((a) => (
            <AlbumCard key={a.id} a={a} />
          ))}
        </Shelf>
      )}
      <section className="mu-shelf">
        <h2>Top songs</h2>
        <TopSongs list={data.topTracks} />
      </section>
      <Shelf title="Top albums">
        {data.topAlbums.map((a) => (
          <AlbumCard key={a.id} a={a} />
        ))}
      </Shelf>
      <Shelf title="Popular artists">
        {data.topArtists.map((a) => (
          <ArtistCircle key={a.id} a={a} />
        ))}
      </Shelf>
    </>
  )
}

function TopSongs({ list }: { list: ListenTrack[] }) {
  const [all, setAll] = useState(false)
  const shown = all ? list : list.slice(0, 10)
  return (
    <div className="mu-songs">
      {shown.map((t, i) => (
        <SongRow key={t.id} t={t} n={i} list={list} />
      ))}
      {list.length > 10 && (
        <button className="mu-more" onClick={() => setAll((a) => !a)}>
          {all ? 'Show less' : 'Show all'}
        </button>
      )}
    </div>
  )
}

function SearchResults({ q }: { q: string }) {
  const { data, error } = useLoad<ListenResults>(() => api.listenSearch(q), [q])
  if (error) return <p className="mu-note">{error}</p>
  if (!data) return <Loading />
  if (!data.tracks.length && !data.albums.length && !data.artists.length) return <p className="mu-note">Nothing found for “{q}”.</p>
  const top = data.artists[0]
  return (
    <>
      <div className="mu-top-result">
        {top && (
          <section className="mu-shelf">
            <h2>Top result</h2>
            <Link to={`/watch/music/artist/${top.id}`} className="mu-best">
              <img src={sized(top.picture, 250)} alt="" />
              <strong>{top.name}</strong>
              <small>Artist</small>
            </Link>
          </section>
        )}
        {data.tracks.length > 0 && (
          <section className="mu-shelf">
            <h2>Songs</h2>
            <div className="mu-songs">
              {data.tracks.slice(0, 5).map((t, i) => (
                <SongRow key={t.id} t={t} n={i} list={data.tracks} />
              ))}
            </div>
          </section>
        )}
      </div>
      {data.albums.length > 0 && (
        <Shelf title="Albums">
          {data.albums.map((a) => (
            <AlbumCard key={a.id} a={a} sub={`${a.artist.name}`} />
          ))}
        </Shelf>
      )}
      {data.artists.length > 0 && (
        <Shelf title="Artists">
          {data.artists.map((a) => (
            <ArtistCircle key={a.id} a={a} />
          ))}
        </Shelf>
      )}
    </>
  )
}

// ---- An album ----

const TYPE: Record<string, string> = { album: 'Album', ep: 'EP', single: 'Single', compile: 'Compilation' }

function totalTime(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.round((sec % 3600) / 60)
  return h ? `${h} hr ${m} min` : `${m} min`
}

export function MusicAlbumPage() {
  const id = Number(useParams().id)
  const { data: a, error } = useLoad<ListenAlbum>(() => api.listenAlbum(id), [id])
  const m = useMusic()
  const tint = useCoverTint(a?.coverSmall)
  if (error) return <p className="mu-note mu-page">{error}</p>
  if (!a) return <Loading />
  const tracks = a.tracks ?? []
  const playingHere = m.current?.albumId === a.id
  const sound = m.soundOf(a.id)
  const discs = new Set(tracks.map((t) => t.disc ?? 1)).size
  return (
    <div className="mu-page mu-detail" style={tint ? { ['--mu-tint' as string]: tint } : undefined}>
      <header className="mu-head">
        <img className="mu-head-cover" src={sized(a.cover, 500)} alt="" />
        <div className="mu-head-words">
          <small>{TYPE[a.type ?? ''] ?? 'Album'}</small>
          <h1>{a.title}</h1>
          <p>
            <Link to={`/watch/music/artist/${a.artist.id}`}>{a.artist.name}</Link>
            {a.released && <> · {a.released.slice(0, 4)}</>}
            {tracks.length > 0 && (
              <>
                {' '}
                · {tracks.length} {tracks.length === 1 ? 'song' : 'songs'}, {totalTime(a.length || tracks.reduce((s, t) => s + t.length, 0))}
              </>
            )}
          </p>
        </div>
      </header>
      <div className="mu-actions">
        <button className="mu-big-play" aria-label={playingHere && m.playing ? 'Pause' : 'Play'} onClick={() => (playingHere ? m.toggle() : m.playTracks(tracks))}>
          <Icon name={playingHere && m.playing ? 'pause' : 'play'} size={26} fill="currentColor" />
        </button>
        <button className={`mu-icon-btn${m.shuffle && playingHere ? ' on' : ''}`} aria-label="Shuffle" onClick={() => m.playTracks(tracks, 0, { shuffle: true })}>
          <Icon name="shuffle" size={22} />
        </button>
        <SoundState s={sound} />
      </div>
      <div className="mu-songs">
        {tracks.map((t, i) => (
          <div key={t.id}>
            {discs > 1 && (i === 0 || tracks[i - 1].disc !== t.disc) && <h3 className="mu-disc">Disc {t.disc}</h3>}
            <SongRow t={t} n={i} list={tracks} showCover={false} showArtist={t.artist.id !== a.artist.id || !!t.explicit} />
          </div>
        ))}
      </div>
      {a.released && (
        <p className="mu-fine">
          {new Date(a.released).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })}
          {a.genre && <> · {a.genre}</>}
        </p>
      )}
    </div>
  )
}

// Where this album's sound stands, once it's been asked for.
function SoundState({ s }: { s?: { status: string; message?: string; progress?: number } }) {
  if (!s) return null
  if (s.status === 'looking') return <span className="mu-state">Finding it…</span>
  if (s.status === 'fetching') return <span className="mu-state">Premiumize is getting it{s.progress ? ` · ${Math.round(s.progress * 100)}%` : '…'}</span>
  if (s.status === 'none') return <span className="mu-state bad">{s.message}</span>
  return null
}

// ---- An artist ----

export function MusicArtistPage() {
  const id = Number(useParams().id)
  const navigate = useNavigate()
  const { data: p, error } = useLoad<ListenArtistPage>(() => api.listenArtist(id), [id])
  const m = useMusic()
  if (error) return <p className="mu-note mu-page">{error}</p>
  if (!p) return <Loading />
  const albums = p.albums.filter((a) => a.type !== 'single' && a.type !== 'ep')
  const singles = p.albums.filter((a) => a.type === 'single' || a.type === 'ep')
  const playingHere = !!m.current && p.top.some((t) => t.id === m.current?.id)
  return (
    <div className="mu-page mu-detail">
      <header className="mu-artist-head" style={{ backgroundImage: `url(${sized(p.artist.picture, 1000)})` }}>
        <div>
          <h1>{p.artist.name}</h1>
          {p.artist.fans ? <small>{p.artist.fans.toLocaleString()} fans</small> : null}
        </div>
      </header>
      <div className="mu-actions">
        <button className="mu-big-play" aria-label={playingHere && m.playing ? 'Pause' : 'Play'} onClick={() => (playingHere ? m.toggle() : m.playTracks(p.top))}>
          <Icon name={playingHere && m.playing ? 'pause' : 'play'} size={26} fill="currentColor" />
        </button>
        <button className="mu-icon-btn" aria-label="Shuffle" onClick={() => m.playTracks(p.top, 0, { shuffle: true })}>
          <Icon name="shuffle" size={22} />
        </button>
      </div>
      {p.top.length > 0 && (
        <section className="mu-shelf">
          <h2>Popular</h2>
          <TopSongs list={p.top} />
        </section>
      )}
      {albums.length > 0 && (
        <Shelf title="Albums">
          {albums.map((a) => (
            <AlbumCard key={a.id} a={a} sub={a.released?.slice(0, 4) ?? ''} />
          ))}
        </Shelf>
      )}
      {singles.length > 0 && (
        <Shelf title="Singles and EPs">
          {singles.map((a) => (
            <AlbumCard key={a.id} a={a} sub={`${a.released?.slice(0, 4) ?? ''} · ${TYPE[a.type ?? ''] ?? ''}`} />
          ))}
        </Shelf>
      )}
      {p.related.length > 0 && (
        <Shelf title="Fans also like">
          {p.related.map((a) => (
            <ArtistCircle key={a.id} a={a} />
          ))}
        </Shelf>
      )}
      <button className="mu-back" onClick={() => navigate(-1)} aria-label="Back">
        <Icon name="chevron-left" size={22} />
      </button>
    </div>
  )
}

// useCoverTint: the album's main colour, for the page's glow (read from a
// small copy of the cover; Deezer's images allow it).
export function useCoverTint(url?: string): string {
  const [tint, setTint] = useState('')
  const asked = useRef('')
  useEffect(() => {
    if (!url || asked.current === url) return
    asked.current = url
    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      try {
        const c = document.createElement('canvas')
        c.width = c.height = 12
        const g = c.getContext('2d')
        if (!g) return
        g.drawImage(img, 0, 0, 12, 12)
        const d = g.getImageData(0, 0, 12, 12).data
        let r = 0,
          gr = 0,
          b = 0,
          n = 0
        for (let i = 0; i < d.length; i += 4) {
          const lum = d[i] + d[i + 1] + d[i + 2]
          if (lum < 60 || lum > 700) continue // skip near-black and near-white
          r += d[i]
          gr += d[i + 1]
          b += d[i + 2]
          n++
        }
        if (n) setTint(`rgb(${Math.round(r / n)}, ${Math.round(gr / n)}, ${Math.round(b / n)})`)
      } catch {
        // A cover the browser won't let us read: no tint.
      }
    }
    img.src = sized(url, 60)
  }, [url])
  return tint
}
