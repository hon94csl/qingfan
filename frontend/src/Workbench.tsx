import { useState } from 'react';
import { backend, messageOf, type Asset, type Clip, type Draft } from './api';
import { SegmentCard } from './components/SegmentCard';
import { toDraftSegment, type Segment } from './components/segment';

const emptySegment = (): Segment => ({
  id: 'seg_1',
  voiceText: '',
  voiceAssetId: '',
  shots: [{ clipId: '', sourceInMS: 0, sourceOutMS: 0 }],
  captions: [{ text: '', startMS: 0, endMS: 0 }],
});

export function Workbench() {
  const api = backend();
  const [projectName, setProjectName] = useState('演示项目');
  const [skuName, setSkuName] = useState('演示商品');
  const [projectID, setProjectID] = useState('');
  const [skuID, setSkuID] = useState('');
  const [importPath, setImportPath] = useState('');
  const [kind, setKind] = useState('clip');
  const [source, setSource] = useState('owned');
  const [clips, setClips] = useState<Clip[]>([]);
  const [voices, setVoices] = useState<Asset[]>([]);
  const [fonts, setFonts] = useState<Asset[]>([]);
  const [bgms, setBgms] = useState<Asset[]>([]);
  const [fontID, setFontID] = useState('');
  const [bgmID, setBgmID] = useState('');
  const [segment, setSegment] = useState<Segment>(emptySegment());
  const [draft, setDraft] = useState<Draft | null>(null);
  const [previewPath, setPreviewPath] = useState('outputs/preview.mp4');
  const [exportPath, setExportPath] = useState('outputs/export.mp4');
  const [outputs, setOutputs] = useState<{ id: string; path: string; draft_revision: number }[]>([]);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState(api ? '' : '当前是浏览器预览。保存、预览和导出需要在桌面应用里打开。');

  async function run(action: () => Promise<void>) {
    setError('');
    try { await action(); }
    catch (err) { setError(messageOf(err)); }
  }

  async function refresh(id: string) {
    if (!api) return;
    const [nextClips, nextVoices, nextFonts, nextBgms] = await Promise.all([
      api.ListClips(id),
      api.ListAssets(id, 'voice'),
      api.ListAssets(id, 'font'),
      api.ListAssets(id, 'bgm'),
    ]);
    setClips(nextClips ?? []);
    setVoices(nextVoices ?? []);
    setFonts(nextFonts ?? []);
    setBgms(nextBgms ?? []);
  }

  function draftFrom(current: Draft | null): Draft {
    return {
      id: current?.id ?? '',
      project_id: projectID,
      sku_id: skuID,
      revision: current?.revision ?? 0,
      market: 'ID',
      locale: 'id-ID',
      font_asset_id: fontID,
      bgm_asset_id: bgmID,
      bgm_gain_db: -18,
      segments: [toDraftSegment(segment)],
    };
  }

  const clipOptions = clips.map((clip) => ({
    id: clip.id,
    label: `${clip.label || clip.id} ${clip.source_in_ms}–${clip.source_out_ms}`,
    selectable: clip.selectable,
    reason: clip.reason,
    inMS: clip.source_in_ms,
    outMS: clip.source_out_ms,
  }));
  const voiceOptions = voices.map((voice) => ({
    id: voice.id,
    label: `${voice.path || voice.id} ${voice.duration_ms}ms`,
    durationMS: voice.duration_ms,
    selectable: voice.selectable,
    reason: voice.reason,
  }));
  const payload = toDraftSegment(segment);

  return (
    <section aria-labelledby="workbench">
      <h2 id="workbench">手动工作台</h2>
      <p>用固定素材和预录配音完成一条成片。这里不调用模型。</p>
      {notice && <p>{notice}</p>}
      <div className="grid">
        <label>项目名称<input value={projectName} onChange={(event) => setProjectName(event.target.value)} /></label>
        <label>商品名称<input value={skuName} onChange={(event) => setSkuName(event.target.value)} /></label>
        <button type="button" onClick={() => run(async () => {
          if (!api) return;
          const project = await api.CreateProject(projectName, 'ID', 'id-ID');
          const sku = await api.CreateSKU(project.id, skuName, '');
          setProjectID(project.id);
          setSkuID(sku.id);
          setNotice(`已创建商品 ${sku.name}`);
        })}>创建项目和商品</button>
      </div>
      <div className="grid">
        <label>文件路径<input value={importPath} onChange={(event) => setImportPath(event.target.value)} /></label>
        <label>类型
          <select value={kind} onChange={(event) => setKind(event.target.value)}>
            <option value="clip">镜头</option>
            <option value="voice">配音</option>
            <option value="bgm">BGM</option>
            <option value="font">字体</option>
          </select>
        </label>
        <label>来源
          <select value={source} onChange={(event) => setSource(event.target.value)}>
            <option value="owned">自有</option>
            <option value="library">素材库</option>
            <option value="reference">参考</option>
          </select>
        </label>
        <button type="button" onClick={() => run(async () => {
          if (!api || !skuID) throw new Error('先创建商品');
          await api.ImportFile(skuID, kind, source, source === 'reference' ? 'reference_only' : 'usable', importPath);
          await refresh(skuID);
          setNotice('导入完成');
        })}>导入</button>
      </div>
      <div className="grid">
        <label>字体
          <select value={fontID} onChange={(event) => setFontID(event.target.value)}>
            <option value="">选择字体</option>
            {fonts.map((font) => <option key={font.id} value={font.id} disabled={!font.selectable}>{font.path || font.id}</option>)}
          </select>
        </label>
        <label>BGM
          <select value={bgmID} onChange={(event) => setBgmID(event.target.value)}>
            <option value="">选择 BGM</option>
            {bgms.map((bgm) => <option key={bgm.id} value={bgm.id} disabled={!bgm.selectable}>{bgm.path || bgm.id}</option>)}
          </select>
        </label>
      </div>
      <SegmentCard segment={segment} clips={clipOptions} voices={voiceOptions} error={error} onChange={setSegment} />
      <p>将传给导出的镜头：{payload.shots.map((shot) => `${shot.clip_id || '未选'} ${shot.source_in_ms}–${shot.source_out_ms}`).join('，') || '还没有镜头'}</p>
      <div className="grid">
        <button type="button" onClick={() => run(async () => {
          if (!api) return;
          const saved = await api.SaveDraft(draftFrom(draft));
          setDraft(saved);
          setNotice(`草稿已保存，修订 ${saved.revision}`);
        })}>保存草稿</button>
        <label>预览路径<input value={previewPath} onChange={(event) => setPreviewPath(event.target.value)} /></label>
        <button type="button" onClick={() => run(async () => {
          if (!api || !draft) throw new Error('先保存草稿');
          await api.PreviewDraft(draft.id, previewPath);
          setNotice(`预览已写入 ${previewPath}`);
        })}>预览</button>
        <label>导出路径<input value={exportPath} onChange={(event) => setExportPath(event.target.value)} /></label>
        <button type="button" onClick={() => run(async () => {
          if (!api || !draft) throw new Error('先保存草稿');
          const output = await api.ExportDraft(draft.id, exportPath);
          setOutputs(await api.ListOutputs(draft.id));
          setNotice(`成片已写入 ${output.path}`);
        })}>导出</button>
      </div>
      {outputs.length > 0 && <ul>{outputs.map((output) => <li key={output.id}>修订 {output.draft_revision}：{output.path}</li>)}</ul>}
    </section>
  );
}
