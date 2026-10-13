import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Alert, Button, Card, ConfigProvider, Descriptions, Layout, Menu, Tag, Typography } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { getHealth } from './api';
import './style.css';

// Layout and theme adapted from tk-flywheel; see docs/admin-reuse.md.
function Admin() {
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
  return <Layout className="shell">
    <Layout.Sider theme="light" breakpoint="lg" collapsedWidth={0} width={240}>
      <div className="brand"><span className="dot">青</span>青帆<Tag>管理后台</Tag></div>
      <Menu mode="inline" selectedKeys={['status']} items={[{ key: 'status', label: '服务状态' }]} />
    </Layout.Sider>
    <Layout>
      <Layout.Header className="header"><strong>青帆管理后台</strong><Tag>本地开发</Tag></Layout.Header>
      <Layout.Content className="content">
        <div className="heading"><Typography.Title level={2}>服务状态</Typography.Title>
          <Button loading={state.loading} onClick={() => setReload(value => value + 1)}>刷新</Button></div>
        <Card title="青帆服务" loading={state.loading}>
          {state.error ? <Alert type="error" showIcon title="连接失败" description={state.error} /> :
            <Descriptions column={1} items={[
              { key: 'service', label: '服务', children: state.service },
              { key: 'status', label: '状态', children: <Tag color="success">运行中</Tag> },
            ]} />}
        </Card>
        <Card title="后续接入"><Typography.Paragraph>
          用量记录在服务端数据库。这个页面只检查服务是否在运行，不保存供应商密钥。视频混剪在桌面端完成。
        </Typography.Paragraph></Card>
      </Layout.Content>
    </Layout>
  </Layout>;
}
createRoot(document.getElementById('root')!).render(<React.StrictMode>
  <ConfigProvider locale={zhCN} theme={{
    token: { colorPrimary: '#0891b2', colorInfo: '#0891b2', colorText: '#0f172a', colorTextSecondary: '#64748b', colorBorder: '#e2e8f0', colorBgLayout: '#f4f8fb', borderRadius: 10, borderRadiusLG: 16, fontFamily: 'Inter, system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif' },
    components: { Layout: { siderBg: '#fff', bodyBg: '#f4f8fb' }, Menu: { itemSelectedBg: '#ecfeff', itemSelectedColor: '#0e7490', itemBorderRadius: 12, itemHeight: 40 }, Card: { headerFontSize: 15 } },
  }}><Admin /></ConfigProvider>
</React.StrictMode>);
