#!/usr/bin/env bash
set -euo pipefail

# Only use rotated logs (exclude the current in-progress access.log)
# so the last visible day is always a complete day
ls -1 /var/log/nginx/access.log.1 /var/log/nginx/access.log.*.gz 2>/dev/null \
    | sort -Vr \
    | xargs -r zcat -f \
    | goaccess - \
        --log-format=COMBINED \
        --no-global-config \
        --no-progress \
        -o /var/www/html/nginx-dashboard.html