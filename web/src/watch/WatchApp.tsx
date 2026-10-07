import { useEffect, useState } from 'react'
import { Link, NavLink, Route, Routes, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import BrandMark from '../components/BrandMark'
import Icon from '../components/Icon'
import Home from './Home'
import MyList from './MyList'
import SearchPage from './SearchPage'
import TitlePage from './TitlePage'
import PlayerPage from './PlayerPage'
import './watch.css'

// Watch: the streaming side of Cue. Browse anything TMDB knows, press
// Play, and it streams from Premiumize. It has no Cue sidebar: the
// library manager is one click away ("Manage").
export default function WatchApp() {
  useEffect(() => {
    document.documentElement.classList.add('watch-mode')
    return () => document.documentElement.classList.remove('watch-mode')
  }, [])

  return (
    <div className="wx">
      <Routes>
        <Route path="play/movie/:tmdbId" element={<PlayerPage />} />
        <Route path="play/tv/:tmdbId/:season/:episode" element={<PlayerPage />} />
        <Route
          path="*"
          element={
            <>
              <TopBar />
              <Routes>
                <Route index element={<Home />} />
                <Route path="movies" element={<Home only="movie" />} />
                <Route path="shows" element={<Home only="tv" />} />
                <Route path="list" element={<MyList />} />
                <Route path="search" element={<SearchPage />} />
                <Route path="movie/:tmdbId" element={<TitlePage kind="movie" />} />
                <Route path="tv/:tmdbId" element={<TitlePage kind="tv" />} />
                <Route path="*" element={<Home />} />
              </Routes>
            </>
          }
        />
      </Routes>
    </div>
  )
}

function TopBar() {
  const navigate = useNavigate()
  const location = useLocation()
  const [params] = useSearchParams()
  const [solid, setSolid] = useState(false)
  const [q, setQ] = useState(location.pathname.endsWith('/search') ? (params.get('q') ?? '') : '')

  useEffect(() => {
    const onScroll = () => setSolid(window.scrollY > 40)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  // Search as you type, after a short pause.
  useEffect(() => {
    const term = q.trim()
    const onSearch = location.pathname.endsWith('/search')
    if (!term) {
      if (onSearch) navigate('/watch', { replace: true })
      return
    }
    const t = setTimeout(() => navigate(`/watch/search?q=${encodeURIComponent(term)}`, { replace: onSearch }), 350)
    return () => clearTimeout(t)
    // Only the typed text should start a search.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q])

  return (
    <nav className={`wx-nav${solid ? ' solid' : ''}`}>
      <Link to="/watch" className="wx-brand" aria-label="Cue home">
        <BrandMark className="wx-brand-mark" />
        <span>Cue</span>
      </Link>
      <div className="wx-links">
        <NavLink to="/watch" end>
          Home
        </NavLink>
        <NavLink to="/watch/shows">Shows</NavLink>
        <NavLink to="/watch/movies">Movies</NavLink>
        <NavLink to="/watch/list">My List</NavLink>
      </div>
      <div className="wx-nav-right">
        <label className="wx-search">
          <Icon name="search" size={16} />
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Titles, people, genres" aria-label="Search" />
        </label>
        <Link to="/" className="wx-manage" title="Back to managing your library">
          <Icon name="sliders" size={16} /> <span>Manage</span>
        </Link>
      </div>
    </nav>
  )
}
