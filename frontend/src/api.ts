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
  segments: DraftSegment[];
};

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
