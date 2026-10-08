import { Navigate, useParams, useSearchParams } from 'react-router-dom'
import { liveHref } from './LivePage'

// The old full-screen channel address (/watch/live/play/:id): Live TV now
// plays channels in its own page, in a window over the guide or full
// screen, so this forwards there, full screen.
export default function LivePlayer() {
  const { id = '' } = useParams()
  const [params] = useSearchParams()
  return <Navigate to={liveHref(id, params.get('cat') ?? 'all')} replace />
}
