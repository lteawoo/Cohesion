let refreshPromise: Promise<boolean> | null = null;
let redirectedToLogin = false;

interface ApiFetchOptions {
  skipAuthHandling?: boolean;
}

function getRequestUrl(input: RequestInfo | URL): string {
  if (typeof input === 'string') {
    return input;
  }
  if (input instanceof URL) {
    return input.pathname;
  }
  if (typeof Request !== 'undefined' && input instanceof Request) {
    return input.url;
  }
  return '';
}

function isAuthBypassPath(url: string): boolean {
  return (
    url.includes('/api/auth/login') ||
    url.includes('/api/auth/refresh') ||
    url.includes('/api/auth/logout') ||
    url.includes('/api/health')
  );
}

function redirectToLogin() {
  if (redirectedToLogin) {
    return;
  }
  redirectedToLogin = true;
  if (window.location.pathname !== '/login') {
    window.location.assign('/login');
    return;
  }
  window.dispatchEvent(new CustomEvent('cohesion:auth-expired'));
}

async function attemptRefreshToken(): Promise<boolean> {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    try {
      const response = await fetch('/api/auth/refresh', {
        method: 'POST',
        credentials: 'include',
      });
      return response.ok;
    } catch {
      return false;
    }
  })();

  const refreshed = await refreshPromise;
  refreshPromise = null;
  return refreshed;
}

export async function apiFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
  options: ApiFetchOptions = {}
): Promise<Response> {
  const requestUrl = getRequestUrl(input);
  const requestInit: RequestInit = {
    credentials: 'include',
    ...init,
  };

  let response = await fetch(input, requestInit);
  if (response.status !== 401 || options.skipAuthHandling || isAuthBypassPath(requestUrl)) {
    if (response.ok) {
      redirectedToLogin = false;
    }
    return response;
  }

  const refreshed = await attemptRefreshToken();
  if (!refreshed) {
    redirectToLogin();
    return response;
  }

  response = await fetch(input, requestInit);
  if (response.status === 401) {
    redirectToLogin();
  } else {
    redirectedToLogin = false;
  }
  return response;
}
