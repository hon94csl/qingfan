// Adapted from tk-flywheel/frontend/src/api.ts; paths are explicit in Qingfan.
export async function getHealth(signal?: AbortSignal): Promise<{ status: string; service: string }> {
  const response = await fetch('/healthz', { credentials: 'same-origin', signal });
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    throw new Error(typeof body?.error === 'string' ? body.error : `请求失败 (${response.status})`);
  }
  const body = await response.json();
  if (body?.status !== 'ok' || body?.service !== 'qingfan-server') {
    throw new Error('服务响应不符合青帆健康检查协议');
  }
  return body;
}
