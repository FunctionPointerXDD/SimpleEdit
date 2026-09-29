export namespace main {
	
	export class AnalyzeRequest {
	    kind: string;
	    start: number;
	    end: number;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.start = source["start"];
	        this.end = source["end"];
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
	    rects: media.Rect[];
	    fileName: string;
	    auto: string[];
	    mute: boolean;
	    exclude: Record<string, Array<number>>;
	
	    static createFrom(source: any = {}) {
	        return new ExportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.rects = this.convertValues(source["rects"], media.Rect);
	        this.fileName = source["fileName"];
	        this.auto = source["auto"];
	        this.mute = source["mute"];
	        this.exclude = source["exclude"];
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
	export class VideoInfo {
	    url: string;
	    duration: number;
	    width: number;
	    height: number;
	    name: string;
	
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
	    }
	}

}

export namespace media {
	
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

