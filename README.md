# moralink-updater

Standalone updater for the **moralinkgost** service.  
Checks GitHub Releases daily and whenever opened manually, with a native Fyne UI.

---

## How it works

| Mode | Trigger | Behaviour |
|---|---|---|
| **GUI** | User opens the binary | Fyne window — shows versions, service status, update button |
| **Headless** | Cron / Task Scheduler | Checks silently, updates automatically, logs to file |

---

## Setup

### 1. Build

```bash
# Requires Go 1.22+ and gcc
make linux    # → dist/moralink-updater-linux-amd64
make windows  # → dist/moralink-updater-windows-amd64.exe (needs mingw-w64)
```

### 2. Configure paths

Edit `internal/github/checker.go` and set:

```go
const owner = "luisgustavorr"   //  GitHub username/org
const repo  = "moralink-updater"   // the repo where you publish releases
```

And the install path in `serviceExecutablePath()`:
```go
// Linux
return "/usr/local/bin/moralinkgost"
// Windows
return `C:\Program Files\MoraLink\moralinkgost.exe`
```

### 3. Register the daily scheduler

```bash
# Linux — adds a crontab entry (runs at 09:00 daily)
sudo ./moralink-updater-linux-amd64 --install-scheduler

# Windows — creates a Task Scheduler task (run as Administrator)
moralink-updater-windows-amd64.exe --install-scheduler
```

To remove:
```bash
./moralink-updater --uninstall-scheduler
```

---

## GitHub Release convention

Name your release assets like:

```
moralinkgost-linux-amd64
moralinkgost-windows-amd64.exe
```

The updater picks the right one based on `runtime.GOOS` + `runtime.GOARCH`.

---

## Releasing a new version

Just push a tag — GitHub Actions builds and publishes automatically:

```bash
git tag v1.2.3
git push origin v1.2.3
```

---

## Log locations

| OS | Path |
|---|---|
| Linux | `/var/log/moralink-updater.log` |
| Windows | `%ProgramData%\MoraLink\updater.log` |
