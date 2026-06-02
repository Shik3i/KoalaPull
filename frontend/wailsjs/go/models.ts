export namespace main {
	
	export class DependencyStatus {
	    ytDlpInstalled: boolean;
	    ffmpegInstalled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DependencyStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ytDlpInstalled = source["ytDlpInstalled"];
	        this.ffmpegInstalled = source["ffmpegInstalled"];
	    }
	}
	export class FormatInfo {
	    formatId: string;
	    ext: string;
	    width: number;
	    height: number;
	    vcodec: string;
	    acodec: string;
	    filesize: number;
	    formatNote: string;
	
	    static createFrom(source: any = {}) {
	        return new FormatInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.formatId = source["formatId"];
	        this.ext = source["ext"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.vcodec = source["vcodec"];
	        this.acodec = source["acodec"];
	        this.filesize = source["filesize"];
	        this.formatNote = source["formatNote"];
	    }
	}
	export class Settings {
	    defaultOutputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultOutputDir = source["defaultOutputDir"];
	    }
	}
	export class VideoMetadata {
	    id: string;
	    title: string;
	    thumbnail: string;
	    uploader: string;
	    duration: number;
	    formats: FormatInfo[];
	    isPlaylist: boolean;
	    entryCount: number;
	
	    static createFrom(source: any = {}) {
	        return new VideoMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.thumbnail = source["thumbnail"];
	        this.uploader = source["uploader"];
	        this.duration = source["duration"];
	        this.formats = this.convertValues(source["formats"], FormatInfo);
	        this.isPlaylist = source["isPlaylist"];
	        this.entryCount = source["entryCount"];
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

