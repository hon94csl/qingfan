import assert from 'node:assert/strict';
import { test } from 'node:test';
import { getHealth, login } from './api.ts';

test('health validates responses and preserves HTTP errors', async () => {
  const original = globalThis.fetch;
  try {
    globalThis.fetch = async () => Response.json({ status: 'ok', service: 'qingfan-server' });
    assert.equal((await getHealth()).service, 'qingfan-server');
    globalThis.fetch = async () => new Response('bad gateway', { status: 502 });
    await assert.rejects(getHealth(), /502/);
    globalThis.fetch = async () => Response.json({ error: 'unavailable' }, { status: 503 });
    await assert.rejects(getHealth(), /unavailable/);
    globalThis.fetch = async () => Response.json({ status: 'ok', service: 'other' });
    await assert.rejects(getHealth(), /健康检查协议/);
    globalThis.fetch = async () => Response.json({ error: '邮箱或密码不正确' }, { status: 401 });
    await assert.rejects(login('ops@example.com', 'long-test-password'), /邮箱或密码不正确/);
  } finally { globalThis.fetch = original; }
});
