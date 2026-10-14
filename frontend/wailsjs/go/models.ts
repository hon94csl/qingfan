export namespace ai {

	export class ScriptSegment {
	    id: string;
	    role: string;
	    purpose: string;
	    fact_ids: string[];
	    voice_text: string;
	    clip_ids: string[];
	    estimated_ms: number;

	    static createFrom(source: any = {}) {
	        return new ScriptSegment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.purpose = source["purpose"];
	        this.fact_ids = source["fact_ids"];
	        this.voice_text = source["voice_text"];
	        this.clip_ids = source["clip_ids"];
	        this.estimated_ms = source["estimated_ms"];
	    }
	}
	export class Candidate {
	    id: string;
	    hook_type: string;
	    back_translation: string;
	    translation_of: string;
	    segments: ScriptSegment[];

	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.hook_type = source["hook_type"];
	        this.back_translation = source["back_translation"];
	        this.translation_of = source["translation_of"];
	        this.segments = this.convertValues(source["segments"], ScriptSegment);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CostReport {
	    known_micros: number;
	    unknown_calls: number;
	    billable_calls: number;
	    cache_hits: number;

	    static createFrom(source: any = {}) {
	        return new CostReport(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.known_micros = source["known_micros"];
	        this.unknown_calls = source["unknown_calls"];
	        this.billable_calls = source["billable_calls"];
	        this.cache_hits = source["cache_hits"];
	    }
	}
	export class ShotNote {
	    clip_id: string;
	    subject: string;
	    action: string;
	    purpose: string;
	    selling_points: string[];
	    source_in_ms: number;
	    source_out_ms: number;
	    manual: boolean;
	    fact_ids?: string[];

	    static createFrom(source: any = {}) {
	        return new ShotNote(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clip_id = source["clip_id"];
	        this.subject = source["subject"];
	        this.action = source["action"];
	        this.purpose = source["purpose"];
	        this.selling_points = source["selling_points"];
	        this.source_in_ms = source["source_in_ms"];
	        this.source_out_ms = source["source_out_ms"];
	        this.manual = source["manual"];
	        this.fact_ids = source["fact_ids"];
	    }
	}
	export class Generation {
	    notes: ShotNote[];
	    candidates: Candidate[];

	    static createFrom(source: any = {}) {
	        return new Generation(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notes = this.convertValues(source["notes"], ShotNote);
	        this.candidates = this.convertValues(source["candidates"], Candidate);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


}

export namespace auth {

	export class Account {
	    email: string;
	    kind: string;
	    created_at: number;

	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.kind = source["kind"];
	        this.created_at = source["created_at"];
	    }
	}

}

export namespace main {

	export class FetchedClip {
	    asset_id: string;
	    path: string;
	    duration_ms: number;

	    static createFrom(source: any = {}) {
	        return new FetchedClip(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.asset_id = source["asset_id"];
	        this.path = source["path"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class ProducedVideo {
	    id: string;
	    path: string;

	    static createFrom(source: any = {}) {
	        return new ProducedVideo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	    }
	}
	export class ProduceResult {
	    candidates: number;
	    outputs: ProducedVideo[];
	    fetched?: FetchedClip[];
	    known_micros: number;
	    unknown_calls: number;
	    note: string;

	    static createFrom(source: any = {}) {
	        return new ProduceResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidates = source["candidates"];
	        this.outputs = this.convertValues(source["outputs"], ProducedVideo);
	        this.fetched = this.convertValues(source["fetched"], FetchedClip);
	        this.known_micros = source["known_micros"];
	        this.unknown_calls = source["unknown_calls"];
	        this.note = source["note"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace media {

	export class BuildInfo {
	    name: string;
	    version: string;
	    path?: string;

	    static createFrom(source: any = {}) {
	        return new BuildInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.path = source["path"];
	    }
	}
	export class EncoderInfo {
	    name: string;
	    config: string;

	    static createFrom(source: any = {}) {
	        return new EncoderInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.config = source["config"];
	    }
	}
	export class QualityReport {
	    width: number;
	    height: number;
	    fps: number;
	    duration_ms: number;
	    audio_streams: number;
	    subtitles: string[];
	    black_frames: string[];
	    silence: string[];

	    static createFrom(source: any = {}) {
	        return new QualityReport(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.fps = source["fps"];
	        this.duration_ms = source["duration_ms"];
	        this.audio_streams = source["audio_streams"];
	        this.subtitles = source["subtitles"];
	        this.black_frames = source["black_frames"];
	        this.silence = source["silence"];
	    }
	}
	export class Resource {
	    id: string;
	    kind: string;
	    path: string;
	    sha256: string;

	    static createFrom(source: any = {}) {
	        return new Resource(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.path = source["path"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class Manifest {
	    schema_version: number;
	    output_id: string;
	    revision: number;
	    sku_id: string;
	    timeline: timeline.Timeline;
	    resources: Resource[];
	    renderer: BuildInfo;
	    ffmpeg: BuildInfo;
	    encoder: EncoderInfo;
	    duration_ms: number;
	    quality: QualityReport;
	    notes?: string[];

	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.output_id = source["output_id"];
	        this.revision = source["revision"];
	        this.sku_id = source["sku_id"];
	        this.timeline = this.convertValues(source["timeline"], timeline.Timeline);
	        this.resources = this.convertValues(source["resources"], Resource);
	        this.renderer = this.convertValues(source["renderer"], BuildInfo);
	        this.ffmpeg = this.convertValues(source["ffmpeg"], BuildInfo);
	        this.encoder = this.convertValues(source["encoder"], EncoderInfo);
	        this.duration_ms = source["duration_ms"];
	        this.quality = this.convertValues(source["quality"], QualityReport);
	        this.notes = source["notes"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


	export class ToolStatus {
	    name: string;
	    path: string;
	    version: string;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new ToolStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.error = source["error"];
	    }
	}

}

export namespace reporting {

	export class RowError {
	    line: number;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new RowError(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.message = source["message"];
	    }
	}
	export class Preview {
	    sources: string[];
	    shops: string[];
	    timezones: string[];
	    currencies: string[];
	    errors: RowError[];
	    unbound: string[];
	    accepted: number;
	    repeated: boolean;

	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sources = source["sources"];
	        this.shops = source["shops"];
	        this.timezones = source["timezones"];
	        this.currencies = source["currencies"];
	        this.errors = this.convertValues(source["errors"], RowError);
	        this.unbound = source["unbound"];
	        this.accepted = source["accepted"];
	        this.repeated = source["repeated"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Publication {
	    id: string;
	    output_id: string;
	    url: string;
	    video_id: string;
	    shop: string;
	    published_at: string;

	    static createFrom(source: any = {}) {
	        return new Publication(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.output_id = source["output_id"];
	        this.url = source["url"];
	        this.video_id = source["video_id"];
	        this.shop = source["shop"];
	        this.published_at = source["published_at"];
	    }
	}
	export class Row {
	    source: string;
	    shop: string;
	    account: string;
	    video_id: string;
	    date: string;
	    views?: number;
	    orders?: number;
	    gmv_minor?: number;
	    currency: string;
	    timezone: string;
	    line: number;

	    static createFrom(source: any = {}) {
	        return new Row(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.shop = source["shop"];
	        this.account = source["account"];
	        this.video_id = source["video_id"];
	        this.date = source["date"];
	        this.views = source["views"];
	        this.orders = source["orders"];
	        this.gmv_minor = source["gmv_minor"];
	        this.currency = source["currency"];
	        this.timezone = source["timezone"];
	        this.line = source["line"];
	    }
	}

}

export namespace shopsync {

	export class ConnectResult {
	    url: string;
	    state: string;

	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.state = source["state"];
	    }
	}
	export class Status {
	    configured: boolean;
	    callback: string;

	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.callback = source["callback"];
	    }
	}
	export class SyncResult {
	    written: number;
	    note: string;
	    latest_available: string;

	    static createFrom(source: any = {}) {
	        return new SyncResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.written = source["written"];
	        this.note = source["note"];
	        this.latest_available = source["latest_available"];
	    }
	}

}

export namespace store {

	export class AssetView {
	    id: string;
	    kind: string;
	    sku_id: string;
	    source: string;
	    rights: string;
	    path: string;
	    duration_ms: number;
	    selectable: boolean;
	    reason?: string;
	    sha256: string;

	    static createFrom(source: any = {}) {
	        return new AssetView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.sku_id = source["sku_id"];
	        this.source = source["source"];
	        this.rights = source["rights"];
	        this.path = source["path"];
	        this.duration_ms = source["duration_ms"];
	        this.selectable = source["selectable"];
	        this.reason = source["reason"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class Clip {
	    id: string;
	    asset_id: string;
	    sku_id: string;
	    label: string;
	    source: string;
	    rights: string;
	    selectable: boolean;
	    reason?: string;
	    source_in_ms: number;
	    source_out_ms: number;
	    duration_ms: number;
	    valid: boolean;
	    path?: string;

	    static createFrom(source: any = {}) {
	        return new Clip(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.asset_id = source["asset_id"];
	        this.sku_id = source["sku_id"];
	        this.label = source["label"];
	        this.source = source["source"];
	        this.rights = source["rights"];
	        this.selectable = source["selectable"];
	        this.reason = source["reason"];
	        this.source_in_ms = source["source_in_ms"];
	        this.source_out_ms = source["source_out_ms"];
	        this.duration_ms = source["duration_ms"];
	        this.valid = source["valid"];
	        this.path = source["path"];
	    }
	}
	export class DraftShot {
	    clip_id: string;
	    source_in_ms: number;
	    source_out_ms: number;

	    static createFrom(source: any = {}) {
	        return new DraftShot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clip_id = source["clip_id"];
	        this.source_in_ms = source["source_in_ms"];
	        this.source_out_ms = source["source_out_ms"];
	    }
	}
	export class DraftSegment {
	    id: string;
	    role: string;
	    voice_text: string;
	    voice_asset_id: string;
	    shots: DraftShot[];
	    captions: timeline.Caption[];

	    static createFrom(source: any = {}) {
	        return new DraftSegment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.voice_text = source["voice_text"];
	        this.voice_asset_id = source["voice_asset_id"];
	        this.shots = this.convertValues(source["shots"], DraftShot);
	        this.captions = this.convertValues(source["captions"], timeline.Caption);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Draft {
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

	    static createFrom(source: any = {}) {
	        return new Draft(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.project_id = source["project_id"];
	        this.sku_id = source["sku_id"];
	        this.revision = source["revision"];
	        this.market = source["market"];
	        this.locale = source["locale"];
	        this.font_asset_id = source["font_asset_id"];
	        this.bgm_asset_id = source["bgm_asset_id"];
	        this.bgm_gain_db = source["bgm_gain_db"];
	        this.hook_type = source["hook_type"];
	        this.prompt_version = source["prompt_version"];
	        this.segments = this.convertValues(source["segments"], DraftSegment);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}


	export class ImportResult {
	    asset_id: string;
	    location_id: string;
	    sha256: string;
	    source: string;
	    rights: string;
	    reused_probe: boolean;
	    clip_ids: string[];
	    duration_ms: number;

	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.asset_id = source["asset_id"];
	        this.location_id = source["location_id"];
	        this.sha256 = source["sha256"];
	        this.source = source["source"];
	        this.rights = source["rights"];
	        this.reused_probe = source["reused_probe"];
	        this.clip_ids = source["clip_ids"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class Output {
	    id: string;
	    draft_id: string;
	    draft_revision: number;
	    path: string;
	    manifest_json: string;
	    created_at: string;

	    static createFrom(source: any = {}) {
	        return new Output(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.draft_id = source["draft_id"];
	        this.draft_revision = source["draft_revision"];
	        this.path = source["path"];
	        this.manifest_json = source["manifest_json"];
	        this.created_at = source["created_at"];
	    }
	}
	export class Project {
	    id: string;
	    name: string;
	    market: string;
	    locale: string;

	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.market = source["market"];
	        this.locale = source["locale"];
	    }
	}
	export class SKU {
	    id: string;
	    project_id: string;
	    name: string;
	    facts: string;

	    static createFrom(source: any = {}) {
	        return new SKU(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.project_id = source["project_id"];
	        this.name = source["name"];
	        this.facts = source["facts"];
	    }
	}
	export class Shop {
	    id: string;
	    name: string;
	    created_at: string;

	    static createFrom(source: any = {}) {
	        return new Shop(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.created_at = source["created_at"];
	    }
	}
	export class TikTokShop {
	    id: string;
	    name: string;
	    region: string;
	    seller_type: string;
	    code: string;
	    seller_name: string;
	    updated_at: string;

	    static createFrom(source: any = {}) {
	        return new TikTokShop(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.region = source["region"];
	        this.seller_type = source["seller_type"];
	        this.code = source["code"];
	        this.seller_name = source["seller_name"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class TikTokVideo {
	    shop_id: string;
	    shop_name: string;
	    video_id: string;
	    title: string;
	    account: string;
	    start: string;
	    end: string;
	    views?: number;
	    orders?: number;
	    gmv_minor?: number;
	    currency: string;
	    latest_available: string;

	    static createFrom(source: any = {}) {
	        return new TikTokVideo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shop_id = source["shop_id"];
	        this.shop_name = source["shop_name"];
	        this.video_id = source["video_id"];
	        this.title = source["title"];
	        this.account = source["account"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.views = source["views"];
	        this.orders = source["orders"];
	        this.gmv_minor = source["gmv_minor"];
	        this.currency = source["currency"];
	        this.latest_available = source["latest_available"];
	    }
	}

}

export namespace timeline {

	export class BGM {
	    asset_id: string;
	    gain_db: number;

	    static createFrom(source: any = {}) {
	        return new BGM(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.asset_id = source["asset_id"];
	        this.gain_db = source["gain_db"];
	    }
	}
	export class Canvas {
	    width: number;
	    height: number;
	    fps: number;

	    static createFrom(source: any = {}) {
	        return new Canvas(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.fps = source["fps"];
	    }
	}
	export class Caption {
	    start_ms: number;
	    end_ms: number;
	    text: string;

	    static createFrom(source: any = {}) {
	        return new Caption(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start_ms = source["start_ms"];
	        this.end_ms = source["end_ms"];
	        this.text = source["text"];
	    }
	}
	export class Recipe {
	    reference_id?: string;
	    hook_type?: string;
	    prompt_version?: string;

	    static createFrom(source: any = {}) {
	        return new Recipe(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reference_id = source["reference_id"];
	        this.hook_type = source["hook_type"];
	        this.prompt_version = source["prompt_version"];
	    }
	}
	export class Shot {
	    clip_id: string;
	    source_in_ms: number;
	    source_out_ms: number;

	    static createFrom(source: any = {}) {
	        return new Shot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clip_id = source["clip_id"];
	        this.source_in_ms = source["source_in_ms"];
	        this.source_out_ms = source["source_out_ms"];
	    }
	}
	export class Voice {
	    text?: string;
	    audio_id: string;
	    duration_ms: number;

	    static createFrom(source: any = {}) {
	        return new Voice(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.audio_id = source["audio_id"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class Segment {
	    id: string;
	    role: string;
	    duration_ms: number;
	    voice: Voice;
	    shots: Shot[];
	    original_audio: string;
	    captions: Caption[];
	    caption_style?: string;

	    static createFrom(source: any = {}) {
	        return new Segment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.duration_ms = source["duration_ms"];
	        this.voice = this.convertValues(source["voice"], Voice);
	        this.shots = this.convertValues(source["shots"], Shot);
	        this.original_audio = source["original_audio"];
	        this.captions = this.convertValues(source["captions"], Caption);
	        this.caption_style = source["caption_style"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class Timeline {
	    schema_version: number;
	    output_id: string;
	    revision: number;
	    sku_id: string;
	    market: string;
	    locale: string;
	    canvas: Canvas;
	    font_asset_id: string;
	    recipe: Recipe;
	    segments: Segment[];
	    bgm: BGM;

	    static createFrom(source: any = {}) {
	        return new Timeline(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.output_id = source["output_id"];
	        this.revision = source["revision"];
	        this.sku_id = source["sku_id"];
	        this.market = source["market"];
	        this.locale = source["locale"];
	        this.canvas = this.convertValues(source["canvas"], Canvas);
	        this.font_asset_id = source["font_asset_id"];
	        this.recipe = this.convertValues(source["recipe"], Recipe);
	        this.segments = this.convertValues(source["segments"], Segment);
	        this.bgm = this.convertValues(source["bgm"], BGM);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}
