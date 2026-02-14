#!/usr/bin/env python3
"""
Upload a media set (photo + optional audio/video) to a concept via the admin API.

Secrets resolution order:
  1. Environment variables (PROD_HOST, API_SESSION_ID)
  2. .env file in the project root (KEY=VALUE format, gitignored)

Usage:
  python3 scripts/upload-media.py cat photo.jpg
  python3 scripts/upload-media.py cat photo.jpg --audio meow.mp3
  python3 scripts/upload-media.py horse photo.png --audio neigh.wav --video gallop.mp4
  python3 scripts/upload-media.py cat photo.jpg --audio meow.mp3 --yes

Exit codes:
  0 - Media uploaded successfully
  2 - Error (auth failure, network, validation, missing config)
"""

import argparse
import json
import mimetypes
import os
import shutil
import struct
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent.parent
ENV_FILE = PROJECT_DIR / ".env"

REQUIRED_KEYS = ("PROD_HOST", "API_SESSION_ID")

# Constraints
MAX_AUDIO_DURATION = 5.0   # seconds
MAX_VIDEO_DURATION = 10.0  # seconds
MAX_PHOTO_SIZE = 10 * 1024 * 1024   # 10 MB
MAX_AUDIO_SIZE = 5 * 1024 * 1024    # 5 MB
MAX_VIDEO_SIZE = 50 * 1024 * 1024   # 50 MB
MIN_RESOLUTION = (320, 240)

ALLOWED_PHOTO_TYPES = {"image/jpeg", "image/png", "image/webp", "image/gif"}
ALLOWED_AUDIO_TYPES = {"audio/mpeg", "audio/wav", "audio/ogg"}
ALLOWED_VIDEO_TYPES = {"video/mp4", "video/webm"}


def load_env_file(path: Path) -> dict[str, str]:
    """Parse a KEY=VALUE file, ignoring comments and blank lines."""
    env: dict[str, str] = {}
    if not path.is_file():
        return env
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, _, value = line.partition("=")
        value = value.strip().strip("'\"")
        env[key.strip()] = value
    return env


def resolve_secrets() -> dict[str, str]:
    """Resolve secrets from env vars and .env file."""
    secrets = load_env_file(ENV_FILE)
    for key in REQUIRED_KEYS:
        val = os.environ.get(key)
        if val:
            secrets[key] = val
    return secrets


def human_size(size: int) -> str:
    """Format a byte count as a human-readable string."""
    for unit in ("B", "KB", "MB", "GB"):
        if abs(size) < 1024:
            return f"{size:.1f} {unit}"
        size /= 1024
    return f"{size:.1f} TB"


def get_ffprobe_duration(filepath: str) -> float | None:
    """Get media duration in seconds via ffprobe. Returns None if unavailable."""
    if not shutil.which("ffprobe"):
        return None
    try:
        result = subprocess.run(
            ["ffprobe", "-v", "quiet", "-show_entries", "format=duration",
             "-of", "csv=p=0", filepath],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode == 0 and result.stdout.strip():
            return float(result.stdout.strip())
    except (subprocess.TimeoutExpired, ValueError):
        pass
    return None


def get_ffprobe_resolution(filepath: str) -> tuple[int, int] | None:
    """Get video resolution via ffprobe. Returns (width, height) or None."""
    if not shutil.which("ffprobe"):
        return None
    try:
        result = subprocess.run(
            ["ffprobe", "-v", "quiet", "-select_streams", "v:0",
             "-show_entries", "stream=width,height",
             "-of", "csv=p=0:s=x", filepath],
            capture_output=True, text=True, timeout=10,
        )
        if result.returncode == 0 and "x" in result.stdout:
            w, h = result.stdout.strip().split("x")
            return int(w), int(h)
    except (subprocess.TimeoutExpired, ValueError):
        pass
    return None


def get_jpeg_dimensions(filepath: str) -> tuple[int, int] | None:
    """Read JPEG dimensions from file headers (no dependencies)."""
    try:
        with open(filepath, "rb") as f:
            data = f.read(2)
            if data != b"\xff\xd8":
                return None
            while True:
                marker = f.read(2)
                if len(marker) < 2:
                    return None
                if marker[0] != 0xFF:
                    return None
                # SOF markers
                if marker[1] in (0xC0, 0xC1, 0xC2):
                    f.read(3)  # length + precision
                    height = struct.unpack(">H", f.read(2))[0]
                    width = struct.unpack(">H", f.read(2))[0]
                    return (width, height)
                # Skip other markers
                length_data = f.read(2)
                if len(length_data) < 2:
                    return None
                length = struct.unpack(">H", length_data)[0]
                f.seek(length - 2, 1)
    except (OSError, struct.error):
        return None


def get_png_dimensions(filepath: str) -> tuple[int, int] | None:
    """Read PNG dimensions from IHDR chunk."""
    try:
        with open(filepath, "rb") as f:
            sig = f.read(8)
            if sig != b"\x89PNG\r\n\x1a\n":
                return None
            f.read(4)  # chunk length
            chunk_type = f.read(4)
            if chunk_type != b"IHDR":
                return None
            width = struct.unpack(">I", f.read(4))[0]
            height = struct.unpack(">I", f.read(4))[0]
            return (width, height)
    except (OSError, struct.error):
        return None


def get_image_dimensions(filepath: str) -> tuple[int, int] | None:
    """Try to get image dimensions using file headers or ffprobe."""
    mime = mimetypes.guess_type(filepath)[0] or ""
    if "jpeg" in mime or "jpg" in mime:
        dims = get_jpeg_dimensions(filepath)
        if dims:
            return dims
    elif "png" in mime:
        dims = get_png_dimensions(filepath)
        if dims:
            return dims

    # Fallback to ffprobe for other formats
    return get_ffprobe_resolution(filepath)


def print_file_metadata(label: str, filepath: str, mime_type: str) -> list[str]:
    """Print metadata about a media file. Returns list of warnings."""
    warnings: list[str] = []
    size = os.path.getsize(filepath)
    print(f"  {label}:")
    print(f"    Path: {filepath}")
    print(f"    Type: {mime_type}")
    print(f"    Size: {human_size(size)}")

    if "image" in mime_type:
        dims = get_image_dimensions(filepath)
        if dims:
            print(f"    Dimensions: {dims[0]}x{dims[1]}")
            if dims[0] < MIN_RESOLUTION[0] or dims[1] < MIN_RESOLUTION[1]:
                warnings.append(f"Photo resolution {dims[0]}x{dims[1]} below minimum {MIN_RESOLUTION[0]}x{MIN_RESOLUTION[1]}")
        if size > MAX_PHOTO_SIZE:
            warnings.append(f"Photo size {human_size(size)} exceeds {human_size(MAX_PHOTO_SIZE)} limit")

    elif "audio" in mime_type:
        duration = get_ffprobe_duration(filepath)
        if duration is not None:
            print(f"    Duration: {duration:.1f}s")
            if duration > MAX_AUDIO_DURATION:
                warnings.append(f"Audio duration {duration:.1f}s exceeds {MAX_AUDIO_DURATION}s limit")
        else:
            print("    Duration: (ffprobe not available)")
        if size > MAX_AUDIO_SIZE:
            warnings.append(f"Audio size {human_size(size)} exceeds {human_size(MAX_AUDIO_SIZE)} limit")

    elif "video" in mime_type:
        duration = get_ffprobe_duration(filepath)
        if duration is not None:
            print(f"    Duration: {duration:.1f}s")
            if duration > MAX_VIDEO_DURATION:
                warnings.append(f"Video duration {duration:.1f}s exceeds {MAX_VIDEO_DURATION}s limit")
        else:
            print("    Duration: (ffprobe not available)")
        dims = get_ffprobe_resolution(filepath)
        if dims:
            print(f"    Resolution: {dims[0]}x{dims[1]}")
            if dims[0] < MIN_RESOLUTION[0] or dims[1] < MIN_RESOLUTION[1]:
                warnings.append(f"Video resolution {dims[0]}x{dims[1]} below minimum {MIN_RESOLUTION[0]}x{MIN_RESOLUTION[1]}")
        if size > MAX_VIDEO_SIZE:
            warnings.append(f"Video size {human_size(size)} exceeds {human_size(MAX_VIDEO_SIZE)} limit")

    return warnings


def upload_media(host: str, session_id: str, concept_id: str,
                 photo_path: str, audio_path: str | None, video_path: str | None) -> int:
    """Upload media files via multipart POST. Returns exit code."""
    # Build multipart body manually (no requests dependency)
    boundary = "----PeekabooUpload" + os.urandom(8).hex()
    body_parts: list[bytes] = []

    # Add concept_id field
    body_parts.append(f"--{boundary}\r\n".encode())
    body_parts.append(b'Content-Disposition: form-data; name="concept_id"\r\n\r\n')
    body_parts.append(f"{concept_id}\r\n".encode())

    # Add files
    def add_file(field: str, filepath: str, mime: str) -> None:
        filename = os.path.basename(filepath)
        body_parts.append(f"--{boundary}\r\n".encode())
        body_parts.append(f'Content-Disposition: form-data; name="{field}"; filename="{filename}"\r\n'.encode())
        body_parts.append(f"Content-Type: {mime}\r\n\r\n".encode())
        with open(filepath, "rb") as f:
            body_parts.append(f.read())
        body_parts.append(b"\r\n")

    photo_mime = mimetypes.guess_type(photo_path)[0] or "image/jpeg"
    add_file("photo", photo_path, photo_mime)

    if audio_path:
        audio_mime = mimetypes.guess_type(audio_path)[0] or "audio/mpeg"
        add_file("audio", audio_path, audio_mime)

    if video_path:
        video_mime = mimetypes.guess_type(video_path)[0] or "video/mp4"
        add_file("video", video_path, video_mime)

    body_parts.append(f"--{boundary}--\r\n".encode())
    body = b"".join(body_parts)

    url = f"{host}/api/admin/media"
    req = urllib.request.Request(url, data=body, method="POST")
    req.add_header("Authorization", f"Bearer {session_id}")
    req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
    req.add_header("User-Agent", "peekaboo-admin/1.0")

    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            print(f"\nUploaded successfully!")
            print(f"  Concept: {data.get('concept_id')}")
            print(f"  Set: {data.get('set')}")
            print(f"  Photo: {data.get('photo_path')}")
            if data.get("audio_path"):
                print(f"  Audio: {data.get('audio_path')}")
            if data.get("video_path"):
                print(f"  Video: {data.get('video_path')}")
            return 0
    except urllib.error.HTTPError as e:
        body_text = ""
        if e.fp:
            body_text = e.fp.read().decode("utf-8", errors="replace")

        if e.code == 401:
            print("Error: Authentication failed (401). Check API_SESSION_ID.", file=sys.stderr)
            return 2
        if e.code == 403:
            print("Error: Forbidden (403). User is not in TRUSTED_USERS.", file=sys.stderr)
            return 2

        try:
            data = json.loads(body_text)
            print(f"Error: {data.get('error', f'HTTP {e.code}')}", file=sys.stderr)
        except json.JSONDecodeError:
            print(f"Error: API returned HTTP {e.code}: {body_text}", file=sys.stderr)
        return 2
    except urllib.error.URLError as e:
        print(f"Error: Network error: {e.reason}", file=sys.stderr)
        return 2


def main(argv: list[str] | None = None) -> int:
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("concept_id", help="Target concept ID (e.g. 'cat', 'horse')")
    parser.add_argument("photo", help="Photo file path (required)")
    parser.add_argument("--audio", help="Audio file path (optional)")
    parser.add_argument("--video", help="Video file path (optional)")
    parser.add_argument("--yes", "-y", action="store_true",
                        help="Skip confirmation prompt")
    args = parser.parse_args(argv)

    secrets = resolve_secrets()
    missing = [k for k in REQUIRED_KEYS if k not in secrets or not secrets[k]]
    if missing:
        print(f"Error: Missing required secrets: {', '.join(missing)}", file=sys.stderr)
        print("Provide them via environment variables or .env file", file=sys.stderr)
        return 2

    # Validate files exist
    for label, path in [("Photo", args.photo), ("Audio", args.audio), ("Video", args.video)]:
        if path and not os.path.isfile(path):
            print(f"Error: {label} file not found: {path}", file=sys.stderr)
            return 2

    # Validate MIME types
    photo_mime = mimetypes.guess_type(args.photo)[0] or ""
    if photo_mime not in ALLOWED_PHOTO_TYPES:
        print(f"Error: Unrecognized photo type '{photo_mime}' for {args.photo}", file=sys.stderr)
        print(f"  Allowed: {', '.join(sorted(ALLOWED_PHOTO_TYPES))}", file=sys.stderr)
        return 2

    if args.audio:
        audio_mime = mimetypes.guess_type(args.audio)[0] or ""
        if audio_mime not in ALLOWED_AUDIO_TYPES:
            print(f"Error: Unrecognized audio type '{audio_mime}' for {args.audio}", file=sys.stderr)
            return 2

    if args.video:
        video_mime = mimetypes.guess_type(args.video)[0] or ""
        if video_mime not in ALLOWED_VIDEO_TYPES:
            print(f"Error: Unrecognized video type '{video_mime}' for {args.video}", file=sys.stderr)
            return 2

    # Print metadata
    print(f"Upload media for concept '{args.concept_id}':\n")
    all_warnings: list[str] = []

    all_warnings.extend(print_file_metadata("Photo", args.photo, photo_mime))
    if args.audio:
        audio_mime = mimetypes.guess_type(args.audio)[0] or "audio/mpeg"
        all_warnings.extend(print_file_metadata("Audio", args.audio, audio_mime))
    if args.video:
        video_mime = mimetypes.guess_type(args.video)[0] or "video/mp4"
        all_warnings.extend(print_file_metadata("Video", args.video, video_mime))

    if all_warnings:
        print(f"\nWarnings:")
        for w in all_warnings:
            print(f"  - {w}")

    # Confirm unless --yes
    if not args.yes:
        try:
            response = input("\nProceed with upload? [y/N] ")
            if response.lower() not in ("y", "yes"):
                print("Upload cancelled.")
                return 0
        except (EOFError, KeyboardInterrupt):
            print("\nUpload cancelled.")
            return 0

    host = secrets["PROD_HOST"].rstrip("/")
    session_id = secrets["API_SESSION_ID"]
    return upload_media(host, session_id, args.concept_id,
                        args.photo, args.audio, args.video)


if __name__ == "__main__":
    sys.exit(main())
