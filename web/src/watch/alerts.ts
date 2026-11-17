import { api } from '../api'

// Phone alerts on this device: the browser's push subscription (with Cue's
// key), kept by Cue for the profile. The worker is push-sw.js, under a scope
// with no pages (it only shows alerts).

const SCOPE = '/cue-push/'

export type AlertsState = 'unsupported' | 'needs-home-screen' | 'blocked' | 'off' | 'on'

function iosBrowserTab(): boolean {
  const ios = /iPhone|iPad|iPod/.test(navigator.userAgent) || (/Macintosh/.test(navigator.userAgent) && navigator.maxTouchPoints > 1)
  const standalone = window.matchMedia('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone === true
  return ios && !standalone
}

async function registration(): Promise<ServiceWorkerRegistration | undefined> {
  return navigator.serviceWorker.getRegistration(SCOPE)
}

export async function alertsState(): Promise<AlertsState> {
  if (!('serviceWorker' in navigator) || !('PushManager' in window) || !('Notification' in window)) {
    return iosBrowserTab() ? 'needs-home-screen' : 'unsupported'
  }
  if (Notification.permission === 'denied') return 'blocked'
  const reg = await registration()
  const sub = await reg?.pushManager.getSubscription()
  return sub ? 'on' : 'off'
}

function keyBytes(b64: string): Uint8Array {
  const pad = '='.repeat((4 - (b64.length % 4)) % 4)
  const raw = atob((b64 + pad).replace(/-/g, '+').replace(/_/g, '/'))
  return Uint8Array.from(raw, (c) => c.charCodeAt(0))
}

// turnAlertsOn asks for permission, subscribes and tells Cue.
export async function turnAlertsOn(): Promise<AlertsState> {
  const perm = await Notification.requestPermission()
  if (perm !== 'granted') return perm === 'denied' ? 'blocked' : 'off'
  const reg = (await registration()) ?? (await navigator.serviceWorker.register('/push-sw.js', { scope: SCOPE }))
  await navigator.serviceWorker.ready.catch(() => undefined)
  const { key } = await api.pushKey()
  const sub =
    (await reg.pushManager.getSubscription()) ??
    (await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(key) as BufferSource }))
  await api.pushSubscribe(sub.endpoint)
  return 'on'
}

export async function turnAlertsOff(): Promise<AlertsState> {
  const reg = await registration()
  const sub = await reg?.pushManager.getSubscription()
  if (sub) {
    await api.pushUnsubscribe(sub.endpoint).catch(() => undefined)
    await sub.unsubscribe()
  }
  return 'off'
}
