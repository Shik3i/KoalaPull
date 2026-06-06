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
	export class HistoryEntryView {
	    downloadId: string;
	    url: string;
	    title: string;
	    formatId: string;
	    fileSize: string;
	    avgSpeed: string;
	    status: string;
	    errorMsg?: string;
	    startTime: string;
	    endTime: string;
	    outputPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadId = source["downloadId"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.formatId = source["formatId"];
	        this.fileSize = source["fileSize"];
	        this.avgSpeed = source["avgSpeed"];
	        this.status = source["status"];
	        this.errorMsg = source["errorMsg"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.outputPath = source["outputPath"];
	    }
	}
	export class PlaylistEntry {
	    id: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaylistEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	    }
	}
	export class Settings {
	    defaultOutputDir: string;
	    theme: string;
	    maxConcurrency: number;
	    autoPasteURL: boolean;
	    language: string;
	    downloadPreset: string;
	    customFormatId: string;
	    customContainer: string;
	    customSubtitle: string;
	    cookieSource: string;
	    cookieBrowser: string;
	    cookieFilePath: string;
	    cookieCachePath: string;
	    cookieCacheBrowser: string;
	    cookieCacheUpdated: string;
	    rateLimitEnabled: boolean;
	    rateLimitValue: string;
	    safeModeEnabled: boolean;
	    customArgs: string;
	    ffmpegPath: string;
	    sponsorBlockEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultOutputDir = source["defaultOutputDir"];
	        this.theme = source["theme"];
	        this.maxConcurrency = source["maxConcurrency"];
	        this.autoPasteURL = source["autoPasteURL"];
	        this.language = source["language"];
	        this.downloadPreset = source["downloadPreset"];
	        this.customFormatId = source["customFormatId"];
	        this.customContainer = source["customContainer"];
	        this.customSubtitle = source["customSubtitle"];
	        this.cookieSource = source["cookieSource"];
	        this.cookieBrowser = source["cookieBrowser"];
	        this.cookieFilePath = source["cookieFilePath"];
	        this.cookieCachePath = source["cookieCachePath"];
	        this.cookieCacheBrowser = source["cookieCacheBrowser"];
	        this.cookieCacheUpdated = source["cookieCacheUpdated"];
	        this.rateLimitEnabled = source["rateLimitEnabled"];
	        this.rateLimitValue = source["rateLimitValue"];
	        this.safeModeEnabled = source["safeModeEnabled"];
	        this.customArgs = source["customArgs"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.sponsorBlockEnabled = source["sponsorBlockEnabled"];
	    }
	}
	export class UpdateInfo {
	    ytdlpUpdateAvailable: boolean;
	    latestYtdlpVersion: string;
	    koalaPullUpdateAvailable: boolean;
	    latestKoalaPullVersion: string;
	    ffmpegUpdateAvailable: boolean;
	    latestFfmpegVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ytdlpUpdateAvailable = source["ytdlpUpdateAvailable"];
	        this.latestYtdlpVersion = source["latestYtdlpVersion"];
	        this.koalaPullUpdateAvailable = source["koalaPullUpdateAvailable"];
	        this.latestKoalaPullVersion = source["latestKoalaPullVersion"];
	        this.ffmpegUpdateAvailable = source["ffmpegUpdateAvailable"];
	        this.latestFfmpegVersion = source["latestFfmpegVersion"];
	    }
	}
	export class VersionInfo {
	    ytdlp: string;
	    ffmpeg: string;
	    app: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ytdlp = source["ytdlp"];
	        this.ffmpeg = source["ffmpeg"];
	        this.app = source["app"];
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
	    entries?: PlaylistEntry[];
	
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
	        this.entries = this.convertValues(source["entries"], PlaylistEntry);
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
