#!/bin/bash
set -euxo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/rip.conf"

DISC_NAME=$(blkid -o value -s LABEL /dev/sr0 || echo "UnknownDisc")

# Set MOVIE_NAME to override the Jellyfin-friendly title, e.g.:
#   MOVIE_NAME="3 Idiots (2009)" ./rip.sh
# TODO auto-detect proper title and year from disc label (e.g. via TMDb API)
# TODO auto detect movie vs tv and update output dirs
MOVIE_NAME="${MOVIE_NAME:-$DISC_NAME}"
# Sanitize names for remote commands (replace spaces/parens with underscores)
SAFE_MOVIE_NAME="${MOVIE_NAME//[^a-zA-Z0-9._-]/_}"
OUTPUT_DIR="$RIP_DIR/Movies/$MOVIE_NAME"
REMOTE_DIR="$BACKUP_DEST/$MOVIE_NAME"
mkdir -p "$OUTPUT_DIR"

# Rip only the longest title (by duration) to avoid grabbing extras
# Field 9 = duration string "H:MM:SS"; more reliable than disk size for Blu-rays
LARGEST_TITLE=$(makemkvcon -r info disc:0 2>/dev/null \
  | awk -F, '/^TINFO:[0-9]+,9,0,/ { gsub(/"/, "", $4); split($4, t, ":"); secs = t[1]*3600 + t[2]*60 + t[3]; if (secs > max) { max = secs; id = $1 } } END { sub(/^TINFO:/, "", id); print id }')
makemkvcon mkv disc:0 "$LARGEST_TITLE" "$OUTPUT_DIR"

# Transcode on the HandBrake host via SSH
# TODO try whisper-cli to generate subtitle files from the audio
largest_mkv=$(ls -S "$OUTPUT_DIR"/*.mkv | head -1)
mkv_basename=$(basename "$largest_mkv")
safe_mkv_basename="${mkv_basename//[^a-zA-Z0-9._-]/_}"

ssh "$HANDBRAKE_HOST" "mkdir -p $HANDBRAKE_WORK_DIR"
scp -O "$largest_mkv" "$HANDBRAKE_HOST:$HANDBRAKE_WORK_DIR/$safe_mkv_basename"
ssh "$HANDBRAKE_HOST" \
  "HandBrakeCLI -i $HANDBRAKE_WORK_DIR/$safe_mkv_basename -o $HANDBRAKE_WORK_DIR/$SAFE_MOVIE_NAME.mp4 -e $HANDBRAKE_ENCODER --encoder-preset $HANDBRAKE_PRESET -q $HANDBRAKE_QUALITY -B $HANDBRAKE_AUDIO_BITRATE"
scp -O "$HANDBRAKE_HOST:$HANDBRAKE_WORK_DIR/$SAFE_MOVIE_NAME.mp4" "$OUTPUT_DIR/$MOVIE_NAME.mp4"
ssh "$HANDBRAKE_HOST" "rm $HANDBRAKE_WORK_DIR/$safe_mkv_basename $HANDBRAKE_WORK_DIR/$SAFE_MOVIE_NAME.mp4"
rm "$largest_mkv"

# TODO organize and rename extras into featurettes/ subfolder for Jellyfin
# Jellyfin recognizes extras in: featurettes/, behind the scenes/, deleted scenes/
# e.g. mkdir -p "$OUTPUT_DIR/featurettes"
#      mv "$OUTPUT_DIR/D1_t01.mp4" "$OUTPUT_DIR/featurettes/Idiots in Ladakh.mp4"
# Extras need manual identification (watch them or look up the DVD's title structure)

# Sync main movie to backup host
# Note: the backup-receiver forced command restricts rsync destinations.
# --mkpath requires rsync 3.2.3+ on the remote. If this fails, create the
# destination dir on the backup host first.
rsync --mkpath -avz "$OUTPUT_DIR/$MOVIE_NAME.mp4" "$BACKUP_HOST:$REMOTE_DIR/$MOVIE_NAME.mp4"

# TODO trigger Jellyfin library refresh via API after rsync
# curl -X POST "http://$BACKUP_HOST:8096/Library/Refresh" -H "X-Emby-Token: $(cat ~/.config/jellyfin/api_key)"

# Eject when done
eject /dev/sr0

# Notify via OpenClaw
"$OPENCLAW_BIN" message send --channel matrix --target "$OPENCLAW_TARGET" \
  --message "Rip complete: $MOVIE_NAME is ready in Jellyfin"
