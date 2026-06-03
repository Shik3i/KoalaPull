# Native Build Assets

This directory contains platform-specific configuration files, app manifests, and artwork assets used by the Wails CLI to compile and package native binaries.

---

## 📁 Directory Structure

```text
build/
├── appicon.png            # Master desktop app icon source (resized automatically by Wails)
├── bin/                   # Compiled standalone binaries and installers (gitignored)
├── darwin/                # macOS specific configuration files
│   ├── Info.plist         # Production macOS application bundle properties
│   └── Info.dev.plist     # Development macOS application bundle properties
└── windows/               # Windows specific configuration files
    ├── icon.ico           # Windows application desktop/explorer icon
    ├── info.json          # Windows installer metadata and executable properties
    └── wails.exe.manifest # Windows app layout security and compatibility manifest
```

---

## ⚙️ Configuration Guides

### 1. Update the Application Icon
To change the application icon across all platforms:
1.  Replace `build/appicon.png` with a high-resolution, square PNG (at least 1024x1024 pixels recommended).
2.  On Windows, you must also convert this image to an ICO file and replace `build/windows/icon.ico`.
3.  Rebuild the application using `wails build -clean`.

### 2. Configure Windows Version Metadata
Open `build/windows/info.json` to change:
-   Product Name and Description
-   Company/Publisher Name
-   Copyright statements
-   Major/Minor/Patch version values

### 3. Configure macOS Bundle Permissions
Edit `build/darwin/Info.plist` to add description keys if the app needs system features (e.g. system notifications, accessibility permissions) or to change macOS version constraints.
