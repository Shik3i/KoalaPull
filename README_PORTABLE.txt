KoalaPull - Desktop Download Manager for yt-dlp

This application is fully portable by default.
Settings, history, and video download engine binaries (yt-dlp, ffmpeg) will be saved in a subfolder next to the executable if the parent folder is writable.

If the folder is read-only (such as standard system application directories or protected paths), KoalaPull will automatically fallback to storing settings and binaries in the standard system user config directory.

To use KoalaPull in portable mode:
1. Keep KoalaPull inside this folder or any writable directory (such as a USB drive or your user home directory).
2. Run KoalaPull. It will automatically download/configure its dependencies inside a 'bin' subdirectory here.
3. Dependency updates are verified before install and swapped in atomically, so failed updates should not leave half-written engine binaries behind.

--------------------------------------------------------------------------------
Note for macOS Users (Unidentified Developer Gatekeeper Warning)
--------------------------------------------------------------------------------
KoalaPull is unsigned. macOS Gatekeeper may block it on first launch.

Method 1: Right-Click Shortcut (Recommended)
1. Right-click, or Control-click, the KoalaPull app.
2. Select 'Open' from the context menu.
3. Click 'Open' on the confirmation dialog.

Method 2: System Settings
1. Double-click the app. If blocked, click 'OK'.
2. Open System Settings > Privacy & Security.
3. Scroll to the 'Security' section.
4. Locate the KoalaPull blocked notice.
5. Click 'Open Anyway' and authenticate.

