import { Outlet, useLocation } from 'react-router-dom'
import { isAdmin } from '../../api'
import { useAuth } from '../../AuthContext'
import { useModules } from '../../ModulesContext'
import { pageTitle } from '../../settingsNav'

// Holds the open settings page. Pages that share a sidebar entry are listed
// under it in the sidebar, and the top bar names the group and the page. On
// phones the top bar has no room for the name, so it is repeated here.
export default function SettingsLayout() {
  const { pathname } = useLocation()
  const { user } = useAuth()
  const { streamingOnly } = useModules()
  return (
    <div className="settings-content">
      <h2 className="settings-phone-title">{pageTitle(pathname, isAdmin(user), streamingOnly)}</h2>
      <Outlet />
    </div>
  )
}
