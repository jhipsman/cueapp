import { useEffect } from 'react'

// useDarkPage shows a page dark whatever the theme, while it is open: the
// sign-in and sign-up cards of Cue as a streaming app, to match Watch.
export function useDarkPage(on: boolean) {
  useEffect(() => {
    if (!on) return
    const root = document.documentElement
    const before = root.dataset.theme
    root.dataset.theme = 'dark'
    return () => {
      if (before === undefined) delete root.dataset.theme
      else root.dataset.theme = before
    }
  }, [on])
}
