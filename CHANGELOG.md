# Changelog

All notable changes to this project will be documented in this file.

## [1.0.4] - 2026-05-20

### Fixed
- Fixed a `runtime error: slice bounds out of range` panic in the Report View. This occurred when scrolling down in a long report and then disabling "Monthly" or "Yearly" totals in Settings, causing the report to shrink while the scroll cursor remained at an invalid position.
- Fixed an issue where the report would appear "empty" or "stuck" after returning from Settings if the previous scroll position was beyond the new end of the report.

### Changed
- Improved scroll cursor management: the view now robustly clamps the cursor to the available content.
- Navigation improvement: The report scroll position is now reset to the top when returning from the Settings menu to ensure a consistent view.
- Repository structure: Reorganized release binaries and archives into a dedicated `binaries/` directory with platform-specific subfolders for better project cleanliness.

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
