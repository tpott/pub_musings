#!/usr/bin/env bash
# start-piper-server.sh - Starts the Piper TTS HTTP server
#
# Piper is a fast, local neural text-to-speech engine. This script activates
# the Python venv and starts the Flask-based HTTP server for use by the
# peekaboo Go backend.
#
# Usage:
#   ./scripts/start-piper-server.sh
#
# Environment:
#   PIPER_PORT  - Server port (default: 8051)
#   PIPER_HOST  - Server host (default: 0.0.0.0)
#   PIPER_VOICE - Voice model name (default: en_US-lessac-high)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

PIPER_PORT="${PIPER_PORT:-8051}"
PIPER_HOST="${PIPER_HOST:-0.0.0.0}"
PIPER_VOICE="${PIPER_VOICE:-en_US-lessac-high}"
VOICE_DIR="$PROJECT_ROOT/data/piper-voices"
VENV_DIR="$PROJECT_ROOT/.venv"

main() {
    if [[ ! -d "$VENV_DIR" ]]; then
        echo "ERROR: Python venv not found at $VENV_DIR" >&2
        echo "Create it with: python3 -m venv $VENV_DIR" >&2
        exit 1
    fi

    # shellcheck disable=SC1091
    source "$VENV_DIR/bin/activate"

    if ! python3 -c "import piper" 2>/dev/null; then
        echo "ERROR: piper-tts not installed in venv" >&2
        echo "Install with: pip install -r $PROJECT_ROOT/requirements-piper.txt" >&2
        exit 1
    fi

    local model_file="$VOICE_DIR/$PIPER_VOICE.onnx"
    if [[ ! -f "$model_file" ]]; then
        echo "ERROR: Voice model not found: $model_file" >&2
        echo "Download with: python3 -m piper.download_voices $PIPER_VOICE --download-dir $VOICE_DIR" >&2
        exit 1
    fi

    echo "Starting Piper TTS server..."
    echo "  Voice: $PIPER_VOICE"
    echo "  Port:  $PIPER_PORT"
    echo "  Host:  $PIPER_HOST"

    exec python3 -m piper.http_server \
        -m "$PIPER_VOICE" \
        --data-dir "$VOICE_DIR" \
        --host "$PIPER_HOST" \
        --port "$PIPER_PORT"
}

main "$@"
