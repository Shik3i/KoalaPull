# Build Directory

This directory holds platform-specific assets used by `wails build` to produce native packages.

## Layout

```
build/
├── appicon.png            # Source icon (256×256 PNG, converted to platform formats)
├── darwin/
│   ├── Info.plist         # macOS app bundle metadata (CFBundleName, etc.)
│   └── Info.dev.plist     # Same as above, used during `wails dev`
├── windows/
│   ├── icon.ico           # Windows application icon
│   ├── info.json          # Windows file version metadata (right-click → Properties)
│   ├── installer/         # Windows installer assets (NSIS/inno)
│   └── wails.exe.manifest # Windows application manifest (DPIAware, etc.)
└── bin/                   # Build output (gitignored)
    ├── koalapull          # macOS / Linux binary
    └── koalapull.exe      # Windows binary
```

## Customisation

- Replace `appicon.png` with your own 256×256 PNG. Wails will convert it to the platform formats on next build.
- Edit `darwin/Info.plist` to change the macOS bundle identifier, version, or category.
- Edit `windows/info.json` to change the Windows file version, copyright, or description.
