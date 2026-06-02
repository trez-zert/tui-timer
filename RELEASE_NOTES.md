# Release Notes - v1.2.1

This patch release adds overlap detection in the Day View, a past start time option for the web UI, and improved touch-friendliness.

## New Features

- **Overlap Highlighting:** Entries with overlapping time ranges in the Day View are now highlighted in red (TUI: red-tinted background, Web: red border and background). Makes accidental double-bookings immediately visible.
- **Web Start Time:** The "Start Timer" dialog in the web UI now includes an optional start time field. Enter a past time to start the timer retroactively.

## Fixed

- **TUI Past Start Time:** Entering an earlier start time (e.g., "09:00" at 15:00) no longer overrides the entered time with the current time.

## Changed

- **Touch-Friendly Inputs:** All text inputs in the web UI now have a minimum height of 48px (Apple's recommended touch target), with increased padding (14px) and larger font size (1.05rem). Suggestion chips are also larger for easier tapping.
