#!/bin/bash
# Generate test audio files from seed YAML files using Piper TTS
#
# Requirements:
#   pip install piper-tts PyYAML
#
# Usage:
#   ./generate.sh              # Generate from all seeds
#   ./generate.sh english-basic # Generate from specific seed
#
# Output:
#   evaluation/audio/<seed-id>/<segment-id>.wav
#
# Note: Generated audio is gitignored - regenerate as needed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SEEDS_DIR="$SCRIPT_DIR/seeds"
AUDIO_DIR="$SCRIPT_DIR/audio"

# Check for piper
if ! command -v piper &> /dev/null; then
    echo "Error: piper not found. Install with: pip install piper-tts"
    exit 1
fi

# Check for yq (YAML processor) or use python
if command -v yq &> /dev/null; then
    YAML_PARSER="yq"
else
    YAML_PARSER="python"
fi

# Parse YAML and generate audio using Python
generate_from_seed() {
    local seed_file="$1"
    local seed_id=$(basename "$seed_file" .yaml)
    local output_dir="$AUDIO_DIR/$seed_id"

    echo "Generating audio for seed: $seed_id"
    mkdir -p "$output_dir"

    python3 << EOF
import yaml
import subprocess
import sys
import os

with open("$seed_file", 'r') as f:
    seed = yaml.safe_load(f)

voice = seed.get('voice', 'en_US-lessac-medium')
segments = seed.get('segments', [])

for seg in segments:
    seg_id = seg.get('id', 'segment')
    text = seg.get('text', '')
    output_file = "$output_dir/" + seg_id + ".wav"

    if os.path.exists(output_file):
        print(f"  Skipping {seg_id} (already exists)")
        continue

    print(f"  Generating {seg_id}...")
    try:
        proc = subprocess.run(
            ['piper', '--model', voice, '--output_file', output_file],
            input=text.encode('utf-8'),
            capture_output=True,
            check=True
        )
    except subprocess.CalledProcessError as e:
        print(f"    Error: {e.stderr.decode('utf-8')}", file=sys.stderr)
        sys.exit(1)

print(f"Generated {len(segments)} audio files to $output_dir")
EOF
}

# Create audio directory
mkdir -p "$AUDIO_DIR"

# Generate from all seeds or specific one
if [ $# -eq 0 ]; then
    # Generate from all seeds
    for seed_file in "$SEEDS_DIR"/*.yaml; do
        [ -f "$seed_file" ] || continue
        generate_from_seed "$seed_file"
    done
else
    # Generate from specific seed
    seed_file="$SEEDS_DIR/$1.yaml"
    if [ ! -f "$seed_file" ]; then
        echo "Error: Seed file not found: $seed_file"
        exit 1
    fi
    generate_from_seed "$seed_file"
fi

echo ""
echo "Audio generation complete!"
echo "Run evaluation with: python evaluate.py"
