import { useState } from 'react';
import { backend, messageOf, type Asset, type Candidate, type Clip, type Draft, type MetricPreview, type Publication, type ShotNote } from './api';
import { ScriptReview } from './components/ScriptReview';
import { SegmentCard } from './components/SegmentCard';
import { mergeDraftSegments, segmentFromDraft, toDraftSegment, type Segment } from './components/segment';

const emptySegment = (): Segment => ({
  id: 'seg_1',
  voiceText: '',
  voiceAssetId: '',
  shots: [{ clipId: '', sourceInMS: 0, sourceOutMS: 0 }],
  captions: [{ text: '', startMS: 0, endMS: 0 }],
});

function describePreview(preview: MetricPreview): string {
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

export function Workbench() {
  const api = backend();
  const [projectName, setProjectName] = useState('演示项目');
  const [skuName, setSkuName] = useState('演示商品');
  const [facts, setFacts] = useState('');
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [notes, setNotes] = useState<ShotNote[]>([]);
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
  const [outputID, setOutputID] = useState('');
  const [publishURL, setPublishURL] = useState('');
  const [videoID, setVideoID] = useState('');
  const [shop, setShop] = useState('');
  const [publishedAt, setPublishedAt] = useState('');
  const [csvText, setCsvText] = useState('');
  const [publications, setPublications] = useState<Publication[]>([]);
  const [feedback, setFeedback] = useState('');
  const [notice, setNotice] = useState(api ? '' : '当前是浏览器预览。保存、预览、导出、生成候选、发布绑定和回流预览需要在桌面应用里打开。');

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
      hook_type: current?.hook_type ?? '',
      prompt_version: current?.prompt_version ?? '',
      segments: mergeDraftSegments(segment, current?.segments),
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
      <p>填写已确认的商品事实后可以生成口播候选。固定回译需要人工核对，也可以继续手工配镜。</p>
      {notice && <p>{notice}</p>}
      <div className="grid">
        <label>项目名称<input value={projectName} onChange={(event) => setProjectName(event.target.value)} /></label>
        <label>商品名称<input value={skuName} onChange={(event) => setSkuName(event.target.value)} /></label>
        <label>已确认的商品事实<textarea value={facts} onChange={(event) => setFacts(event.target.value)} /></label>
        <button type="button" onClick={() => run(async () => {
          if (!api) return;
          const project = await api.CreateProject(projectName, 'ID', 'id-ID');
          const sku = await api.CreateSKU(project.id, skuName, facts);
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
      <button type="button" onClick={() => run(async () => {
        if (!api) throw new Error('生成候选需要在桌面应用里打开');
        if (!skuID) throw new Error('先创建商品');
        const generated = await api.GenerateScripts(skuID, facts);
        setCandidates(generated.candidates ?? []);
        setNotes(generated.notes ?? []);
        setNotice(`已生成 ${generated.candidates?.length ?? 0} 条候选`);
      })}>生成候选</button>
      <ScriptReview
        candidates={candidates}
        notes={notes}
        onEditCandidate={(next) => setCandidates((current) => current.map((item) => item.id === next.id ? next : item))}
        onRetranslate={(candidate) => run(async () => {
          if (!api) throw new Error('生成候选需要在桌面应用里打开');
          const next = await api.Retranslate(skuID, candidate);
          setCandidates((current) => current.map((item) => item.id === next.id ? next : item));
          setNotice('已重新生成中文回译');
        })}
        onAdopt={(candidate) => run(async () => {
          if (!api) throw new Error('生成候选需要在桌面应用里打开');
          if (!candidate.translation_of) throw new Error('中文回译已失效，请先重新生成回译');
          if (!fontID || !bgmID) throw new Error('先选择字体和 BGM');
          const saved = await api.AssembleCandidate(projectID, skuID, fontID, bgmID, '', candidate);
          setDraft(saved);
          if (saved.segments[0]) setSegment(segmentFromDraft(saved.segments[0]));
          await refresh(skuID);
          setNotice(`已按配音时长配镜，修订 ${saved.revision}`);
        })}
        onChangeNote={(note) => setNotes((current) => current.map((item) => item.clip_id === note.clip_id ? note : item))}
        onSaveNote={(note) => run(async () => {
          if (!api) throw new Error('生成候选需要在桌面应用里打开');
          const saved = await api.SaveCorrection(skuID, note);
          setNotes((current) => current.map((item) => item.clip_id === saved.clip_id ? saved : item));
          setNotice('镜头修正已保存');
        })}
      />
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
          setOutputID(output.id);
          setNotice(`成片已写入 ${output.path}`);
        })}>导出</button>
      </div>
      {outputs.length > 0 && <ul>{outputs.map((output) => <li key={output.id}>修订 {output.draft_revision}：{output.path}</li>)}</ul>}
      <fieldset className="card">
        <legend>发布与回流</legend>
        <label>成片编号<input value={outputID} onChange={(event) => setOutputID(event.target.value)} /></label>
        <label>发布链接<input value={publishURL} onChange={(event) => setPublishURL(event.target.value)} /></label>
        <label>视频编号<input value={videoID} onChange={(event) => setVideoID(event.target.value)} /></label>
        <label>店铺<input value={shop} onChange={(event) => setShop(event.target.value)} /></label>
        <label>发布时间<input value={publishedAt} onChange={(event) => setPublishedAt(event.target.value)} /></label>
        <button type="button" onClick={() => run(async () => {
          if (!api) throw new Error('绑定发布需要在桌面应用里打开');
          const saved = await api.BindPublication(outputID, publishURL, videoID, shop, publishedAt);
          setPublications((current) => [...current, saved]);
          setNotice(`已绑定视频 ${saved.video_id}`);
        })}>绑定发布</button>
        {publications.length > 0 && <ul>{publications.map((item) => <li key={item.id}>{item.video_id} · {item.shop} · {item.published_at}</li>)}</ul>}
        <label>回流 CSV<textarea value={csvText} onChange={(event) => setCsvText(event.target.value)} /></label>
        <button type="button" onClick={() => run(async () => {
          if (!api) throw new Error('预览回流需要在桌面应用里打开');
          setFeedback(describePreview(await api.PreviewMetrics(csvText)));
        })}>预览回流</button>
        <button type="button" onClick={() => run(async () => {
          if (!api) throw new Error('导入回流需要在桌面应用里打开');
          const imported = await api.ImportMetrics(csvText);
          setFeedback(describePreview(imported));
          setNotice(imported.repeated ? '这批报表已经导入过' : `已导入 ${imported.accepted} 行`);
        })}>导入回流</button>
        {feedback && <p>{feedback}</p>}
      </fieldset>
    </section>
  );
}
