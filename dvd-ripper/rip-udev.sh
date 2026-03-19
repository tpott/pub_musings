#!/bin/bash
# Wrapper for udev — launches rip.py as a user service so it isn't
# killed by udev's execution timeout.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/rip.conf"

systemd-run --user -M "$RIP_USER"@ --unit=dvd-rip-$(date +%s) \
  "$RIP_DIR/rip.py"
