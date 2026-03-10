let refreshPromise: Promise<boolean> | null = null;
let redirectedToLogin = false;

interface ApiFetchOptions {
  skipAuthHandling?: boolean;
}

export interface ApiUploadOptions extends ApiFetchOptions {
  onUploadProgress?: (loaded: number, total: number) => void;
  signal?: AbortSignal;
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

function getRequestTarget(input: RequestInfo | URL): string {
  if (typeof input === 'string') {
    return input;
  }
  if (input instanceof URL) {
    return input.toString();
  }
  if (typeof Request !== 'undefined' && input instanceof Request) {
    return input.url;
  }
  return '';
}

function buildResponseFromXhr(xhr: XMLHttpRequest): Response {
  const headers = new Headers();
  const rawHeaders = xhr.getAllResponseHeaders().trim();
  if (rawHeaders) {
    rawHeaders.split(/\r?\n/).forEach((line) => {
      const separatorIndex = line.indexOf(':');
      if (separatorIndex <= 0) {
        return;
      }
      const headerName = line.slice(0, separatorIndex).trim();
      const headerValue = line.slice(separatorIndex + 1).trim();
      if (headerName) {
        headers.append(headerName, headerValue);
      }
    });
  }

  return new Response(xhr.responseText, {
    status: xhr.status,
    statusText: xhr.statusText,
    headers,
  });
}

function xhrRequest(
  input: RequestInfo | URL,
  init: RequestInit,
  options: ApiUploadOptions
): Promise<Response> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const method = init.method ?? 'GET';
    const url = getRequestTarget(input);

    if (!url) {
      reject(new Error('Invalid request URL'));
      return;
    }

    xhr.open(method, url, true);
    xhr.withCredentials = true;
    xhr.responseType = 'text';

    const headers = new Headers(init.headers ?? undefined);
    headers.forEach((value, key) => {
      xhr.setRequestHeader(key, value);
    });

    const handleAbort = () => {
      xhr.abort();
    };

    if (options.signal) {
      if (options.signal.aborted) {
        reject(new DOMException('The operation was aborted.', 'AbortError'));
        return;
      }
      options.signal.addEventListener('abort', handleAbort, { once: true });
    }

    xhr.upload.onprogress = (event) => {
      if (!options.onUploadProgress) {
        return;
      }
      options.onUploadProgress(event.loaded, event.lengthComputable ? event.total : 0);
    };

    xhr.onload = () => {
      options.signal?.removeEventListener('abort', handleAbort);
      resolve(buildResponseFromXhr(xhr));
    };
    xhr.onerror = () => {
      options.signal?.removeEventListener('abort', handleAbort);
      reject(new Error('Network request failed'));
    };
    xhr.onabort = () => {
      options.signal?.removeEventListener('abort', handleAbort);
      reject(new DOMException('The operation was aborted.', 'AbortError'));
    };

    xhr.send((init.body as XMLHttpRequestBodyInit | Document | null | undefined) ?? null);
  });
}

export async function apiUpload(
  input: RequestInfo | URL,
  init: RequestInit,
  options: ApiUploadOptions = {}
): Promise<Response> {
  const requestUrl = getRequestUrl(input);
  let response = await xhrRequest(input, init, options);
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

  response = await xhrRequest(input, init, options);
  if (response.status === 401) {
    redirectToLogin();
  } else {
    redirectedToLogin = false;
  }
  return response;
}
