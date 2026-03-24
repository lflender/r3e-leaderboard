# Nginx Dashboard — Automatic Daily Update

The dashboard at `https://r3e-leaderboards.info/nginx-dashboard.html` is generated
by [GoAccess](https://goaccess.io) from the nginx access logs.  
These files set it up to regenerate automatically every day via a **systemd timer**.

This document reflects the working setup currently used on the server:
- The update script lives at `/root/update-nginx-dashboard.sh`
- The generated report is written to `/var/www/html/nginx-dashboard.html`
- The systemd unit files live in `/etc/systemd/system/`
- The timer runs daily at `00:05 UTC`

---

## Files

| File | Purpose |
|------|---------|
| `update-nginx-dashboard.sh` | Runs GoAccess over nginx access logs and writes `/var/www/html/nginx-dashboard.html` |
| `goaccess-report.service` | systemd oneshot service that executes the script |
| `goaccess-report.timer` | systemd timer — fires daily at 00:05 UTC |

---

## One-time Setup (run on the server over SSH)

### 1. Copy files to the server

From your local machine (Windows PowerShell), copy the files to a temporary place on the server:

```powershell
scp deploy/update-nginx-dashboard.sh  root@r3e-leaderboards.info:/root/
scp deploy/goaccess-report.service    root@r3e-leaderboards.info:/root/deploy/
scp deploy/goaccess-report.timer      root@r3e-leaderboards.info:/root/deploy/
```

If you prefer, you can copy them manually with WinSCP to the same paths.

### 2. Verify the script output path

The working script writes the HTML dashboard here:

```bash
/var/www/html/nginx-dashboard.html
```

That path matters because it is the file nginx is serving publicly.

The script itself should be the short version below:

```bash
#!/usr/bin/env bash
set -euo pipefail

zcat -f /var/log/nginx/access.log* | goaccess - \
	--log-format=COMBINED \
	--no-global-config \
	--no-progress \
	-o /var/www/html/nginx-dashboard.html
```

### 3. Install and enable on the server

```bash
# Make the script executable
chmod +x /root/update-nginx-dashboard.sh

# Install the systemd units
cp /root/deploy/goaccess-report.service /etc/systemd/system/
cp /root/deploy/goaccess-report.timer   /etc/systemd/system/

# Reload systemd, enable and start the timer
systemctl daemon-reload
systemctl enable --now goaccess-report.timer

# Verify the timer is active
systemctl list-timers goaccess-report.timer --no-pager
```

### 4. Run it once immediately to verify

```bash
systemctl start goaccess-report.service

# Check it succeeded
systemctl status goaccess-report.service --no-pager
# or view logs
journalctl -u goaccess-report.service -n 50 --no-pager
```

The dashboard should now be updated at the URL within seconds.

For a direct manual test without systemd:

```bash
/root/update-nginx-dashboard.sh
ls -lh /var/www/html/nginx-dashboard.html
```

---

## Troubleshooting

**GoAccess not found**

```bash
apt-get install goaccess
```

**Wrong log format**

If the dashboard shows 0 valid requests, your nginx might use a custom log format.  
Check `/etc/nginx/nginx.conf` for a `log_format` directive and pass the matching
`--log-format` option in the script.

**Service succeeds but the page does not change**

Make sure the script writes to the file nginx is actually serving:

```bash
stat /var/www/html/nginx-dashboard.html
```

If that timestamp changes after running `/root/update-nginx-dashboard.sh`, the page source is being updated correctly.

**Current day looks abnormally low**

That is expected if the report is regenerated shortly after midnight. The current day is still in progress, so the last point on the graph may be much lower than previous days.

---

## Checking the timer schedule

```bash
systemctl list-timers --all | grep goaccess
```

The `NEXT` column shows when it will next fire.
