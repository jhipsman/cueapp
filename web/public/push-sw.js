// Cue's phone alerts. This worker only shows alerts: it has no fetch
// handler and caches nothing (pages always come from Cue as they are), and
// its scope (/cue-push/) holds no page, so it never controls one.
//
// A push from Cue carries no text. On one, the worker asks Cue what to say
// (POST /api/push/pending, signed in with the browser's own cookie) and
// shows it; pressing an alert opens Cue at that title.

self.addEventListener('push', (event) => {
  event.waitUntil(show())
})

async function show() {
  let alerts = []
  try {
    const sub = await self.registration.pushManager.getSubscription()
    const res = await fetch('/api/push/pending', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: sub ? sub.endpoint : '' }),
    })
    if (res.ok) alerts = await res.json()
  } catch (e) {
    // Shown below as a plain alert.
  }
  if (!Array.isArray(alerts) || alerts.length === 0) {
    // A push must show something (browsers insist).
    return self.registration.showNotification('Cue', { body: 'Something new is on Cue.', icon: '/icon-192.png', data: { url: '/watch' } })
  }
  await Promise.all(
    alerts.map((a) =>
      self.registration.showNotification(a.title, { body: a.body, icon: '/icon-192.png', tag: 'cue-' + a.id, data: { url: a.url || '/watch' } }),
    ),
  )
}

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = new URL((event.notification.data && event.notification.data.url) || '/watch', self.location.origin).href
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((wins) => {
      for (const w of wins) {
        if ('navigate' in w && 'focus' in w) return w.navigate(url).then((c) => (c || w).focus())
      }
      return self.clients.openWindow(url)
    }),
  )
})
