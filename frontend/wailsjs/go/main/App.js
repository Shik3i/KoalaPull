export function CheckDependencies() {
    return window['go']['main']['App']['CheckDependencies']();
}

export function DownloadDependencies() {
    return window['go']['main']['App']['DownloadDependencies']();
}

export function FetchMetadata(url) {
    return window['go']['main']['App']['FetchMetadata'](url);
}

export function StartDownload(url, formatID, outputDir) {
    return window['go']['main']['App']['StartDownload'](url, formatID, outputDir);
}
