#!/usr/bin/env bash
set -euo pipefail

zcat -f /var/log/nginx/access.log* | goaccess - \
    --log-format=COMBINED \
    --no-global-config \
    --no-progress \
    -o /var/www/html/nginx-dashboard.html