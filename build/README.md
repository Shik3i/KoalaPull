# Build Directory

This folder holds the native build assets used by `wails build`.

## Layout

```text
build/
├── appicon.png            # Source icon, converted by Wails
├── darwin/
│   ├── Info.plist         # macOS app bundle metadata
│   └── Info.dev.plist     # Dev-time macOS bundle metadata
├── windows/
│   ├── icon.ico           # Windows application icon
│   ├── info.json          # Windows version metadata
│   └── wails.exe.manifest # Windows app manifest
└── bin/                   # Build output, gitignored
```

## Notes

- Replace `appicon.png` to change the app icon.
- Edit `darwin/Info.plist` to change the macOS bundle metadata.
- Edit `windows/info.json` to change Windows version details.
- `build/bin/` is where release binaries land after a build.
