// The server reports its direct HTTP listener and IPv4 interface addresses.
// Preserve its port even when the desktop is behind a proxy or Vite.
export function mobileAddresses(hosts: string[]): string[] {
  const addresses = new Set<string>();
  for (const host of hosts) {
    if (!/^\d{1,3}(?:\.\d{1,3}){3}:\d+$/.test(host)) continue;
    const [ip, port] = host.split(':');
    const octets = ip.split('.').map(Number);
    if (octets.some(n => n > 255) || octets[0] === 0 || octets[0] === 127 || octets[0] >= 224 ||
      Number(port) < 1 || Number(port) > 65535) continue;
    addresses.add(`http://${ip}:${port}/`);
  }
  return Array.from(addresses);
}
