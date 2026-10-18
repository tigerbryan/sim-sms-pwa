#!/usr/bin/env bash
set -Eeuo pipefail
printf '%s\n' 'Follow docs/DEPLOYMENT.md: back up, build the reviewed release, then rerun install-native.sh with the same explicit device.' >&2
exit 1
