import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { CheckMediaTools } from '../wailsjs/go/main/App';
import { Workbench } from './Workbench';
import './style.css';

type ToolStatus = { name: string; path: string; version: string; error?: string };
function App() {
  const [tools, setTools] = useState<ToolStatus[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function check() {
    setBusy(true);
    setError('');
    try { setTools(await CheckMediaTools()); }
    catch { setError('环境检查失败。请通过 make dev 启动桌面应用后重试。'); }
    finally { setBusy(false); }
  }
  return <main>
    <p className="eyebrow">QINGFAN / 青帆</p>
    <h1>从商品素材，到可发布的视频</h1>
    <p>用素材库整理商品镜头，审核口播候选，再导出竖屏成片。</p>
    <section aria-labelledby="environment">
      <h2 id="environment">本地媒体环境</h2>
      <p>首次开发先确认 FFmpeg 和 ffprobe 可用。</p>
      <button onClick={check} disabled={busy}>{busy ? '检查中…' : '检查环境'}</button>
      {error && <p role="alert">{error}</p>}
      <div aria-live="polite">{tools.map(tool => <article key={tool.name}>
        <h3>{tool.name} · {tool.error ? '不可用' : '可用'}</h3>
        <p>{tool.error || tool.version}</p><code>{tool.path}</code>
      </article>)}</div>
    </section>
    <Workbench />
  </main>;
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><App /></React.StrictMode>);
