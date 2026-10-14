import { useEffect, useState, type ReactNode } from 'react';
import { CheckMediaTools } from '../wailsjs/go/main/App';
import { backend, messageOf, type MetricPreview, type PlatformVideo, type ProduceResult, type Shop, type UsageView } from './api';

type Page = 'library' | 'produce';
type Source = 'compass' | 'local';
type ToolStatus = { name: string; path: string; version: string; error?: string };
type Material = { id: string; product: string; path: string; kind: string; imported: boolean; assetID: string; durationMS: number | null };

export function formatDuration(ms: number | null | undefined): string {
  if (ms == null || ms <= 0) return '—';
  const rounded = Math.round(ms / 100) / 10;
  const text = Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1);
  return `${text} 秒`;
}

const kindLabel: Record<string, string> = { clip: '镜头', voice: '配音', bgm: 'BGM', font: '字体' };

const glyphs = {
  library: 'M12 2 2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
  produce: 'M4 4h16v16H4zM4 9h16M9 4v5',
  tools: 'M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M2 14h4M10 8h4M18 16h4',
};

function NavGlyph({ d }: { d: string }) {
  return (
    <svg className="nav-icon" viewBox="0 0 24 24" width="20" height="20" fill="none" aria-hidden="true">
      <path d={d} stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function fileName(path: string) {
  const parts = path.split(/[/\\]/);
  return parts[parts.length - 1] || path;
}

export function videoLinks(text: string): string[] {
  const parts = text.trim().split(/\s+/).filter(Boolean);
  for (const part of parts) {
    let parsed: URL;
    try {
      parsed = new URL(part);
    } catch {
      throw new Error('先填写 http 或 https 视频链接');
    }
    if ((parsed.protocol !== 'http:' && parsed.protocol !== 'https:') || !parsed.hostname) {
      throw new Error('先填写 http 或 https 视频链接');
    }
    if (parsed.username || parsed.password) throw new Error('视频链接不能带账号');
  }
  return parts;
}

function countCell(value: number | null | undefined) {
  return value == null ? '—' : String(value);
}

function moneyCell(minor: number | null | undefined, currency: string) {
  if (minor == null || !currency) return '—';
  const code = currency.trim().toUpperCase();
  const scale = ['JPY', 'KRW', 'VND', 'CLP', 'ISK', 'PYG'].includes(code) ? 0
    : ['BHD', 'KWD', 'OMR'].includes(code) ? 3
      : /^[A-Z]{3}$/.test(code) ? 2 : -1;
  if (scale < 0) return '—';
  const sign = minor < 0 ? '-' : '';
  const abs = Math.abs(minor);
  if (scale === 0) return `${sign}${abs} ${code}`;
  const div = 10 ** scale;
  return `${sign}${Math.trunc(abs / div)}.${String(abs % div).padStart(scale, '0')} ${code}`;
}

function produceNotice(produced: ProduceResult) {
  const paths = (produced.outputs ?? []).map((item) => item.path).join('、');
  return [
    `已导出 ${produced.outputs?.length ?? 0} 条成片。`,
    paths,
    produced.note,
    `已知调用费用 ${produced.known_micros} 微单位，待对账 ${produced.unknown_calls} 次。`,
  ].filter(Boolean).join(' ');
}

function usageLine(report: UsageView) {
  if (report.billable_calls + report.cache_hits === 0) {
    return '还没有调用记录。没有换成货币，也没有按采用成片分摊。';
  }
  return `已知调用费用 ${report.known_micros} 微单位，待对账 ${report.unknown_calls} 次。没有换成货币，也没有按采用成片分摊。`;
}

function describePreview(preview: MetricPreview) {
  const sources = (preview.sources ?? []).join('、') || '未识别';
  const shops = (preview.shops ?? []).join('、') || '未识别';
  const zones = (preview.timezones ?? []).join('、') || '未识别';
  const currencies = (preview.currencies ?? []).join('、') || '未识别';
  const problems = (preview.errors ?? []).map((item) => item.message).join('；');
  const unbound = (preview.unbound ?? []).join('、');
  return [
    `来源 ${sources}`,
    `店铺 ${shops}`,
    `时区 ${zones}`,
    `币种 ${currencies}`,
    preview.repeated ? '这批报表已经导入过' : `可导入 ${preview.accepted} 行`,
    problems ? `错误：${problems}` : '',
    unbound ? `未绑定：${unbound}` : '',
  ].filter(Boolean).join('。');
}

export function Terminal({ initialEmail, registrationOpen }: { initialEmail?: string; registrationOpen?: boolean } = {}) {
  const api = backend();
  const known = initialEmail !== undefined;
  const [email, setEmail] = useState(initialEmail ?? '');
  const [booting, setBooting] = useState(!known);
  const [page, setPage] = useState<Page>('library');
  const [skuName, setSkuName] = useState('');
  const [videoLinkText, setVideoLinkText] = useState('');
  const [prompt, setPrompt] = useState('');
  const [importPath, setImportPath] = useState('');
  const [kind, setKind] = useState('clip');
  const [materials, setMaterials] = useState<Material[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [products, setProducts] = useState<Record<string, { projectID: string; skuID: string }>>({});
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [envOpen, setEnvOpen] = useState(false);
  const [tools, setTools] = useState<ToolStatus[]>([]);
  const [checking, setChecking] = useState(false);
  const [outputs, setOutputs] = useState<{ id: string; path: string }[]>([]);
  const [usage, setUsage] = useState('还没有调用记录。没有换成货币，也没有按采用成片分摊。');

  useEffect(() => {
    if (known) return;
    if (!api?.CurrentAccount) {
      setBooting(false);
      return;
    }
    let live = true;
    api.CurrentAccount().then((account) => {
      if (live) setEmail(account.email ?? '');
    }).catch(() => {
      if (live) setEmail('');
    }).finally(() => {
      if (live) setBooting(false);
    });
    return () => { live = false; };
  }, [api, known]);

  function leave() {
    const finish = () => {
      setEmail('');
      setPage('library');
      setSkuName('');
      setImportPath('');
      setMaterials([]);
      setSelected([]);
      setProducts({});
      setOutputs([]);
      setUsage('还没有调用记录。没有换成货币，也没有按采用成片分摊。');
      setNotice('');
      setError('');
      setEnvOpen(false);
      setTools([]);
    };
    if (!api) {
      finish();
      return;
    }
    api.Logout().then(finish).catch((err) => setError(messageOf(err)));
  }

  function open(next: Page) {
    setPage(next);
    setError('');
    setNotice('');
  }

  useEffect(() => {
    if (page !== 'produce' || !api?.Usage) return;
    let live = true;
    api.Usage().then((report) => {
      if (live) setUsage(usageLine(report));
    }).catch(() => undefined);
    return () => { live = false; };
  }, [api, page]);

  async function run(action: () => Promise<void>) {
    setError('');
    setNotice('');
    try { await action(); }
    catch (err) { setError(messageOf(err)); }
  }

  async function ensureSKU(name: string) {
    if (!api) throw new Error('自动出片需要在桌面应用里打开');
    const existing = products[name];
    if (existing) return existing;
    const project = await api.CreateProject(name, 'ID', 'id-ID');
    const sku = await api.CreateSKU(project.id, name, name);
    const ids = { projectID: project.id, skuID: sku.id };
    setProducts((current) => ({ ...current, [name]: ids }));
    return ids;
  }

  async function assetOf(item: Material, skuID: string) {
    if (!api) throw new Error('自动出片需要在桌面应用里打开');
    if (item.assetID) return item.assetID;
    const imported = await api.ImportFile(skuID, item.kind, 'owned', 'usable', item.path);
    const assetID = imported.asset_id;
    const durationMS = imported.duration_ms > 0 ? imported.duration_ms : item.durationMS;
    setMaterials((current) => current.map((row) => row.id === item.id ? { ...row, assetID, imported: true, durationMS } : row));
    return assetID;
  }

  function toggle(id: string) {
    setSelected((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  }

  async function checkTools() {
    setChecking(true);
    setError('');
    try { setTools(await CheckMediaTools()); }
    catch { setError('环境检查失败。请通过 make dev 启动桌面应用后重试。'); }
    finally { setChecking(false); }
  }

  if (booting) return <div className="terminal" />;
  if (!email) return <SignIn onSignedIn={setEmail} registrationOpen={registrationOpen} />;

  return (
    <div className="terminal">
      <aside className="rail">
        <div className="brand"><span className="mark" aria-hidden="true">青</span><strong>青帆</strong></div>
        <nav aria-label="工作终端">
          <button type="button" aria-current={page === 'library' ? 'page' : undefined} onClick={() => open('library')}>
            <NavGlyph d={glyphs.library} />素材库
          </button>
          <button type="button" aria-current={page === 'produce' ? 'page' : undefined} onClick={() => open('produce')}>
            <NavGlyph d={glyphs.produce} />出片
          </button>
        </nav>
        <div className="rail-foot">
          <p className="who">{email}</p>
          <button type="button" onClick={leave}>退出</button>
          <button type="button" aria-expanded={envOpen} onClick={() => setEnvOpen((open) => !open)}>
            <NavGlyph d={glyphs.tools} />媒体环境
          </button>
        </div>
      </aside>
      <main className="stage">
        {page === 'library' ? <Library
          banner={<>
            {error && <p role="alert">{error}</p>}
            {notice && <p className="notice">{notice}</p>}
            {envOpen && <Env tools={tools} checking={checking} onCheck={checkTools} />}
          </>}
          skuName={skuName}
          importPath={importPath}
          kind={kind}
          materials={materials}
          onSkuName={setSkuName}
          onPath={setImportPath}
          onKind={setKind}
          onPick={() => run(async () => {
            if (!api?.PickMaterial) throw new Error('选择文件需要在桌面应用里打开');
            const path = await api.PickMaterial();
            if (path) setImportPath(path);
          })}
          onImport={() => run(async () => {
            const product = skuName.trim();
            const path = importPath.trim();
            if (!product) throw new Error('先填写商品');
            if (!path) throw new Error('先填写文件路径');
            if (materials.some((item) => item.product === product && item.path === path)) {
              setNotice('这份素材已经在库里');
              return;
            }
            let imported = false;
            let assetID = '';
            let durationMS: number | null = null;
            if (api) {
              const ids = await ensureSKU(product);
              const saved = await api.ImportFile(ids.skuID, kind, 'owned', 'usable', path);
              imported = true;
              assetID = saved.asset_id;
              durationMS = saved.duration_ms > 0 ? saved.duration_ms : null;
            }
            setMaterials((current) => [...current, {
              id: `local-${current.length + 1}`,
              product,
              path,
              kind,
              imported,
              assetID,
              durationMS,
            }]);
            setImportPath('');
            setNotice(imported ? `已放入本地素材 ${path}` : '已列入素材库。文件导入需要在桌面应用里打开。');
          })}
        /> : <Produce
          banner={<>
            {error && <p role="alert">{error}</p>}
            {notice && <p className="notice">{notice}</p>}
            {envOpen && <Env tools={tools} checking={checking} onCheck={checkTools} />}
          </>}
          skuName={skuName}
          videoLinkText={videoLinkText}
          prompt={prompt}
          materials={materials}
          selected={selected}
          outputs={outputs}
          usage={usage}
          onSkuName={setSkuName}
          onVideoLinks={setVideoLinkText}
          onPrompt={setPrompt}
          onToggle={toggle}
          onRun={run}
          onNotice={setNotice}
          onStart={() => run(async () => {
            const product = skuName.trim();
            if (!product) throw new Error('先填写商品');
            const urls = videoLinks(videoLinkText);
            const promptText = prompt.trim();
            const chosen = materials.filter((item) => item.product === product && selected.includes(item.id));
            if (urls.length > 0) {
              if (!promptText) throw new Error('先填写提示词');
              if (!api?.MixFromLinks) throw new Error('自动混剪需要在桌面应用里打开');
              const ids = await ensureSKU(product);
              const font = chosen.find((item) => item.kind === 'font');
              const bgm = chosen.find((item) => item.kind === 'bgm');
              const fontID = font ? await assetOf(font, ids.skuID) : '';
              const bgmID = bgm ? await assetOf(bgm, ids.skuID) : '';
              const produced = await api.MixFromLinks(ids.projectID, ids.skuID, fontID, bgmID, promptText, urls);
              const fetched = produced.fetched ?? [];
              if (fetched.length > 0) {
                setMaterials((current) => {
                  const next = [...current];
                  for (const item of fetched) {
                    if (next.some((row) => row.product === product && row.path === item.path)) continue;
                    next.push({
                      id: `fetched-${item.asset_id}`,
                      product,
                      path: item.path,
                      kind: 'clip',
                      imported: true,
                      assetID: item.asset_id,
                      durationMS: item.duration_ms > 0 ? item.duration_ms : null,
                    });
                  }
                  return next;
                });
              }
              setOutputs(produced.outputs ?? []);
              if (api.Usage) setUsage(usageLine(await api.Usage()));
              setNotice(produceNotice(produced));
              return;
            }
            if (chosen.length === 0) throw new Error('先选择素材');
            if (!api) throw new Error('自动出片需要在桌面应用里打开');
            if (!chosen.some((item) => item.kind === 'clip')) throw new Error('先选择镜头');
            const font = chosen.find((item) => item.kind === 'font');
            const bgm = chosen.find((item) => item.kind === 'bgm');
            if (!font || !bgm) throw new Error('先选择字体和 BGM');
            const ids = await ensureSKU(product);
            const fontID = await assetOf(font, ids.skuID);
            const bgmID = await assetOf(bgm, ids.skuID);
            const clipAssetIDs: string[] = [];
            for (const item of chosen) {
              if (item.kind === 'font' || item.kind === 'bgm') continue;
              const assetID = await assetOf(item, ids.skuID);
              if (item.kind === 'clip') clipAssetIDs.push(assetID);
            }
            const produced = await api.ProduceVideo(ids.projectID, ids.skuID, fontID, bgmID, clipAssetIDs, promptText);
            setOutputs(produced.outputs ?? []);
            if (api.Usage) setUsage(usageLine(await api.Usage()));
            setNotice(produceNotice(produced));
          })}
        />}
      </main>
    </div>
  );
}

function SignIn({ onSignedIn, registrationOpen }: { onSignedIn: (email: string) => void; registrationOpen?: boolean }) {
  const api = backend();
  const [open, setOpen] = useState(registrationOpen === true);
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const registering = open && mode === 'register';

  useEffect(() => {
    if (registrationOpen !== undefined) {
      setOpen(registrationOpen);
      return;
    }
    const read = () => {
      if (!api?.RegistrationOpen) {
        setOpen(false);
        return;
      }
      api.RegistrationOpen().then(setOpen).catch(() => setOpen(false));
    };
    read();
    window.addEventListener('focus', read);
    return () => window.removeEventListener('focus', read);
  }, [api, registrationOpen]);

  useEffect(() => {
    if (!open) setMode('login');
  }, [open]);

  return (
    <div className="terminal gate">
      <section className="panel sign-in" aria-label={registering ? '注册' : '登录'}>
        <h1>{registering ? '注册' : '登录'}</h1>
        <p>{registering
          ? '注册后即可使用。还没有收款。'
          : open
            ? '可以登录，也可以注册一个使用账号。还没有收款。'
            : '账号由管理后台开通。登录后才能使用。'}</p>
        {error && <p role="alert">{error}</p>}
        <form onSubmit={(event) => {
          event.preventDefault();
          setBusy(true);
          setError('');
          if (registering && password !== confirm) {
            setError('两次输入的密码不一致');
            setBusy(false);
            return;
          }
          const run = !api
            ? Promise.reject(new Error('请在桌面窗口里登录'))
            : registering
              ? api.Register(email, password)
              : api.Login(email, password);
          run.then((account) => onSignedIn(account.email)).catch((err: unknown) => {
            setError(messageOf(err));
          }).finally(() => setBusy(false));
        }}>
          <label>邮箱<input type="email" autoComplete="username" value={email} onChange={(event) => setEmail(event.target.value)} /></label>
          <label>密码<input type="password" autoComplete={registering ? 'new-password' : 'current-password'} value={password} onChange={(event) => setPassword(event.target.value)} /></label>
          {registering && <label>确认密码<input type="password" autoComplete="new-password" value={confirm} onChange={(event) => setConfirm(event.target.value)} /></label>}
          <button type="submit" className="primary" disabled={busy}>{busy ? '提交中…' : registering ? '注册' : '登录'}</button>
        </form>
        {open && <button type="button" className="text" onClick={() => {
          setMode(registering ? 'login' : 'register');
          setError('');
        }}>{registering ? '返回登录' : '注册账号'}</button>}
      </section>
    </div>
  );
}

function Env({ tools, checking, onCheck }: { tools: ToolStatus[]; checking: boolean; onCheck: () => void }) {
  return (
    <section className="env" aria-label="媒体环境">
      <h2>媒体环境</h2>
      <p>确认这台机器上的 FFmpeg 和 ffprobe。</p>
      <button type="button" onClick={onCheck} disabled={checking}>{checking ? '检查中…' : '检查环境'}</button>
      <div aria-live="polite">{tools.map((tool) => <article key={tool.name}>
        <h3>{tool.name} · {tool.error ? '不可用' : '可用'}</h3>
        <p>{tool.error || tool.version}</p><code>{tool.path}</code>
      </article>)}</div>
    </section>
  );
}

function Library({ banner, skuName, importPath, kind, materials, onSkuName, onPath, onKind, onPick, onImport }: {
  banner: ReactNode;
  skuName: string;
  importPath: string;
  kind: string;
  materials: Material[];
  onSkuName: (value: string) => void;
  onPath: (value: string) => void;
  onKind: (value: string) => void;
  onPick: () => void;
  onImport: () => void;
}) {
  const api = backend();
  const [source, setSource] = useState<Source>('compass');
  const [shops, setShops] = useState<Shop[]>([]);
  const [videos, setVideos] = useState<PlatformVideo[]>([]);
  const [shopName, setShopName] = useState('');
  const [platformError, setPlatformError] = useState('');
  const [platformNote, setPlatformNote] = useState('');
  useEffect(() => {
    if (!api?.ListShops && !api?.ListPlatformVideos) return;
    let live = true;
    const shopCall = api.ListShops ? api.ListShops() : Promise.resolve([]);
    const videoCall = api.ListPlatformVideos ? api.ListPlatformVideos() : Promise.resolve([]);
    Promise.all([shopCall, videoCall]).then(([shopRows, videoRows]) => {
      if (!live) return;
      setShops(shopRows ?? []);
      setVideos(videoRows ?? []);
    }).catch((error: unknown) => {
      if (live) setPlatformError(messageOf(error));
    });
    return () => { live = false; };
  }, [api]);
  async function createShop() {
    setPlatformNote('');
    const name = shopName.trim();
    if (!name) {
      setPlatformError('先填写店铺');
      return;
    }
    if (!api?.CreateShop) {
      setPlatformError('创建店铺需要在桌面应用里打开');
      return;
    }
    try {
      const shop = await api.CreateShop(name);
      setShops((current) => [...current.filter((item) => item.id !== shop.id), shop].sort((a, b) => a.name.localeCompare(b.name, 'zh')));
      setShopName('');
      setPlatformError('');
      setPlatformNote('店铺已创建。授权和数据回流等接口开通后再绑定。');
    } catch (error) {
      setPlatformError(messageOf(error));
    }
  }
  return (
    <div className="console">
      <header className="page-head">
        <h1>素材库</h1>
        <div className="segmented" role="tablist" aria-label="素材来源">
          <button type="button" role="tab" aria-selected={source === 'compass'} onClick={() => setSource('compass')}>平台罗盘</button>
          <button type="button" role="tab" aria-selected={source === 'local'} onClick={() => setSource('local')}>本地素材</button>
        </div>
      </header>
      {source === 'compass' ? (
        <section className="panel" aria-label="云端罗盘">
          {banner}
          <h2 className="panel-title">平台罗盘</h2>
          <p className="sub">登录后的账号自己创建店铺，一个账号可以创建多家。店铺授权和数据回流等接口开通后再绑定。不是罗盘页面抓取。</p>
          {platformError && <p role="alert">{platformError}</p>}
          {platformNote && <p className="notice">{platformNote}</p>}
          <div className="report-fields">
            <label className="wide">店铺<input value={shopName} autoComplete="off" onChange={(event) => setShopName(event.target.value)} /></label>
          </div>
          <div className="report-actions">
            <button type="button" className="primary" onClick={() => { void createShop(); }}>创建店铺</button>
          </div>
          <table>
            <thead>
              <tr><th>店铺</th></tr>
            </thead>
            <tbody>
              {shops.length === 0 ? <tr><td className="empty">还没有店铺</td></tr> : shops.map((shop) => (
                <tr key={shop.id}>
                  <td>{shop.name}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <table>
            <thead>
              <tr><th>素材</th><th>商品</th><th>播放</th><th>订单</th><th>成交额</th></tr>
            </thead>
            <tbody>
              {videos.length === 0 ? <tr><td className="empty" colSpan={5}>还没有平台素材</td></tr> : videos.map((video) => (
                <tr key={`${video.shop_name}-${video.video_id}-${video.start}-${video.end}`}>
                  <td>{video.shop_name ? `${video.shop_name} · ` : ''}{video.title || video.video_id}</td>
                  <td>—</td>
                  <td>{countCell(video.views)}</td>
                  <td>{countCell(video.orders)}</td>
                  <td>{moneyCell(video.gmv_minor, video.currency)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ) : (
        <section className="panel" aria-label="本地素材">
          {banner}
          <h2 className="panel-title">本地素材</h2>
          <div className="add-row">
            <label>商品<input value={skuName} onChange={(event) => onSkuName(event.target.value)} /></label>
            <label>文件路径<span className="path-line"><input value={importPath} onChange={(event) => onPath(event.target.value)} /><button type="button" onClick={onPick}>选择文件</button></span></label>
            <label>类型
              <select value={kind} onChange={(event) => onKind(event.target.value)}>
                <option value="clip">镜头</option>
                <option value="voice">配音</option>
                <option value="bgm">BGM</option>
                <option value="font">字体</option>
              </select>
            </label>
            <button type="button" className="primary" onClick={onImport}>放入素材库</button>
          </div>
          <table>
            <thead>
              <tr><th>素材</th><th>类型</th><th>商品</th><th>时长</th></tr>
            </thead>
            <tbody>
              {materials.length === 0 ? <tr><td className="empty" colSpan={4}>还没有本地文件</td></tr> : materials.map((item) => (
                <tr key={item.id}>
                  <td>{fileName(item.path)}</td>
                  <td>{kindLabel[item.kind] ?? item.kind}</td>
                  <td>{item.product}</td>
                  <td>{formatDuration(item.durationMS)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  );
}

function Produce({ banner, skuName, videoLinkText, prompt, materials, selected, outputs, usage, onSkuName, onVideoLinks, onPrompt, onToggle, onRun, onNotice, onStart }: {
  banner: ReactNode;
  skuName: string;
  videoLinkText: string;
  prompt: string;
  materials: Material[];
  selected: string[];
  outputs: { id: string; path: string }[];
  usage: string;
  onSkuName: (value: string) => void;
  onVideoLinks: (value: string) => void;
  onPrompt: (value: string) => void;
  onToggle: (id: string) => void;
  onRun: (action: () => Promise<void>) => void;
  onNotice: (text: string) => void;
  onStart: () => void;
}) {
  const api = backend();
  const product = skuName.trim();
  const matches = materials.filter((item) => item.product === product);
  const [outputID, setOutputID] = useState('');
  const [publishURL, setPublishURL] = useState('');
  const [videoID, setVideoID] = useState('');
  const [shop, setShop] = useState('');
  const [publishedAt, setPublishedAt] = useState('');
  const [csvText, setCsvText] = useState('');
  const [feedback, setFeedback] = useState('');
  useEffect(() => {
    if (outputs[0] && !outputID) setOutputID(outputs[0].id);
  }, [outputs, outputID]);
  return (
    <div className="console">
      <header className="page-head">
        <h1>出片</h1>
        <button type="button" className="primary" onClick={onStart}>开始自动混剪</button>
      </header>
      <section className="panel" aria-label="自动出片">
        {banner}
        <h2 className="panel-title">自动出片</h2>
        <p className="sub">填写商品、视频链接和提示词，开始自动混剪。链接需要是可以直接下载的视频文件。不填链接时，改为勾选镜头、字体和 BGM。</p>
        <div className="fields">
          <label>商品<input value={skuName} onChange={(event) => onSkuName(event.target.value)} /></label>
          <label>视频链接<textarea value={videoLinkText} onChange={(event) => onVideoLinks(event.target.value)} /></label>
          <label>提示词<textarea value={prompt} onChange={(event) => onPrompt(event.target.value)} /></label>
        </div>
        <div className="pick-block">
          <p className="sub">选镜头是另一种做法。勾选这件商品已有的镜头、字体和 BGM。</p>
          {!product ? <p className="empty-pick">先填写商品，再选择素材</p> : matches.length === 0 ? (
            <p className="empty-pick">素材库里还没有这件商品的素材</p>
          ) : (
            <ul className="picks">
              {matches.map((item) => (
                <li key={item.id}>
                  <label>
                    <input
                      type="checkbox"
                      aria-label={fileName(item.path)}
                      checked={selected.includes(item.id)}
                      onChange={() => onToggle(item.id)}
                    />
                    <span className="pick-name">{fileName(item.path)}</span>
                    <span className="pick-meta">本地 · {kindLabel[item.kind] ?? item.kind} · {formatDuration(item.durationMS)}</span>
                  </label>
                </li>
              ))}
            </ul>
          )}
        </div>
        <section className="report-block" aria-label="发布与回流">
          <h2 className="panel-title">发布与回流</h2>
          <p className="sub">{usage}</p>
          <div className="report-fields">
            {outputs.length > 0 && (
              <label className="wide">本次成片
                <select value={outputID} onChange={(event) => setOutputID(event.target.value)}>
                  {outputs.map((item) => <option key={item.id} value={item.id}>{fileName(item.path)}</option>)}
                </select>
              </label>
            )}
            <label>成片编号<input value={outputID} onChange={(event) => setOutputID(event.target.value)} /></label>
            <label>发布链接<input value={publishURL} onChange={(event) => setPublishURL(event.target.value)} /></label>
            <label>视频编号<input value={videoID} onChange={(event) => setVideoID(event.target.value)} /></label>
            <label>店铺<input value={shop} onChange={(event) => setShop(event.target.value)} /></label>
            <label className="wide">发布时间<input value={publishedAt} onChange={(event) => setPublishedAt(event.target.value)} /></label>
            <label className="wide">回流 CSV<textarea value={csvText} onChange={(event) => setCsvText(event.target.value)} /></label>
          </div>
          <div className="report-actions">
            <button type="button" onClick={() => onRun(async () => {
              if (!api) throw new Error('绑定发布需要在桌面应用里打开');
              const saved = await api.BindPublication(outputID, publishURL, videoID, shop, publishedAt);
              onNotice(`已绑定视频 ${saved.video_id}`);
            })}>绑定发布</button>
            <button type="button" onClick={() => onRun(async () => {
              if (!api) throw new Error('预览回流需要在桌面应用里打开');
              setFeedback(describePreview(await api.PreviewMetrics(csvText)));
            })}>预览回流</button>
            <button type="button" onClick={() => onRun(async () => {
              if (!api) throw new Error('导入回流需要在桌面应用里打开');
              const imported = await api.ImportMetrics(csvText);
              setFeedback(describePreview(imported));
              onNotice(imported.repeated ? '这批报表已经导入过' : `已导入 ${imported.accepted} 行`);
            })}>导入回流</button>
          </div>
          {feedback && <p className="notice">{feedback}</p>}
        </section>
      </section>
    </div>
  );
}
