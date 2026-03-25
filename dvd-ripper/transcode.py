"""Remote transcoding, file handling, and sync."""

import re
from pathlib import Path


def safe_name(name):
    """Replace non-filename-safe characters with underscores."""
    return re.sub(r'[^a-zA-Z0-9._-]', '_', name)


def find_mkv_for_title(output_dir, title_id, filename=None):
    """Find the MKV file produced by makemkvcon for a given title ID.

    When filename is provided (from makemkvcon --robot field 27), uses it
    directly to avoid ambiguity with leftover files from previous rips.
    Falls back to glob matching *_tNN.mkv when filename is not available.
    """
    if filename:
        path = Path(output_dir) / filename
        if path.exists():
            return path

    pattern = f"*_t{title_id:02d}.mkv"
    matches = list(Path(output_dir).glob(pattern))
    if matches:
        return matches[0]
    # Fallback: try without zero-padding
    pattern = f"*_t{title_id}.mkv"
    matches = list(Path(output_dir).glob(pattern))
    if matches:
        return matches[0]
    return None


def transcode_and_sync(conf, jobs, run_fn):
    """Transcode MKV files on the HandBrake host and sync to backup.

    Each job is a dict with: mkv_path, output_name, output_dir, remote_dir
    """
    handbrake_host = conf["HANDBRAKE_HOST"]
    backup_host = conf["BACKUP_HOST"]
    handbrake_work_dir = conf["HANDBRAKE_WORK_DIR"]
    handbrake_encoder = conf["HANDBRAKE_ENCODER"]
    handbrake_preset = conf["HANDBRAKE_PRESET"]
    handbrake_quality = conf["HANDBRAKE_QUALITY"]
    handbrake_audio_bitrate = conf["HANDBRAKE_AUDIO_BITRATE"]

    run_fn(f"ssh {handbrake_host} 'mkdir -p {handbrake_work_dir}'")

    for job in jobs:
        mkv_path = job["mkv_path"]
        output_name = job["output_name"]
        output_dir = job["output_dir"]
        remote_dir = job["remote_dir"]
        safe_mkv = safe_name(mkv_path.name)
        safe_out = safe_name(output_name)

        # Upload MKV to HandBrake host
        run_fn(f"scp -O '{mkv_path}' {handbrake_host}:{handbrake_work_dir}/{safe_mkv}")

        # Transcode
        run_fn(
            f"ssh {handbrake_host} '"
            f"HandBrakeCLI"
            f" -i {handbrake_work_dir}/{safe_mkv}"
            f" -o {handbrake_work_dir}/{safe_out}.mp4"
            f" -e {handbrake_encoder}"
            f" --encoder-preset {handbrake_preset}"
            f" -q {handbrake_quality}"
            f" -B {handbrake_audio_bitrate}'"
        )

        # Download transcoded file
        run_fn(f"scp -O {handbrake_host}:{handbrake_work_dir}/{safe_out}.mp4"
               f" '{output_dir}/{output_name}.mp4'")

        # Clean up remote
        run_fn(f"ssh {handbrake_host} 'rm"
               f" {handbrake_work_dir}/{safe_mkv}"
               f" {handbrake_work_dir}/{safe_out}.mp4'")

        # Clean up local MKV
        mkv_path.unlink()

        # Sync to backup host
        run_fn(f"rsync --mkpath -avz"
               f" '{output_dir}/{output_name}.mp4'"
               f" {backup_host}:'{remote_dir}/{output_name}.mp4'")
