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

export type AdminAccount = { email: string; created_at: number };

async function readError(response: Response): Promise<string> {
  const body = await response.json().catch(() => null) as { error?: string } | null;
  return typeof body?.error === 'string' ? body.error : `请求失败 (${response.status})`;
}

export async function currentAdmin(): Promise<{ email: string } | null> {
  const response = await fetch('/v1/session', { credentials: 'same-origin' });
  if (response.status === 401) return null;
  if (!response.ok) throw new Error(await readError(response));
  const body = await response.json() as { email?: string; kind?: string };
  if (!body.email) throw new Error('无法确认登录状态');
  if (typeof body.kind === 'string' && body.kind !== 'admin') return null;
  return { email: body.email };
}

export async function setupState(): Promise<{ needs_admin: boolean }> {
  const response = await fetch('/v1/setup', { credentials: 'same-origin' });
  if (!response.ok) throw new Error(await readError(response));
  const body = await response.json() as { needs_admin?: boolean };
  return { needs_admin: body.needs_admin === true };
}

async function submitAccount(path: string, email: string, password: string): Promise<{ email: string }> {
  const response = await fetch(path, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) throw new Error(await readError(response));
  const body = await response.json() as { email?: string };
  if (!body.email) throw new Error('操作没有完成');
  return { email: body.email };
}

export function login(email: string, password: string): Promise<{ email: string }> {
  return submitAccount('/v1/login', email, password);
}

export function registerAdmin(email: string, password: string): Promise<{ email: string }> {
  return submitAccount('/v1/register', email, password);
}

async function listAccounts(path: string): Promise<AdminAccount[]> {
  const response = await fetch(path, { credentials: 'same-origin' });
  if (!response.ok) throw new Error(await readError(response));
  const body = await response.json() as { accounts?: AdminAccount[] };
  return body.accounts ?? [];
}

export function listAdmins(): Promise<AdminAccount[]> {
  return listAccounts('/v1/accounts');
}

export function listDesktopAccounts(): Promise<AdminAccount[]> {
  return listAccounts('/v1/desktop-accounts');
}

export function createDesktopAccount(email: string, password: string): Promise<{ email: string }> {
  return submitAccount('/v1/desktop-accounts', email, password);
}

export async function registrationState(): Promise<{ open: boolean }> {
  const response = await fetch('/v1/registration', { credentials: 'same-origin' });
  if (!response.ok) throw new Error(await readError(response));
  const body = await response.json() as { open?: boolean };
  return { open: body.open === true };
}

export async function setRegistrationOpen(open: boolean): Promise<{ open: boolean }> {
  const response = await fetch('/v1/registration', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ open }),
  });
  if (!response.ok) throw new Error(await readError(response));
  const body = await response.json() as { open?: boolean };
  return { open: body.open === true };
}

export async function logout(): Promise<void> {
  const response = await fetch('/v1/logout', { method: 'POST', credentials: 'same-origin' });
  if (!response.ok) throw new Error(await readError(response));
}
