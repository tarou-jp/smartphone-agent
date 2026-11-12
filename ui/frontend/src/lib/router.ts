// A hash router: the window's address is the loopback server's, so pages
// live after the #. Routes:
//
//	#/phones            all phones
//	#/phones/<id>       one phone
//	#/activity[/<task>] Activity, with a task open
//	#/flows[/<app>]     Flows, with an app open
//	#/settings[/<section>]
//	#/add-phone[/android|iphone]

import { useEffect, useState } from 'react';

export type Route =
  | { page: 'phones'; deviceId?: string }
  | { page: 'activity'; taskId?: string }
  | { page: 'flows'; app?: string }
  | { page: 'settings'; section?: string }
  | { page: 'add-phone'; platform?: string };

export function parseRoute(hash: string): Route {
  const parts = hash.replace(/^#\/?/, '').split('/').map(decodeURIComponent);
  switch (parts[0]) {
    case 'activity':
      return { page: 'activity', taskId: parts[1] || undefined };
    case 'flows':
      return { page: 'flows', app: parts[1] || undefined };
    case 'settings':
      return { page: 'settings', section: parts[1] || undefined };
    case 'add-phone':
      return { page: 'add-phone', platform: parts[1] || undefined };
    default:
      return { page: 'phones', deviceId: parts[1] || undefined };
  }
}

export function href(r: Route): string {
  const e = encodeURIComponent;
  switch (r.page) {
    case 'phones':
      return r.deviceId ? `#/phones/${e(r.deviceId)}` : '#/phones';
    case 'activity':
      return r.taskId ? `#/activity/${e(r.taskId)}` : '#/activity';
    case 'flows':
      return r.app ? `#/flows/${e(r.app)}` : '#/flows';
    case 'settings':
      return r.section ? `#/settings/${e(r.section)}` : '#/settings';
    case 'add-phone':
      return r.platform ? `#/add-phone/${e(r.platform)}` : '#/add-phone';
  }
}

export function navigate(r: Route, replace = false): void {
  const h = href(r);
  if (replace) window.history.replaceState(null, '', h);
  else window.location.hash = h;
  if (replace) window.dispatchEvent(new HashChangeEvent('hashchange'));
}

export function useRoute(): Route {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash));
  useEffect(() => {
    const on = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener('hashchange', on);
    return () => window.removeEventListener('hashchange', on);
  }, []);
  return route;
}
