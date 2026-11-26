import { expect, it } from 'vitest';
import { publicLinkAddress } from './publicLink';

it.each(['/request', '/share'] as const)('uses configured or legacy origin for %s', (path: '/request' | '/share') => {
  expect(publicLinkAddress(path, 'abc-123', 'https://files.example.com:8443')).toBe(`https://files.example.com:8443${path}#abc-123`);
  expect(publicLinkAddress(path, 'abc-123')).toBe(`${window.location.origin}${path}#abc-123`);
});
