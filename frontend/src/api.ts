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
export type Publication = {
  id: string;
  output_id: string;
  url: string;
  video_id: string;
  shop: string;
  published_at: string;
};
export type MetricPreview = {
  sources: string[] | null;
  shops: string[] | null;
  timezones: string[] | null;
  currencies: string[] | null;
  accepted: number;
  repeated: boolean;
  errors: { line: number; message: string }[] | null;
  unbound: string[] | null;
};

export type DesktopAccount = { email: string; kind: string; created_at: number };
export type ProduceResult = {
  candidates: number;
  outputs: { id: string; path: string }[] | null;
  fetched?: { asset_id: string; path: string; duration_ms: number }[] | null;
  known_micros: number;
  unknown_calls: number;
  note: string;
};
export type UsageView = {
  known_micros: number;
  unknown_calls: number;
  billable_calls: number;
  cache_hits: number;
};
export type Shop = {
  id: string;
  name: string;
  created_at: string;
};
export type PlatformShop = {
  id: string;
  name: string;
  region: string;
  seller_type: string;
  code: string;
  seller_name: string;
  updated_at: string;
};
export type PlatformStatus = { configured: boolean; callback: string };
export type PlatformConnect = { url: string; state: string };
export type PlatformSync = { written: number; note: string; latest_available: string };
export type PlatformVideo = {
  shop_name: string;
  video_id: string;
  title: string;
  start: string;
  end: string;
  views: number | null;
  orders: number | null;
  gmv_minor: number | null;
  currency: string;
  latest_available: string;
};

export function isAuthorizeURL(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'https:'
      && (parsed.hostname === 'services.tiktokshop.com' || parsed.hostname === 'services.us.tiktokshop.com')
      && parsed.pathname === '/open/authorize';
  } catch {
    return false;
  }
}

type Backend = {
  Login(email: string, password: string): Promise<DesktopAccount>;
  Register(email: string, password: string): Promise<DesktopAccount>;
  RegistrationOpen(): Promise<boolean>;
  Logout(): Promise<void>;
  CurrentAccount(): Promise<DesktopAccount>;
  CreateProject(name: string, market: string, locale: string): Promise<Project>;
  CreateSKU(projectID: string, name: string, facts: string): Promise<SKU>;
  ImportFile(skuID: string, kind: string, source: string, rights: string, path: string): Promise<{ asset_id: string; clip_ids?: string[]; duration_ms: number }>;
  ListClips(skuID: string): Promise<Clip[]>;
  ListAssets(skuID: string, kind: string): Promise<Asset[]>;
  SaveDraft(draft: Draft): Promise<Draft>;
  PreviewDraft(draftID: string, outputPath: string): Promise<{ output_id: string }>;
  ExportDraft(draftID: string, outputPath: string): Promise<{ id: string; path: string }>;
  ListOutputs(draftID: string): Promise<{ id: string; path: string; draft_revision: number }[]>;
  GenerateScripts(skuID: string, facts: string): Promise<Generation>;
  PickMaterial?(): Promise<string>;
  MixFromLinks?(projectID: string, skuID: string, fontID: string, bgmID: string, prompt: string, links: string[]): Promise<ProduceResult>;
  ProduceVideo(projectID: string, skuID: string, fontID: string, bgmID: string, clipAssetIDs: string[], prompt: string): Promise<ProduceResult>;
  Usage(): Promise<UsageView>;
  ListNotes(skuID: string): Promise<ShotNote[]>;
  SaveCorrection(skuID: string, note: ShotNote): Promise<ShotNote>;
  Retranslate(skuID: string, candidate: Candidate): Promise<Candidate>;
  AssembleCandidate(projectID: string, skuID: string, fontID: string, bgmID: string, voice: string, candidate: Candidate): Promise<Draft>;
  BindPublication(outputID: string, url: string, videoID: string, shop: string, publishedAt: string): Promise<Publication>;
  PreviewMetrics(csv: string): Promise<MetricPreview>;
  ImportMetrics(csv: string): Promise<MetricPreview>;
  CreateShop?(name: string): Promise<Shop>;
  ListShops?(): Promise<Shop[]>;
  PlatformStatus?(): Promise<PlatformStatus>;
  ConnectPlatformShop?(): Promise<PlatformConnect>;
  CompletePlatformShop?(code: string, state: string): Promise<PlatformShop[]>;
  ListPlatformShops?(): Promise<PlatformShop[]>;
  SyncPlatformShop?(shopID: string, start: string, end: string): Promise<PlatformSync>;
  ListPlatformVideos?(): Promise<PlatformVideo[]>;
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
