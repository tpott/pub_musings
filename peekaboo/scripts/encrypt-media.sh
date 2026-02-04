#!/bin/bash
# encrypt-media.sh - Encrypts all media files in data/media/ using age encryption
# Creates .age files alongside originals, then optionally removes originals

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MEDIA_DIR="${MEDIA_DIR:-$PROJECT_ROOT/data/media}"
KEY_FILE="${AGE_KEY_FILE:-$PROJECT_ROOT/data/age.key}"

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Encrypts all media files in data/media/ using age encryption.

Options:
    --generate-key    Generate a new age key if one doesn't exist
    --remove-originals  Remove original files after encryption
    --help            Show this help message

Environment variables:
    MEDIA_DIR         Directory containing media files (default: data/media)
    AGE_KEY_FILE      Path to age key file (default: data/age.key)

Example:
    # First time setup - generate key and encrypt
    $0 --generate-key

    # Subsequent runs (key exists)
    $0

    # Encrypt and remove originals (for production)
    $0 --remove-originals
EOF
    exit 0
}

generate_key() {
    if [[ -f "$KEY_FILE" ]]; then
        echo "Key file already exists: $KEY_FILE"
        return 0
    fi

    echo "Generating new age key..."
    mkdir -p "$(dirname "$KEY_FILE")"
    age-keygen -o "$KEY_FILE" 2>/dev/null
    chmod 600 "$KEY_FILE"
    echo "Key file created: $KEY_FILE"
    echo "Public key: $(grep 'public key:' "$KEY_FILE" | sed 's/.*public key: //')"
}

extract_public_key() {
    if [[ ! -f "$KEY_FILE" ]]; then
        echo "ERROR: Key file not found: $KEY_FILE" >&2
        echo "Run with --generate-key first" >&2
        exit 1
    fi

    grep 'public key:' "$KEY_FILE" | sed 's/.*public key: //'
}

encrypt_file() {
    local src="$1"
    local dst="${src}.age"
    local recipient="$2"

    # Skip if already encrypted
    if [[ -f "$dst" ]]; then
        # Check if source is newer than encrypted version
        if [[ "$src" -nt "$dst" ]]; then
            echo "  Re-encrypting: $(basename "$src")"
        else
            echo "  Skipping (already encrypted): $(basename "$src")"
            return 0
        fi
    else
        echo "  Encrypting: $(basename "$src")"
    fi

    age -r "$recipient" -o "$dst" "$src"
}

main() {
    local generate=false
    local remove_originals=false

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --generate-key)
                generate=true
                shift
                ;;
            --remove-originals)
                remove_originals=true
                shift
                ;;
            --help|-h)
                usage
                ;;
            *)
                echo "Unknown option: $1" >&2
                exit 1
                ;;
        esac
    done

    # Check age is installed
    if ! command -v age &>/dev/null; then
        echo "ERROR: age is not installed. Install with: brew install age (macOS) or apt install age (Debian/Ubuntu)" >&2
        exit 1
    fi

    if [[ "$generate" == "true" ]]; then
        generate_key
    fi

    local recipient
    recipient=$(extract_public_key)

    echo "Encrypting media files in $MEDIA_DIR..."
    echo "Using key: $KEY_FILE"
    echo "Public key: $recipient"
    echo ""

    local count=0
    local removed=0

    # Find all media files (jpg, mp3, mp4, webm, etc.) but not .age or LICENSE.txt
    while IFS= read -r -d '' file; do
        encrypt_file "$file" "$recipient"
        ((count++))

        if [[ "$remove_originals" == "true" ]]; then
            echo "  Removing original: $(basename "$file")"
            rm "$file"
            ((removed++))
        fi
    done < <(find "$MEDIA_DIR" -type f \( -name "*.jpg" -o -name "*.jpeg" -o -name "*.png" -o -name "*.mp3" -o -name "*.mp4" -o -name "*.webm" -o -name "*.wav" \) -print0 2>/dev/null || true)

    echo ""
    echo "Done! Processed $count file(s)."
    if [[ "$remove_originals" == "true" ]]; then
        echo "Removed $removed original file(s)."
    fi

    echo ""
    echo "Encrypted files:"
    find "$MEDIA_DIR" -name "*.age" -type f | sort | sed 's|'"$MEDIA_DIR"'/||'
}

main "$@"
