export interface DependencyStatus {
  ytDlpInstalled: boolean;
  ffmpegInstalled: boolean;
}

export function CheckDependencies(): Promise<DependencyStatus>;
export function DownloadDependencies(): Promise<void>;
