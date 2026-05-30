# Changelog

All notable changes to this project will be documented in this file.

## [1.2.0] - 2026-05-30

### Added
- **Concurrent TUI + Web:** Both the terminal UI and web server now run simultaneously on port 9999. No need for `--web` flag (still supported for headless mode).
- **Shared timer state:** Start a timer from the web UI and it appears in the TUI within a second, and vice versa. Pause, stop, and task switch are synchronized between both interfaces.
- **Web server info in TUI Settings:** Press `w` in the Settings screen to view web server URLs (local, LAN, Tailscale).
- **Instance lock:** Prevents running multiple copies of tuitime simultaneously to avoid data corruption. Stale locks (5+ min) automatically cleaned. Properly released on Ctrl+C and `--help`.
- **Report sort toggle:** Press `s` in TUI reports or click the `⇅` button in the web reports to cycle sort modes: alphabetical, by hours spent (descending), or by most recently used.
- **Day total in Day View:** Both TUI and web Day View now show a running total of all hours for the selected day.
- **Today's total on Timer screen:** Both TUI and web timer screens show today's logged time (including the currently running session) below the clock.
- **Week date range labels:** Weekly report entries now show the Monday–Sunday date range (e.g. "2026-W22 (May 25-31)").
- **HHMM time format:** Time inputs now accept "0900" without a colon (in addition to "09:00"). Auto-colon inserts `:` after typing 2 digits in the web UI.
- **--help flag:** Run `tuitime --help` to see usage syntax.

### Fixed
- **Report page overflow:** The page number display no longer exceeds the actual number of available pages.
- **Daily totals sort order:** Now consistently sorted newest-to-oldest (matching weekly/monthly/yearly).
- **Lock file cleanup:** `os.Exit` paths (including `--help`) now properly release the instance lock.

### Changed
- Version bumped to v1.2.0.
- Default port changed from 8080 to 9999.

## [1.1.0] - 2026-05-27

### Added
- **Mobile-Friendly Web UI:** New `--web` flag launches an HTTP server with a touch-optimized SPA for phones and tablets. Open the displayed LAN/Tailscale URL in your phone's browser.
  - Timer with large Start/Pause/Stop buttons, inline task switching, and comment autocomplete chips.
  - Day View with swipeable date navigation, tap-to-edit/delete entries, and suggestion chips when typing comments.
  - Reports with collapsible daily/weekly/monthly/yearly sections, progress bars, and per-comment breakdowns.
  - Settings for toggling report sections, disabling goals, and adjusting weekly/yearly/vacation targets.
  - PWA manifest for "Add to Home Screen" support.
- **Shared Data Layer:** Refactored all file I/O into `pkg/timedata/` so the TUI and web server share the same data code. Both modes read/write the identical log files, config, and recent comments — switch freely between them.
- **Server-Side Timer State:** The web server tracks timer state (running, paused, elapsed) in memory, surviving page refreshes.

### Changed
- Version bumped to v1.1.0.
- Single binary now serves both TUI and Web modes (`tuitime` for TUI, `tuitime --web` for the web server).
- Build command updated: `go build -o tuitime .` (includes `web.go` and embedded static files).

## [1.0.5] - 2026-05-21

### Fixed
- Fixed progress bar alignment issues in the Report View by enforcing fixed-width column layouts for key names and time values.
- Yearly totals in the Report View are now properly aligned even with large values.

### Changed
- Bumped version to v1.0.5.

## [1.0.4] - 2026-05-20

### Fixed
- Fixed a `runtime error: slice bounds out of range` panic in the Report View. This occurred when scrolling down in a long report and then disabling "Monthly" or "Yearly" totals in Settings, causing the report to shrink while the scroll cursor remained at an invalid position.
- Fixed an issue where the report would appear "empty" or "stuck" after returning from Settings if the previous scroll position was beyond the new end of the report.

### Changed
- Improved scroll cursor management: the view now robustly clamps the cursor to the available content.
- Navigation improvement: The report scroll position is now reset to the top when returning from the Settings menu to ensure a consistent view.
- Repository structure: Reorganized release binaries and archives into a dedicated `binaries/` directory with platform-specific subfolders.
- In-App Versioning: Display v1.0.4 in the Setup view.

## [1.0.3] - 2026-05-19

### Added
- Unified application structure: Access all tools (Timer, Reports, Day View) from a single hub menu.
- Context-aware settings: The Settings menu now dynamically adjusts options based on whether you are in the Timer or Report view.
- Bidirectional Goal Sync: Setting a Weekly target now automatically calculates the Yearly target and vice-versa.
- Vacation Support: Added a "Vacation Days" setting which automatically adjusts work hour targets to compensate for time off.
- Report Customization: Added toggles to show/hide Daily, Weekly, Monthly, and Yearly totals in reports.

### Fixed
- Improved terminal height detection for report scrolling.
- Enhanced comment suggestion navigation and revert behavior.
- Various fixes for menu navigation and state management.

## [1.0.2] - 2026-05-18

### Added
- Unified `tuitime` executable: Merged the separate tracker and reporter tools into one application.
- Top-level navigation menu for switching between views.
- Advanced "Day View": New interface for navigating history, editing entries in-place, and manual logging.
- Undo System: Press 'u' in the Day View to revert all changes made during the current session.
- Sanity Checks: Added validation for time formats and protection against accidental negative durations.

## [1.0.1] - 2026-05-15

### Added
- Initial release of the suite with basic timer and reporting functionality.

## [1.1.0] - 2026-05-27

### Added
- **Mobile-Friendly Web UI:** New `--web` flag launches an HTTP server with a touch-optimized SPA for phones and tablets. Open the displayed LAN/Tailscale URL in your phone's browser.
  - Timer with large Start/Pause/Stop buttons, inline task switching, and comment autocomplete chips.
  - Day View with swipeable date navigation, tap-to-edit/delete entries, and suggestion chips when typing comments.
  - Reports with collapsible daily/weekly/monthly/yearly sections, progress bars, and per-comment breakdowns.
  - Settings for toggling report sections, disabling goals, and adjusting weekly/yearly/vacation targets.
  - PWA manifest for "Add to Home Screen" support.
- **Shared Data Layer:** Refactored all file I/O into `pkg/timedata/` so the TUI and web server share the same data code. Both modes read/write the identical log files, config, and recent comments — switch freely between them.
- **Server-Side Timer State:** The web server tracks timer state (running, paused, elapsed) in memory, surviving page refreshes.

### Changed
- Version bumped to v1.1.0.
- Single binary now serves both TUI and Web modes (`tuitime` for TUI, `tuitime --web` for the web server).
- Build command updated: `go build -o tuitime .` (includes `web.go` and embedded static files).

## [1.0.5] - 2026-05-21

### Fixed
- Fixed progress bar alignment issues in the Report View by enforcing fixed-width column layouts for key names and time values.
- Yearly totals in the Report View are now properly aligned even with large values.

### Changed
- Bumped version to v1.0.5.

## [1.0.4] - 2026-05-20

### Fixed
- Fixed a `runtime error: slice bounds out of range` panic in the Report View. This occurred when scrolling down in a long report and then disabling "Monthly" or "Yearly" totals in Settings, causing the report to shrink while the scroll cursor remained at an invalid position.
- Fixed an issue where the report would appear "empty" or "stuck" after returning from Settings if the previous scroll position was beyond the new end of the report.

### Changed
- Improved scroll cursor management: the view now robustly clamps the cursor to the available content.
- Navigation improvement: The report scroll position is now reset to the top when returning from the Settings menu to ensure a consistent view.
- Repository structure: Reorganized release binaries and archives into a dedicated `binaries/` directory with platform-specific subfolders.
- In-App Versioning: Display v1.0.4 in the Setup view.

## [1.0.3] - 2026-05-19

### Added
- Unified application structure: Access all tools (Timer, Reports, Day View) from a single hub menu.
- Context-aware settings: The Settings menu now dynamically adjusts options based on whether you are in the Timer or Report view.
- Bidirectional Goal Sync: Setting a Weekly target now automatically calculates the Yearly target and vice-versa.
- Vacation Support: Added a "Vacation Days" setting which automatically adjusts work hour targets to compensate for time off.
- Report Customization: Added toggles to show/hide Daily, Weekly, Monthly, and Yearly totals in reports.

### Fixed
- Improved terminal height detection for report scrolling.
- Enhanced comment suggestion navigation and revert behavior.
- Various fixes for menu navigation and state management.

## [1.0.2] - 2026-05-18

### Added
- Unified `tuitime` executable: Merged the separate tracker and reporter tools into one application.
- Top-level navigation menu for switching between views.
- Advanced "Day View": New interface for navigating history, editing entries in-place, and manual logging.
- Undo System: Press 'u' in the Day View to revert all changes made during the current session.
- Sanity Checks: Added validation for time formats and protection against accidental negative durations.

## [1.0.1] - 2026-05-15

### Added
- Initial release of the suite with basic timer and reporting functionality.
