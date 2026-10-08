import { useCallback, useEffect, useState } from 'react'
import { api, type LiveProgramme, type LiveRecording } from '../api'

// The household's Live TV recordings, kept fresh while a page shows them.
export function useRecordings(enabled = true) {
  const [list, setList] = useState<LiveRecording[]>([])
  const load = useCallback(() => {
    if (!enabled) return
    api
      .liveRecordings()
      .then(setList)
      .catch(() => undefined)
  }, [enabled])
  useEffect(() => {
    load()
    const t = setInterval(load, 30_000)
    return () => clearInterval(t)
  }, [load])

  const find = (channelId: string, start: string) => list.find((r) => r.channelId === channelId && Date.parse(r.start) === Date.parse(start))
  const record = async (channelId: string, show: LiveProgramme) => {
    await api.addLiveRecording({ channelId, start: show.start, stop: show.stop, title: show.title })
    load()
  }
  const remove = async (id: number) => {
    await api.removeLiveRecording(id)
    load()
  }
  return { list, find, record, remove, reload: load }
}

// recordingSize is a recording's size in words.
export function recordingSize(bytes: number): string {
  if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GB`
  return `${Math.max(1, Math.round(bytes / (1 << 20)))} MB`
}
