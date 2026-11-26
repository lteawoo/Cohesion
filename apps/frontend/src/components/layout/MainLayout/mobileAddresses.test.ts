import { expect, it } from 'vitest';
import { mobileAddresses } from './mobileAddresses';

it('uses the server listener port for distinct non-loopback network candidates', () => {
  expect(mobileAddresses(['localhost:3000','127.0.0.2:3000','[::1]:3000','0.0.0.0:3000',
    '10.0.0.8:8123','192.168.1.9:8123','10.0.0.8:8123','192.168.256.2:3000']))
    .toEqual(['http://10.0.0.8:8123/','http://192.168.1.9:8123/']);
});
