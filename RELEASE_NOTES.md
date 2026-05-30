# Release Notes - v1.2.0

This release brings the TUI and web UI together as concurrent interfaces sharing timer state, plus quality-of-life improvements across both.

## New Features

- **Concurrent TUI + Web UI:** Both now run simultaneously. The web server starts automatically on port 9999 alongside the TUI — no `--web` flag needed. The `--web` flag is still supported for headless/server-only mode.
- **Shared Timer State:** Start a timer from the web UI and it appears in the TUI within a second, and vice versa. Pause, stop, and task switch are synchronized between both interfaces.
- **Web Server Info in TUI:** Press `w` in the Settings screen to view web server URLs (local, LAN IPs, Tailscale IPs). Press `w` again to hide details.
- **Instance Lock:** A lock file prevents running multiple copies of tuitime simultaneously, protecting your log files from corruption. Stale locks (older than 5 minutes) are automatically cleaned up. Lock is properly cleaned on both Ctrl+C and `--help` exit.
- **Report Sort Toggle:** Press `s` in TUI reports or click the `⇅` button in web reports to cycle through three sort modes: alphabetical, by hours spent (descending), or by most recently used.
- **Day Total:** Both TUI and web Day View now display a running total of all hours logged for the selected day.
- **Today's Total on Timer:** The timer screen now shows "Today: Xh Ym" below the clock, combining already-logged entries with the currently running session.
- **Week Date Range:** Weekly report entries now show the Monday–Sunday date range (e.g. "2026-W22 (May 25-31)") for clarity.
- **HHMM Time Format:** Time inputs now accept four digits without a colon (e.g. "0900") in addition to "09:00". The web UI automatically inserts a colon as you type.
- **--help Flag:** Run `tuitime --help` to see usage syntax.

## Fixed

- **Report Page Overflow:** The page number display no longer exceeds the actual number of available pages when navigating reports.
- **Daily Totals Sort Order:** Now consistently sorted newest-to-oldest across all report sections.
- **Ctrl+C Cleanup:** The lock file is now reliably removed on Ctrl+C in both TUI and headless web modes.

## Changed

- Version bumped to v1.2.0.
- Default port changed from 8080 to 9999 to avoid conflicts with common services.
- Default behavior: running `tuitime` now starts both the TUI and web server. Use `tuitime --web` for headless web-only mode.
