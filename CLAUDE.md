# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Windows-only GUI tool (Go + `lxn/walk`) that clears stuck print jobs. The UI text is Thai and aimed at non-technical users.

## Commands

```sh
go build -ldflags="-H windowsgui -s -w" -o ClearPrintQueue.exe   # flags the shipped exe was built with
go test ./...
go test ./spooler -run TestClearDir -v
```

- `-H windowsgui` keeps a console window from opening. Without it the app still runs.
- `rsrc_windows_amd64.syso` holds the app manifest, and `go build` links it in automatically. It requests `requireAdministrator` (stopping the spooler needs admin), turns on Common-Controls v6 (walk does not work without it) and sets per-monitor DPI awareness. If you delete or rename this file, the exe still builds but no longer works. It also holds the app icon (`winres/icon.ico`, drawn from `winres/icon.svg`) as resource ID 1, and `main.go` sets `Icon: 1` because walk only looks for ID 7 on its own. There is no source manifest in the repo. Regenerate the `.syso` with exactly these flags, or the admin prompt or icon is silently lost:
  `go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --manifest gui --admin --product-name "Clear Print Queue" --file-description "Clear Print Queue" --icon winres/icon.ico`
- `TestListReadsInstalledPrinters` calls the real `EnumPrintersW`. It checks the struct layout, so the printers it logs need readable names.

## Architecture

- `main.go`: the whole UI, built with walk's declarative API (dot-imported). `spooler.Clear()` runs in a goroutine, and every UI update after it goes through `mw.Synchronize`.
- `spooler/`: raw Win32 calls with no extra libraries.
  - `List` and `defaultPrinter` call `winspool.drv` through lazy procs. `printerInfo2` mirrors `PRINTER_INFO_2W` field for field, so its field order and types must not change.
  - `Clear` runs in this order: stop the services that depend on the Spooler (Fax, for example) → stop the Spooler → if it hangs, kill `spoolsv.exe` by PID → delete the files at the top level of the spool directory → start the Spooler again. The Spooler is **always** restarted, even when an earlier step fails. Dependent services are restarted through `defer`, so they come back after the Spooler. Keep that order.
  - The spool directory comes from the registry value `HKLM\...\Print\Printers\DefaultSpoolDirectory`. If that value is missing, the code uses `%SystemRoot%\System32\spool\PRINTERS`. `clearDir` deletes only the files directly inside that folder and leaves subfolders alone.
