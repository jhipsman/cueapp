import { useEffect, useState } from 'react'
import Icon from './Icon'

type Theme = 'light' | 'dark'

const STORAGE_KEY = 'cue-theme'

function applyTheme(theme: Theme) {
  if (theme === 'dark') {
    delete document.documentElement.dataset.theme
  } else {
    document.documentElement.dataset.theme = theme
  }
}

function savedTheme(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored === 'light' ? 'light' : 'dark'
  } catch {
    // storage can be blocked (private windows): Cue's own dark
    return 'dark'
  }
}

// Cue is dark, as Watch is; this switches the manager to light and back,
// as a single icon button (sun or moon).
export default function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(savedTheme)

  useEffect(() => {
    applyTheme(theme)
    try {
      localStorage.setItem(STORAGE_KEY, theme)
    } catch {
      // a blocked localStorage just means the choice will not survive a reload
    }
  }, [theme])

  const next: Theme = theme === 'light' ? 'dark' : 'light'
  const label = theme === 'light' ? 'Theme: light' : 'Theme: dark'

  return (
    <button className="icon-btn top-icon" onClick={() => setTheme(next)} title={`${label}. Click to change.`} aria-label={label}>
      <Icon name={theme === 'light' ? 'sun' : 'moon'} size={18} />
    </button>
  )
}
