export interface DependencyStatus {
  ytDlpInstalled: boolean;
  ffmpegInstalled: boolean;
}

export interface FormatInfo {
  formatId: string;
  ext: string;
  width: number;
  height: number;
  vcodec: string;
  acodec: string;
  filesize: number;
  formatNote: string;
}

export interface VideoMetadata {
  id: string;
  title: string;
  thumbnail: string;
  uploader: string;
  duration: number;
  formats: FormatInfo[];
}

export function CheckDependencies(): Promise<DependencyStatus>;
export function DownloadDependencies(): Promise<void>;
export function FetchMetadata(url: string): Promise<VideoMetadata>;
export function StartDownload(url: string, formatID: string, outputDir: string): Promise<string>;
