export namespace backup {
	
	export class Entry {
	    name: string;
	    path: string;
	    size: number;
	    mod_time: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.mod_time = source["mod_time"];
	        this.source = source["source"];
	    }
	}

}

export namespace global {
	
	export class LanguageInfo {
	    language_name: string;
	    language_code: string;
	    textmap_path: string;
	    translation_progress: string;
	    translator: string;
	    last_updated: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new LanguageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language_name = source["language_name"];
	        this.language_code = source["language_code"];
	        this.textmap_path = source["textmap_path"];
	        this.translation_progress = source["translation_progress"];
	        this.translator = source["translator"];
	        this.last_updated = source["last_updated"];
	        this.version = source["version"];
	    }
	}
	export class LanguagePack {
	    language_name: string;
	    language_code: string;
	    textmap_path: string;
	    translation_progress: string;
	    translator: string;
	    last_updated: string;
	    version: string;
	    textmap: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new LanguagePack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language_name = source["language_name"];
	        this.language_code = source["language_code"];
	        this.textmap_path = source["textmap_path"];
	        this.translation_progress = source["translation_progress"];
	        this.translator = source["translator"];
	        this.last_updated = source["last_updated"];
	        this.version = source["version"];
	        this.textmap = source["textmap"];
	    }
	}

}

export namespace plugin {
	
	export class ProbeResult {
	    status: string;
	    status_code: number;
	    latency_ms: number;
	    message?: string;
	    checked_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.status_code = source["status_code"];
	        this.latency_ms = source["latency_ms"];
	        this.message = source["message"];
	        this.checked_at = source["checked_at"];
	    }
	}
	export class Model {
	    id: string;
	    display_name: string;
	    provider: string;
	    base_url: string;
	    api_key: string;
	    enabled: boolean;
	    description: string;
	    tags: string[];
	    plugin_id: string;
	    source_file: string;
	    native_enabled: boolean;
	    capabilities?: Record<string, boolean>;
	    probe?: ProbeResult;
	
	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	        this.provider = source["provider"];
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.enabled = source["enabled"];
	        this.description = source["description"];
	        this.tags = source["tags"];
	        this.plugin_id = source["plugin_id"];
	        this.source_file = source["source_file"];
	        this.native_enabled = source["native_enabled"];
	        this.capabilities = source["capabilities"];
	        this.probe = this.convertValues(source["probe"], ProbeResult);
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
	
	export class RemoteModel {
	    id: string;
	    display_name?: string;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	        this.note = source["note"];
	    }
	}

}

export namespace service {
	
	export class ApplyModelsRequest {
	    plugin_id: string;
	    models: plugin.Model[];
	
	    static createFrom(source: any = {}) {
	        return new ApplyModelsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plugin_id = source["plugin_id"];
	        this.models = this.convertValues(source["models"], plugin.Model);
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
	export class ApplyModelsResult {
	    written: number;
	    file: string;
	    models: plugin.Model[];
	
	    static createFrom(source: any = {}) {
	        return new ApplyModelsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.written = source["written"];
	        this.file = source["file"];
	        this.models = this.convertValues(source["models"], plugin.Model);
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
	export class ImportRequest {
	    upstream_ids: string[];
	    model_ids?: Record<string, Array<string>>;
	
	    static createFrom(source: any = {}) {
	        return new ImportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upstream_ids = source["upstream_ids"];
	        this.model_ids = source["model_ids"];
	    }
	}
	export class ImportResult {
	    added: number;
	    updated: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.updated = source["updated"];
	        this.total = source["total"];
	    }
	}
	export class PluginView {
	    id: string;
	    name: string;
	    description: string;
	    vendor: string;
	    color: string;
	    version: string;
	    builtin: boolean;
	    enabled: boolean;
	    loaded: boolean;
	    load_error?: string;
	    def_path: string;
	    source_file: string;
	    source_ready: boolean;
	    candidates: string[];
	    model_count: number;
	    backup_count: number;
	    probe_type: string;
	    can_toggle_enabled: boolean;
	    mapped_fields: string[];
	    capabilities: string[];
	
	    static createFrom(source: any = {}) {
	        return new PluginView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.vendor = source["vendor"];
	        this.color = source["color"];
	        this.version = source["version"];
	        this.builtin = source["builtin"];
	        this.enabled = source["enabled"];
	        this.loaded = source["loaded"];
	        this.load_error = source["load_error"];
	        this.def_path = source["def_path"];
	        this.source_file = source["source_file"];
	        this.source_ready = source["source_ready"];
	        this.candidates = source["candidates"];
	        this.model_count = source["model_count"];
	        this.backup_count = source["backup_count"];
	        this.probe_type = source["probe_type"];
	        this.can_toggle_enabled = source["can_toggle_enabled"];
	        this.mapped_fields = source["mapped_fields"];
	        this.capabilities = source["capabilities"];
	    }
	}
	export class Settings {
	    language: string;
	    log_level: string;
	    plugin_dir: string;
	    data_dir: string;
	    backup_dir: string;
	    backup_keep: number;
	    work_dir: string;
	    source_overrides: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.log_level = source["log_level"];
	        this.plugin_dir = source["plugin_dir"];
	        this.data_dir = source["data_dir"];
	        this.backup_dir = source["backup_dir"];
	        this.backup_keep = source["backup_keep"];
	        this.work_dir = source["work_dir"];
	        this.source_overrides = source["source_overrides"];
	    }
	}
	export class SystemInfo {
	    os: string;
	    arch: string;
	    num_cpu: number;
	    hostname: string;
	    go_ver: string;
	    time: string;
	    process_name: string;
	    work_dir: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.num_cpu = source["num_cpu"];
	        this.hostname = source["hostname"];
	        this.go_ver = source["go_ver"];
	        this.time = source["time"];
	        this.process_name = source["process_name"];
	        this.work_dir = source["work_dir"];
	    }
	}

}

export namespace upstream {
	
	export class Model {
	    id: string;
	    display_name?: string;
	    capabilities?: Record<string, boolean>;
	
	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.display_name = source["display_name"];
	        this.capabilities = source["capabilities"];
	    }
	}
	export class Upstream {
	    id: string;
	    name: string;
	    vendor?: string;
	    url: string;
	    api_key: string;
	    notes?: string;
	    models: Model[];
	    origins?: string[];
	    updated_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new Upstream(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.vendor = source["vendor"];
	        this.url = source["url"];
	        this.api_key = source["api_key"];
	        this.notes = source["notes"];
	        this.models = this.convertValues(source["models"], Model);
	        this.origins = source["origins"];
	        this.updated_at = source["updated_at"];
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

