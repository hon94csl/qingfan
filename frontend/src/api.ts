import type { DraftSegment } from './components/segment';

export type Project = { id: string; name: string; market: string; locale: string };
export type SKU = { id: string; project_id: string; name: string; facts: string };
export type Clip = {
  id: string;
  asset_id: string;
  label: string;
  selectable: boolean;
  reason?: string;
  source_in_ms: number;
  source_out_ms: number;
  duration_ms: number;
};
export type Asset = {
  id: string;
  kind: string;
  path: string;
  duration_ms: number;
  selectable: boolean;
  reason?: string;
};
export type Draft = {
  id: string;
  project_id: string;
  sku_id: string;
  revision: number;
  market: string;
  locale: string;
  font_asset_id: string;
  bgm_asset_id: string;
  bgm_gain_db: number;
  hook_type?: string;
  prompt_version?: string;
  segments: DraftSegment[];
};
export type ShotNote = {
  clip_id: string;
  subject: string;
  action: string;
  purpose: string;
  selling_points: string[] | null;
  source_in_ms: number;
  source_out_ms: number;
  manual: boolean;
};
export type ScriptSegment = {
  id: string;
  role: string;
  purpose: string;
  fact_ids: string[];
  voice_text: string;
  clip_ids: string[];
  estimated_ms: number;
};
export type Candidate = {
  id: string;
  hook_type: string;
  back_translation: string;
  translation_of: string;
  segments: ScriptSegment[];
};
export type Generation = { notes: ShotNote[] | null; candidates: Candidate[] | null };

type Backend = {
  CreateProject(name: string, market: string, locale: string): Promise<Project>;
  CreateSKU(projectID: string, name: string, facts: string): Promise<SKU>;
  ImportFile(skuID: string, kind: string, source: string, rights: string, path: string): Promise<{ asset_id: string; clip_ids?: string[] }>;
  ListClips(skuID: string): Promise<Clip[]>;
  ListAssets(skuID: string, kind: string): Promise<Asset[]>;
  SaveDraft(draft: Draft): Promise<Draft>;
  PreviewDraft(draftID: string, outputPath: string): Promise<{ output_id: string }>;
  ExportDraft(draftID: string, outputPath: string): Promise<{ id: string; path: string }>;
  ListOutputs(draftID: string): Promise<{ id: string; path: string; draft_revision: number }[]>;
  GenerateScripts(skuID: string, facts: string): Promise<Generation>;
  ListNotes(skuID: string): Promise<ShotNote[]>;
  SaveCorrection(skuID: string, note: ShotNote): Promise<ShotNote>;
  Retranslate(skuID: string, candidate: Candidate): Promise<Candidate>;
  AssembleCandidate(projectID: string, skuID: string, fontID: string, bgmID: string, voice: string, candidate: Candidate): Promise<Draft>;
};

export function backend(): Backend | null {
  const go = (window as unknown as { go?: { main?: { App?: Backend } } }).go;
  return go?.main?.App ?? null;
}

export function messageOf(error: unknown): string {
  if (error instanceof Error && error.message) return error.message;
  if (typeof error === 'string' && error) return error;
  return '操作没有完成';
}
