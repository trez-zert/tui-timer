# Release Notes - v1.1.0

This release adds a mobile-friendly web UI on top of the existing terminal application. The TUI and web UI share the same data files, so you can switch freely between them.

## New Features

- **Web UI Mode**: Run `tuitime --web` to start an HTTP server with a touch-optimized interface for phones and tablets. The startup output shows your LAN and Tailscale IPs for easy mobile access.
- **Timer Screen**: Large Start/Pause/Stop buttons, inline task switching with comment autocomplete suggestions displayed as tappable chips.
- **Day View**: Swipeable date navigation, tap-to-edit entries, inline comment suggestions, add/delete entry support.
- **Reports**: Collapsible daily/weekly/monthly/yearly sections with color-coded progress bars and per-comment breakdowns.
- **Settings**: Toggle report visibility, adjust weekly/yearly/vacation targets, disable goals.
- **PWA Manifest**: "Add to Home Screen" support for native-app-like experience.

## Changed

- **Single Binary**: Both TUI and Web UI are now in a single binary. Run `tuitime` for the terminal app or `tuitime --web` for the web server.
- **Version Bump**: Application version updated to `v1.1.0`.
- **Build Command**: Updated to `go build -o tuitime .` to include all Go source files and embedded static assets.

## Distribution

Binaries for all supported platforms are included in this release under the `binaries/` directory.
