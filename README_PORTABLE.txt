KoalaPull - Desktop Download Manager for yt-dlp

This application is fully portable by default.
Settings, history, and video download engine binaries (yt-dlp, ffmpeg) will be saved in a subfolder next to the executable if the parent folder is writable.

If the folder is read-only (such as standard system application directories or protected paths), KoalaPull will automatically fallback to storing settings and binaries in the standard system user config directory.

To use KoalaPull in portable mode:
1. Keep KoalaPull inside this folder or any writable directory (such as a USB drive or your user home directory).
2. Run KoalaPull. It will automatically download/configure its dependencies inside a 'bin' subdirectory here.
