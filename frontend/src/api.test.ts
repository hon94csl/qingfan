import { expect, test } from 'vitest';
import { isAuthorizeURL } from './api';

test('授权地址只接受 TikTok Shop 官方授权页', () => {
  expect(isAuthorizeURL('https://services.tiktokshop.com/open/authorize?service_id=1')).toBe(true);
  expect(isAuthorizeURL('https://services.us.tiktokshop.com/open/authorize?service_id=1')).toBe(true);
  expect(isAuthorizeURL('https://example.com/open/authorize?service_id=1')).toBe(false);
  expect(isAuthorizeURL('http://services.tiktokshop.com/open/authorize?service_id=1')).toBe(false);
  expect(isAuthorizeURL('https://services.tiktokshop.com/other')).toBe(false);
});
