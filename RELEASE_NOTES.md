# Release Notes - v1.0.4

This release addresses a critical stability issue in the Report View and improves the repository structure for better distribution.

## Bug Fixes

- **Resolved Report View Crash**: Fixed a `runtime error: slice bounds out of range` panic. This crash occurred when a user had scrolled down in a report and then disabled "Monthly" or "Yearly" totals in the settings. The reduction in content caused the scroll cursor to point to an invalid (out-of-bounds) location.
- **Improved UI Stability**: The report view now robustly handles dynamic content changes. If the content shrinks (e.g., due to toggling section visibility), the view automatically adjusts the scroll position to remain within the new bounds.
- **Navigation UX**: Returning from the Settings menu to the Report View now automatically resets the scroll position to the top, providing a cleaner and more predictable user experience.

## Improvements

- **In-App Versioning**: The current version number (`v1.0.4`) is now clearly displayed in the application's Setup view.
- **Repository Reorganization**: All platform-specific binaries have been moved into a structured `binaries/` directory. This keeps the root of the repository clean while providing easy access to pre-compiled versions for Linux, macOS, Windows, FreeBSD, and Raspberry Pi.
- **Documentation**: Added a comprehensive `CHANGELOG.md` to track project evolution.

## Distribution

Binaries for all supported platforms are included in this release under the `binaries/` directory:
- **Linux**: `amd64`, `arm64` (RPi 4/5), `armv7` (RPi 3/4)
- **macOS**: `arm64` (Apple Silicon), `amd64` (Intel)
- **Windows**: `amd64`, `arm64`
- **FreeBSD**: `amd64`
