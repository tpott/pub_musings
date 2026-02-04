#!/bin/bash
# source-media.sh - Downloads CC0/public domain media for the 6 MVP animals
# All photos from Wikimedia Commons, all audio from Internet Archive
# License: CC0 / Public Domain

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MEDIA_DIR="$PROJECT_ROOT/data/media"

# Photo URLs (Wikimedia Commons - resized to 640px width)
declare -A PHOTO_URLS=(
    ["cat"]="https://upload.wikimedia.org/wikipedia/commons/thumb/3/3a/Cat03.jpg/640px-Cat03.jpg"
    ["dog"]="https://upload.wikimedia.org/wikipedia/commons/thumb/2/2d/Dog_-_%E0%B4%A8%E0%B4%BE%E0%B4%AF-6.JPG/640px-Dog_-_%E0%B4%A8%E0%B4%BE%E0%B4%AF-6.JPG"
    ["cow"]="https://upload.wikimedia.org/wikipedia/commons/thumb/0/0c/Cow_female_black_white.jpg/640px-Cow_female_black_white.jpg"
    ["pig"]="https://upload.wikimedia.org/wikipedia/commons/thumb/3/3e/Pig_farm_Vampula_1.jpg/640px-Pig_farm_Vampula_1.jpg"
    ["chicken"]="https://upload.wikimedia.org/wikipedia/commons/thumb/8/84/Male_and_female_chicken_sitting_together.jpg/640px-Male_and_female_chicken_sitting_together.jpg"
    ["duck"]="https://upload.wikimedia.org/wikipedia/commons/thumb/b/bf/Bucephala-albeola-010.jpg/640px-Bucephala-albeola-010.jpg"
)

# Audio URLs (Internet Archive - CC0/Public Domain)
declare -A AUDIO_URLS=(
    ["cat"]="https://archive.org/download/animal_201701/Cat.mp3"
    ["dog"]="https://archive.org/download/animal_201701/Dog.mp3"
    ["cow"]="https://archive.org/download/animal_201701/Cows.mp3"
    ["pig"]="https://archive.org/download/animal_201701/Pig.mp3"
    ["chicken"]="https://archive.org/download/animal_201701/Chicken.mp3"
    ["duck"]="https://archive.org/download/duck-sounds/duck%20sounds.mp3"
)

# License text for all downloaded media
LICENSE_TEXT='All media in this directory is CC0 / Public Domain.

Photos sourced from Wikimedia Commons:
- Cat: https://commons.wikimedia.org/wiki/File:Cat03.jpg
- Dog: https://commons.wikimedia.org/wiki/File:Dog_-_%E0%B4%A8%E0%B4%BE%E0%B4%AF-6.JPG
- Cow: https://commons.wikimedia.org/wiki/File:Cow_female_black_white.jpg
- Pig: https://commons.wikimedia.org/wiki/File:Pig_farm_Vampula_1.jpg
- Chicken: https://commons.wikimedia.org/wiki/File:Male_and_female_chicken_sitting_together.jpg
- Duck: https://commons.wikimedia.org/wiki/File:Bucephala-albeola-010.jpg

Audio sourced from Internet Archive:
- Cat, Dog, Cow, Pig, Chicken: https://archive.org/details/animal_201701
- Duck: https://archive.org/details/duck-sounds

You may use these files for any purpose without attribution.
'

download_file() {
    local url="$1"
    local dest="$2"

    echo "  Downloading: $(basename "$dest")"
    if ! curl -fsSL --max-time 60 -o "$dest" "$url"; then
        echo "  ERROR: Failed to download $url" >&2
        return 1
    fi
}

main() {
    echo "Downloading CC0/Public Domain media for Peekaboo..."
    echo "Media directory: $MEDIA_DIR"
    echo ""

    local failed=0

    for animal in cat dog cow pig chicken duck; do
        echo "[$animal]"
        local animal_dir="$MEDIA_DIR/$animal/set1"
        mkdir -p "$animal_dir"

        # Download photo
        if ! download_file "${PHOTO_URLS[$animal]}" "$animal_dir/photo.jpg"; then
            ((failed++))
        fi

        # Download audio
        if ! download_file "${AUDIO_URLS[$animal]}" "$animal_dir/audio.mp3"; then
            ((failed++))
        fi

        # Write license file
        echo "$LICENSE_TEXT" > "$animal_dir/LICENSE.txt"
        echo "  Created: LICENSE.txt"
        echo ""
    done

    if [[ $failed -gt 0 ]]; then
        echo "WARNING: $failed download(s) failed. Check errors above."
        exit 1
    fi

    echo "Done! All media downloaded to $MEDIA_DIR"
    echo ""
    echo "Directory structure:"
    find "$MEDIA_DIR" -type f | sort | sed 's|'"$MEDIA_DIR"'/||'
}

main "$@"
