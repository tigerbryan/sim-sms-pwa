const RELEASE = '__PWA_RELEASE__';
const CACHE = `sms-shell-${RELEASE}`;
const SHELL = ['/offline.html', '/icons/messages-192.png', '/icons/messages-512.png', '/icons/apple-touch-icon.png'];
self.addEventListener('install', event => {
  event.waitUntil(caches.open(CACHE).then(cache => cache.addAll(SHELL.map(url => new Request(url, {cache:'reload'})))));
});
// An installed update waits for the user's explicit update action. Push subscriptions stay intact.
self.addEventListener('message', event => {
  if (event.data?.type === 'SKIP_WAITING') event.waitUntil(self.skipWaiting());
});
self.addEventListener('activate', event => {
  event.waitUntil(caches.keys().then(keys => Promise.all(keys.filter(key => key.startsWith('sms-shell-') && key !== CACHE).map(key => caches.delete(key)))).then(() => self.clients.claim()));
});
self.addEventListener('fetch', event => {
  const request = event.request;
  const url = new URL(request.url);
  if (request.method !== 'GET' || url.origin !== self.location.origin) return;
  if (SHELL.includes(url.pathname)) {
    event.respondWith(caches.open(CACHE).then(cache => cache.match(url.pathname)).then(cached => cached || fetch(request)));
    return;
  }
  // Never cache SMS responses, credentials, login pages, or API data.
  if (request.mode === 'navigate') {
    event.respondWith(fetch(request).catch(() => caches.match('/offline.html')));
  }
});

self.addEventListener('push', event => {
  let data = {};
  try {
    const value = event.data?.json();
    if (value && typeof value === 'object' && !Array.isArray(value)) data = value;
  } catch {}
  // Always display a visible notification, including when the page is closed or unauthenticated.
  event.waitUntil((async () => {
    await self.registration.showNotification(typeof data.title === 'string' && data.title ? data.title : '信息', {
      body: typeof data.body === 'string' && data.body ? data.body : '收到一条新短信，点按查看。',
      icon: `/icons/messages-192.png?v=${RELEASE}`, badge: `/icons/messages-192.png?v=${RELEASE}`,
      tag: typeof data.tag === 'string' && data.tag ? data.tag : 'sms', data: {url: '/sms'},
    });
    // Missing or malformed counts must not erase an existing unread badge.
    if (Number.isSafeInteger(data.unread) && data.unread >= 0) {
      try {
        if (data.unread > 0 && 'setAppBadge' in navigator) await navigator.setAppBadge(data.unread);
        else if (data.unread === 0 && 'clearAppBadge' in navigator) await navigator.clearAppBadge();
      } catch {}
    }
    const pages = await self.clients.matchAll({type: 'window', includeUncontrolled: true});
    for (const page of pages) page.postMessage({type: 'sms-push'});
  })());
});
self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil((async () => {
    const url = new URL('/sms', self.location.origin).href;
    const pages = await self.clients.matchAll({type: 'window', includeUncontrolled: true});
    for (const page of pages) {
      if (page.url.startsWith(self.location.origin + '/')) {
        if ('navigate' in page) await page.navigate(url);
        return page.focus();
      }
    }
    return self.clients.openWindow(url);
  })());
});
