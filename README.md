# tuitime

A terminal-based time tracking suite with real-time visualization, goal tracking, and scalable log management — now with a mobile-friendly web UI.

## Features

### Terminal (TUI)
- **Unified Application:** Access timer setup, historical day management, and reports from a single hub.
- **Smart Setup:** Focused screen for Start Time, End Time, and Comments with real-time format validation.
- **Timer Modes:** Log time in real-time or retrospectively with manual entry.
- **Advanced Day View:** Navigate through time, edit entries in-place, delete them, or add new retrospective logs.
- **Full Undo:** Made a mistake? Press **u** in the Day View to instantly revert all changes from your current session.
- **High-Res ASCII Clock:** Choose between Plain Text, Small (5-row), or Large (7-row) blocky clocks.
- **Customizable Themes:** Cycle through 7 colors for your timer display.
- **Goal Visualization:** Heatmaps and progress bars (e.g., "34h of 40h") based on your custom targets.
- **Flexible Goals:** Set a **Weekly** or **Yearly** target and the app will synchronize the other automatically.
- **Vacation Support:** Specify vacation days per year and the app will automatically adjust your work targets to compensate.
- **Report Customization:** Toggle the visibility of Daily, Weekly, Monthly, and Yearly sections.
- **Intelligent Autocomplete:** Remembers your 50 most recent unique comments with instant, non-destructive navigation.
- **Scalable Logs:** Automatically organizes entries into `logs/YYYY/MM-MonthName.md`.

### Web UI (Mobile-Friendly)
- **Touch-Optimized Timer:** Large Start/Pause/Stop buttons, inline task switching with comment suggestions.
- **Day View:** Swipeable date navigation, tap-to-edit entries, inline comment autocomplete chips.
- **Reports:** Collapsible daily/weekly/monthly/yearly sections with progress bars and per-comment breakdowns.
- **Settings:** Toggle report sections, adjust weekly/yearly/vacation targets.
- **Responsive:** Mobile-first layout with bottom tab navigation, works on phones and tablets.
- **PWA Ready:** Add-to-Home-Screen support via manifest.
- **Seamless Sharing:** Web UI and TUI share the same log files, config, and recent comments — switch freely between them.

---

## Visual Preview

### 1. Timer Setup & Navigation
![screenshot of timer setup and navigation screen](timer-setup.png)

### 2. Live Tracking (Multiple Clock Modes)
| Plain Text | Small ASCII | Large ASCII |
| :---: | :---: | :---: |
| ![screenshot of plain text clock](plain-text-timer.png) | ![screenshot of small ASCII clock](small-ascii-timer.png) | ![screenshot of large ASCII clock](large-ascii-timer.png) |

### 3. Advanced Day View (Historical Editing)
![screenshot of day view with entry editing](day-view.png)

### 4. Reports & Heatmaps
![screenshot of reports and heatmaps](reports.png)

### 5. Goal & Vacation Settings
![screenshot of goal and vacation settings](goal-settings.png)

---

## Installation & Usage

### Binaries
Pre-compiled binaries for the following platforms are available in the [Releases](https://github.com/Trez-zerT/TUI-timer/releases) section or within the [binaries/](./binaries) directory of this repository.

See [CHANGELOG.md](./CHANGELOG.md) for a full history of changes.

| Platform | Architecture | Recommended Use |
| :--- | :--- | :--- |
| **Linux** | `amd64` | Standard PCs, Servers |
| **Linux** | `arm64` | Raspberry Pi 4/5 (64-bit OS) |
| **Linux** | `armv7` | Raspberry Pi 3/4 (32-bit OS) |
| **macOS** | `arm64` | Apple Silicon (A/M-series chips) |
| **macOS** | `amd64` | Intel-based Macs |
| **Windows** | `amd64` | Standard PCs |
| **Windows** | `arm64` | Windows on ARM devices |
| **FreeBSD** | `amd64` | Servers |

### Running (TUI + Web UI)
1. **Launch:** Run `tuitime`. Both the terminal UI and web server (port 9999) start simultaneously.
2. **Navigate:** Use **Arrow Keys (Up/Down)** to move between the top menu and the input fields.
3. **Select View:** Use **Left/Right** on the top menu to select a tool and press **Enter**.
4. **Exit:** Press **q** or **Esc** while the top menu is focused to quit.
5. **Web Info:** Press `w` in the Settings screen to view web server URLs for mobile access.
6. **Timer Sync:** Start a timer from the web UI and it appears in the TUI within a second, and vice versa.

### Running (Headless Web Server)
1. Run `tuitime --web` to start the HTTP server without the TUI.
2. Open the displayed URL in your phone's browser.

```bash
./tuitime --web              # headless, default port 9999
./tuitime --web --port 9090  # headless, custom port
```

### Controls (TUI)
- **Timer:** 
    - **p** - Pause/Resume.
    - **t** - Switch tasks (logs current and starts new).
    - **s** - Stop and log.
    - **,** - Change clock settings.
    - **w** - Toggle web server info (in Settings view).
- **Day View:** 
    - **Arrows** - Navigate entries and days (navigate to header to change day).
    - **Enter** - Edit field in-place.
    - **a** - Add manual entry for the current day.
    - **del** - Delete entry.
    - **u** - Undo current session changes.
- **Reports:**
    - **←/→** - Navigate pages.
    - **s** - Cycle sort mode (alpha / hours / recent).
    - **up/down** - Scroll within the current page.
    - **,** - Change target and visibility settings.
- **Settings:**
    - **w** - Toggle web server info (URLs for mobile access).

---

## Building from Source

If you have Go installed, you can build tuitime yourself:

1. Clone the repository:
   ```bash
   git clone https://github.com/Trez-zerT/TUI-timer.git
   cd TUI-timer/timetracker-code
   go mod tidy
   ```
2. Build for your current platform (TUI + Web UI in a single binary):
   ```bash
   go build -o tuitime .
   ```
3. Cross-compile for all platforms:
   ```bash
   # Linux
   GOOS=linux GOARCH=amd64 go build -o tuitime-linux/tuitime .
   GOOS=linux GOARCH=arm64 go build -o tuitime-rpi64/tuitime .
   GOOS=linux GOARCH=arm GOARM=7 go build -o tuitime-rpi32/tuitime .
   
   # macOS
   GOOS=darwin GOARCH=arm64 go build -o tuitime-macos-arm/tuitime .
   GOOS=darwin GOARCH=amd64 go build -o tuitime-macos-intel/tuitime .
   
   # Windows
   GOOS=windows GOARCH=amd64 go build -o tuitime-win-amd64/tuitime.exe .
   GOOS=windows GOARCH=arm64 go build -o tuitime-win-arm64/tuitime.exe .
   
   # FreeBSD
   GOOS=freebsd GOARCH=amd64 go build -o tuitime-freebsd/tuitime .
   ```

---

## Log Structure
Logs are stored relative to the executable:
```text
.
├── logs/
│   └── 2026/
│       ├── 05-May.md
│       └── 06-June.md
├── config.json
└── recent_comments.json
```

## License
MIT License - see `LICENSE` for details.
