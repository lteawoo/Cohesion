export function publicLinkAddress(path: '/request' | '/share', token: string, publicOrigin?: string) {
  const url = new URL(path, publicOrigin || window.location.origin);
  url.hash = token;
  return url.href;
}
