export namespace main {
	
	export class AnalyzeRequest {
	    kind: string;
	    start: number;
	    end: number;
	    fresh: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.fresh = source["fresh"];
	    }
	}
	export class AutoMask {
	    kind: string;
	    start: number;
	    duration: number;
	    fps: number;
	    found: number;
	    tracks: number;
	    frames: number[][][];
	
	    static createFrom(source: any = {}) {
	        return new AutoMask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.start = source["start"];
	        this.duration = source["duration"];
	        this.fps = source["fps"];
	        this.found = source["found"];
	        this.tracks = source["tracks"];
	        this.frames = source["frames"];
	    }
	}
	export class ExportRequest {
	    start: number;
	    end: number;
	    rects: media.MaskRect[];
	    fileName: string;
	    auto: string[];
	    mute: boolean;
	    exclude: Record<string, Array<number>>;
	    overwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.rects = this.convertValues(source["rects"], media.MaskRect);
	        this.fileName = source["fileName"];
	        this.auto = source["auto"];
	        this.mute = source["mute"];
	        this.exclude = source["exclude"];
	        this.overwrite = source["overwrite"];
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
	export class RecentVideo {
	    id: string;
	    title: string;
	    duration: number;
	    savedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new RecentVideo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.duration = source["duration"];
	        this.savedAt = source["savedAt"];
	    }
	}
	export class TrackRequest {
	    time: number;
	    start: number;
	    end: number;
	    rect: media.Rect;
	
	    static createFrom(source: any = {}) {
	        return new TrackRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.rect = this.convertValues(source["rect"], media.Rect);
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
	export class TrackResult {
	    id: number;
	    start: number;
	    fps: number;
	    frames: number[][];
	    lostStart: boolean;
	    lostEnd: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TrackResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.start = source["start"];
	        this.fps = source["fps"];
	        this.frames = source["frames"];
	        this.lostStart = source["lostStart"];
	        this.lostEnd = source["lostEnd"];
	    }
	}
	export class VideoInfo {
	    url: string;
	    duration: number;
	    width: number;
	    height: number;
	    name: string;
	    reused: boolean;
	    lowered: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VideoInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.duration = source["duration"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.name = source["name"];
	        this.reused = source["reused"];
	        this.lowered = source["lowered"];
	    }
	}

}

export namespace media {
	
	export class MaskRect {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	    start: number;
	    end: number;
	    track: number;
	
	    static createFrom(source: any = {}) {
	        return new MaskRect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.track = source["track"];
	    }
	}
	export class Rect {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new Rect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}

}

