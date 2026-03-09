export namespace connection {
	
	export class Profile {
	    name: string;
	    host: string;
	    port: number;
	    key_hex: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.key_hex = source["key_hex"];
	    }
	}

}

export namespace mgmt {
	
	export class BanRecord {
	    id: number;
	    ip: string;
	    dedupe_id: string;
	    banned_at: number;
	    expires_at: number;
	    source: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new BanRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ip = source["ip"];
	        this.dedupe_id = source["dedupe_id"];
	        this.banned_at = source["banned_at"];
	        this.expires_at = source["expires_at"];
	        this.source = source["source"];
	        this.reason = source["reason"];
	    }
	}
	export class BansResult {
	    total: number;
	    page: number;
	    bans: BanRecord[];
	
	    static createFrom(source: any = {}) {
	        return new BansResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.bans = this.convertValues(source["bans"], BanRecord);
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
	export class PeerRecord {
	    node_id: string;
	    addr: string;
	    connected: boolean;
	    last_seen: number;
	    direction: string;
	
	    static createFrom(source: any = {}) {
	        return new PeerRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node_id = source["node_id"];
	        this.addr = source["addr"];
	        this.connected = source["connected"];
	        this.last_seen = source["last_seen"];
	        this.direction = source["direction"];
	    }
	}
	export class PeersResult {
	    peers: PeerRecord[];
	
	    static createFrom(source: any = {}) {
	        return new PeersResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peers = this.convertValues(source["peers"], PeerRecord);
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
	export class StatsResult {
	    bans_this_session: number;
	    unbans_this_session: number;
	    scan_detects_this_session: number;
	    connections_accepted: number;
	
	    static createFrom(source: any = {}) {
	        return new StatsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bans_this_session = source["bans_this_session"];
	        this.unbans_this_session = source["unbans_this_session"];
	        this.scan_detects_this_session = source["scan_detects_this_session"];
	        this.connections_accepted = source["connections_accepted"];
	    }
	}
	export class StatusResult {
	    node_id: string;
	    version: string;
	    uptime_sec: number;
	    peer_count: number;
	    ban_count: number;
	
	    static createFrom(source: any = {}) {
	        return new StatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node_id = source["node_id"];
	        this.version = source["version"];
	        this.uptime_sec = source["uptime_sec"];
	        this.peer_count = source["peer_count"];
	        this.ban_count = source["ban_count"];
	    }
	}

}

