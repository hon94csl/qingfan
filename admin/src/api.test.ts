import assert from 'node:assert/strict';
import { test } from 'node:test';
import { getHealth } from './api.ts';

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
  } finally { globalThis.fetch = original; }
});
