import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Alert, Button, Card, ConfigProvider, Descriptions, Form, Input, Layout, Menu, Switch, Table, Tag, Typography } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { createDesktopAccount, currentAdmin, getHealth, listAdmins, listDesktopAccounts, login, logout, registerAdmin, registrationState, setRegistrationOpen, setupState, type AdminAccount } from './api';
import './style.css';

type Page = 'status' | 'accounts' | 'desktop';
type PasswordForm = { email: string; password: string; confirm?: string };

function NavIcon({ d }: { d: string }) {
  return (
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" aria-hidden="true">
      <path d={d} stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function PageFrame({ title, actions, children }: { title: string; actions: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="screen">
      <header className="page-head">
        <h1>{title}</h1>
        <div className="page-actions">{actions}</div>
      </header>
      <section className="panel">{children}</section>
    </div>
  );
}

function Session({ email, leaving, onLogout }: { email: string; leaving: boolean; onLogout: () => void }) {
  return (
    <span className="header-user">
      {email}
      <Button type="link" loading={leaving} onClick={onLogout}>退出</Button>
    </span>
  );
}

function Admin() {
  const [ready, setReady] = useState(false);
  const [email, setEmail] = useState('');
  const [needsAdmin, setNeedsAdmin] = useState(false);
  const [bootError, setBootError] = useState('');
  const [page, setPage] = useState<Page>('status');

  useEffect(() => {
    let live = true;
    Promise.all([currentAdmin(), setupState()]).then(([user, setup]) => {
      if (!live) return;
      setEmail(user?.email ?? '');
      setNeedsAdmin(setup.needs_admin);
      setReady(true);
    }).catch((error: unknown) => {
      if (!live) return;
      setBootError(error instanceof Error ? error.message : '无法连接管理服务');
      setReady(true);
    });
    return () => { live = false; };
  }, []);

  if (!ready) return <main className="gate" />;
  if (!email) {
    return <Gate
      needsAdmin={needsAdmin}
      initialError={bootError}
      onSignedIn={(next) => { setEmail(next); setNeedsAdmin(false); setPage('status'); }}
    />;
  }
  return <Shell email={email} page={page} onPage={setPage} onLogout={() => { setEmail(''); setPage('status'); }} />;
}

function Gate({ needsAdmin, initialError, onSignedIn }: {
  needsAdmin: boolean;
  initialError: string;
  onSignedIn: (email: string) => void;
}) {
  const [error, setError] = useState(initialError);
  const [busy, setBusy] = useState(false);
  return <main className="gate">
    <Card>
      <div className="brand"><span className="dot">青</span><span className="word">青帆</span></div>
      <Typography.Title level={3} className="gate-title">{needsAdmin ? '创建管理员' : '登录'}</Typography.Title>
      <Typography.Paragraph type="secondary">
        {needsAdmin ? '第一个管理员可以进入管理后台，并继续添加管理员。' : '使用管理员账号进入青帆管理后台。'}
      </Typography.Paragraph>
      {error && <Alert type="error" showIcon title={error} style={{ marginBottom: 16 }} />}
      <Form<PasswordForm> layout="vertical" requiredMark={false} disabled={busy} onFinish={(values) => {
        setBusy(true);
        setError('');
        const action = needsAdmin ? registerAdmin(values.email, values.password) : login(values.email, values.password);
        action.then((account) => onSignedIn(account.email)).catch((err: unknown) => {
          setError(err instanceof Error ? err.message : '操作没有完成');
        }).finally(() => setBusy(false));
      }}>
        <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email', message: '请填写有效的邮箱' }]}>
          <Input autoComplete="username" />
        </Form.Item>
        <Form.Item name="password" label="密码" rules={[{ required: true, min: 12, max: 72, message: '密码至少 12 个字符' }]}>
          <Input.Password autoComplete={needsAdmin ? 'new-password' : 'current-password'} />
        </Form.Item>
        {needsAdmin && <Form.Item name="confirm" label="确认密码" dependencies={['password']} rules={[
          { required: true, message: '请再次填写密码' },
          ({ getFieldValue }) => ({ validator(_, value) {
            return !value || getFieldValue('password') === value ? Promise.resolve() : Promise.reject(new Error('两次输入的密码不一致'));
          } }),
        ]}>
          <Input.Password autoComplete="new-password" />
        </Form.Item>}
        <Button type="primary" htmlType="submit" block loading={busy} className="gate-submit">{needsAdmin ? '创建并进入' : '登录'}</Button>
      </Form>
    </Card>
  </main>;
}

function Shell({ email, page, onPage, onLogout }: {
  email: string;
  page: Page;
  onPage: (page: Page) => void;
  onLogout: () => void;
}) {
  const [leaving, setLeaving] = useState(false);
  function leave() {
    setLeaving(true);
    logout().then(onLogout).catch(() => setLeaving(false));
  }
  return <Layout className="shell">
    <Layout.Sider theme="light" breakpoint="lg" collapsedWidth={0} width={248}>
      <div className="brand"><span className="dot">青</span><span className="word">青帆</span></div>
      <Menu mode="inline" selectedKeys={[page]} onClick={({ key }) => onPage(key as Page)} items={[
        { key: 'status', icon: <NavIcon d="M4 6h16v12H4zM4 10h16" />, label: '服务状态' },
        { key: 'accounts', icon: <NavIcon d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />, label: '管理员' },
        { key: 'desktop', icon: <NavIcon d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z" />, label: '使用账号' },
      ]} />
    </Layout.Sider>
    <Layout>
      <Layout.Content className="content">
        {page === 'status' && <StatusPage email={email} leaving={leaving} onLogout={leave} />}
        {page === 'accounts' && <AccountsPage email={email} leaving={leaving} onLogout={leave} />}
        {page === 'desktop' && <DesktopAccountsPage email={email} leaving={leaving} onLogout={leave} />}
      </Layout.Content>
    </Layout>
  </Layout>;
}

function StatusPage({ email, leaving, onLogout }: { email: string; leaving: boolean; onLogout: () => void }) {
  const [reload, setReload] = useState(0);
  const [state, setState] = useState<{ loading: boolean; error?: string; service?: string }>({ loading: true });
  useEffect(() => {
    const controller = new AbortController();
    setState({ loading: true });
    getHealth(controller.signal).then(result => {
      if (!controller.signal.aborted) setState({ loading: false, service: result.service });
    }).catch(error => {
      if (!controller.signal.aborted) setState({ loading: false, error: String(error.message || error) });
    });
    return () => controller.abort();
  }, [reload]);
  return <PageFrame title="服务状态" actions={<>
    <Button loading={state.loading} onClick={() => setReload(value => value + 1)}>刷新</Button>
    <Session email={email} leaving={leaving} onLogout={onLogout} />
  </>}>
    <h2 className="panel-title">青帆服务</h2>
    {state.loading ? <p className="muted">读取中</p> : state.error
      ? <Alert type="error" showIcon title="连接失败" description={state.error} />
      : <Descriptions column={1} items={[
          { key: 'service', label: '服务', children: state.service },
          { key: 'status', label: '状态', children: <Tag className="status-tag">运行中</Tag> },
        ]} />}
  </PageFrame>;
}

function AccountsPage({ email, leaving, onLogout }: { email: string; leaving: boolean; onLogout: () => void }) {
  const [rows, setRows] = useState<AdminAccount[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [form] = Form.useForm<PasswordForm>();

  function reload() {
    setLoading(true);
    listAdmins().then((accounts) => { setRows(accounts); setError(''); }).catch((err: unknown) => {
      setError(err instanceof Error ? err.message : '无法读取管理员');
    }).finally(() => setLoading(false));
  }

  useEffect(() => { reload(); }, []);

  return <PageFrame title="管理员" actions={<Session email={email} leaving={leaving} onLogout={onLogout} />}>
    {error && <Alert type="error" showIcon title={error} style={{ marginBottom: 16 }} />}
    <h2 className="panel-title">账号</h2>
    <Table<AdminAccount> rowKey="email" loading={loading} pagination={false} dataSource={rows} columns={[
      { title: '邮箱', dataIndex: 'email' },
      { title: '创建时间', dataIndex: 'created_at', render: (value: number) => new Date(value * 1000).toLocaleString() },
    ]} />
    <h3 className="section-title">添加管理员</h3>
    <Form<PasswordForm> form={form} layout="vertical" style={{ maxWidth: 420 }} onFinish={(values) => {
        setError('');
        registerAdmin(values.email, values.password).then(() => {
          form.resetFields();
          reload();
        }).catch((err: unknown) => setError(err instanceof Error ? err.message : '没有添加成功'));
      }}>
        <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email', message: '请填写有效的邮箱' }]}>
          <Input autoComplete="off" />
        </Form.Item>
        <Form.Item name="password" label="密码" rules={[{ required: true, min: 12, max: 72, message: '密码至少 12 个字符' }]}>
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="confirm" label="确认密码" dependencies={['password']} rules={[
          { required: true, message: '请再次填写密码' },
          ({ getFieldValue }) => ({ validator(_, value) {
            return !value || getFieldValue('password') === value ? Promise.resolve() : Promise.reject(new Error('两次输入的密码不一致'));
          } }),
        ]}>
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Button type="primary" htmlType="submit">添加</Button>
      </Form>
  </PageFrame>;
}

function DesktopAccountsPage({ email, leaving, onLogout }: { email: string; leaving: boolean; onLogout: () => void }) {
  const [rows, setRows] = useState<AdminAccount[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<PasswordForm>();

  function reload() {
    setLoading(true);
    listDesktopAccounts().then((accounts) => { setRows(accounts); setError(''); }).catch((err: unknown) => {
      setError(err instanceof Error ? err.message : '无法读取使用账号');
    }).finally(() => setLoading(false));
  }

  useEffect(() => {
    reload();
    registrationState().then((state) => setOpen(state.open)).catch((err: unknown) => {
      setError(err instanceof Error ? err.message : '无法读取注册开关');
    });
  }, []);

  return <PageFrame title="使用账号" actions={<Session email={email} leaving={leaving} onLogout={onLogout} />}>
    {error && <Alert type="error" showIcon title={error} style={{ marginBottom: 16 }} />}
    <div className="setting-row">
      <div>
        <h2 className="panel-title">开放注册</h2>
        <p className="muted">打开后，桌面应用可以自行注册。关掉后，只能使用已经开通的账号。还没有收款。</p>
      </div>
      <Switch aria-label="开放注册" checked={open} loading={saving} onChange={(checked) => {
        setSaving(true);
        setError('');
        setRegistrationOpen(checked).then((state) => setOpen(state.open)).catch((err: unknown) => {
          setError(err instanceof Error ? err.message : '没有改成');
        }).finally(() => setSaving(false));
      }} />
    </div>
    <h2 className="panel-title">桌面登录</h2>
    <p className="muted" style={{ marginBottom: 16 }}>这些账号用来登录桌面应用。还没有收款，开通之后对方才能登录使用。</p>
    <Table<AdminAccount> rowKey="email" loading={loading} pagination={false} dataSource={rows} locale={{ emptyText: '还没有使用账号' }} columns={[
      { title: '邮箱', dataIndex: 'email' },
      { title: '创建时间', dataIndex: 'created_at', render: (value: number) => new Date(value * 1000).toLocaleString() },
    ]} />
    <h3 className="section-title">开通使用账号</h3>
    <Form<PasswordForm> form={form} layout="vertical" style={{ maxWidth: 420 }} onFinish={(values) => {
        setError('');
        createDesktopAccount(values.email, values.password).then(() => {
          form.resetFields();
          reload();
        }).catch((err: unknown) => setError(err instanceof Error ? err.message : '没有开通成功'));
      }}>
        <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email', message: '请填写有效的邮箱' }]}>
          <Input autoComplete="off" />
        </Form.Item>
        <Form.Item name="password" label="密码" rules={[{ required: true, min: 12, max: 72, message: '密码至少 12 个字符' }]}>
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="confirm" label="确认密码" dependencies={['password']} rules={[
          { required: true, message: '请再次填写密码' },
          ({ getFieldValue }) => ({ validator(_, value) {
            return !value || getFieldValue('password') === value ? Promise.resolve() : Promise.reject(new Error('两次输入的密码不一致'));
          } }),
        ]}>
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Button type="primary" htmlType="submit">开通</Button>
      </Form>
  </PageFrame>;
}

createRoot(document.getElementById('root')!).render(<React.StrictMode>
  <ConfigProvider locale={zhCN} theme={{
    token: {
      colorPrimary: '#1E88C4', colorPrimaryHover: '#1683C4', colorPrimaryActive: '#1683C4', colorInfo: '#1E88C4',
      colorText: '#090D0A', colorTextSecondary: '#416248', colorTextTertiary: '#71967C', colorTextLightSolid: '#FFFFFF',
      colorBorder: '#D1E0D7', colorBgLayout: '#EFF4F1', colorBgContainer: '#FFFFFF',
      borderRadius: 8, borderRadiusLG: 12, controlHeight: 40,
      controlOutline: 'rgba(30,136,196,0.18)',
      fontFamily: 'Inter, "PingFang SC", system-ui, -apple-system, "Microsoft YaHei", sans-serif',
    },
    components: {
      Layout: { siderBg: '#FFFFFF', bodyBg: '#EFF4F1', headerBg: 'transparent', headerHeight: 44 },
      Menu: {
        itemBg: 'transparent', itemColor: '#416248', itemHoverColor: '#090D0A', itemHoverBg: '#EEF3EC',
        itemSelectedBg: '#1E88C4', itemSelectedColor: '#FFFFFF', itemBorderRadius: 8, itemHeight: 44,
        itemMarginInline: 0, itemMarginBlock: 4, itemPaddingInline: 16, iconSize: 20, activeBarWidth: 0,
      },
      Button: {
        primaryColor: '#FFFFFF', fontWeight: 800, borderRadius: 12, controlHeight: 40,
        defaultColor: '#090D0A', defaultBg: 'transparent', defaultBorderColor: '#090D0A',
        defaultHoverBg: '#090D0A', defaultHoverColor: '#FFFFFF', defaultHoverBorderColor: '#090D0A',
        defaultActiveBg: '#090D0A', defaultActiveColor: '#FFFFFF', defaultActiveBorderColor: '#090D0A',
        primaryShadow: 'none', defaultShadow: 'none', contentFontSize: 16,
      },
      Card: { headerFontSize: 20, borderRadiusLG: 30, bodyPadding: 40, paddingLG: 40 },
      Input: {
        colorBgContainer: '#E7F3FB', activeBorderColor: '#1E88C4', hoverBorderColor: '#1E88C4',
        activeShadow: '0 0 0 2px rgba(30,136,196,0.18)',
      },
    },
  }}><Admin /></ConfigProvider>
</React.StrictMode>);
