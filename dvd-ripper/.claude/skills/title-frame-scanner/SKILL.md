---
name: title-frame-scanner
description: Extract frames from a video file and OCR the title card text using Claude vision. Use when identifying episode titles, verifying ripped episodes match their filenames, or checking what episode a video file contains.
argument-hint: <video-path-or-directory> [offset-seconds]
allowed-tools: Bash(ffmpeg *) Bash(mkdir *) Bash(rm *) Bash(ls *)
---

Scan video files for title card text by extracting frames with ffmpeg and reading them visually.

## Input

- `$0` is a video file path or a directory of `.mp4` files to scan in batch.
- `$1` is an optional frame offset in seconds (default: read `VERIFY_FRAME_OFFSET` from `rip.conf` in the project directory, or fall back to 75). This is the timestamp where title cards typically appear — tuned for animated shows where the card follows a cold open.

## Steps

1. **Resolve the offset.** If `$1` is provided, use it. Otherwise, check for `VERIFY_FRAME_OFFSET` in `rip.conf` (parse the `KEY="value"` format). Fall back to 75 seconds.

2. **Determine targets.** If `$0` is a directory, collect all `.mp4` files sorted alphabetically. If it's a single file, use just that file.

3. **For each video file**, extract frames at 1 frame per 2 seconds covering a
   ~2-minute window (large enough to span variable cold-open lengths):

   ```bash
   mkdir -p /tmp/title-frames
   ffmpeg -y -ss <offset> -i "<video_path>" -vf fps=1/2 -frames:v 60 -q:v 2 /tmp/title-frames/%03d.jpg
   ```

   This produces up to 60 frames spaced 2 seconds apart, starting at `<offset>`.
   Frame N corresponds to timestamp `<offset> + (N - 1) * 2` seconds in the source file.

   If ffmpeg fails (e.g., video is shorter than the offset), retry at 30 seconds, then 10 seconds. If all attempts fail, report "Could not extract frames" for that file and continue to the next.

4. **Read each extracted frame** using the Read tool. Look for frames containing episode title text — typically large, centered text on a solid or stylized background. You don't need to read every frame if you find the title card early; stop once you've identified it.

5. **Report findings** for each file:
   - The detected title text exactly as shown on screen
   - The timestamp in seconds where the title card appeared, computed as
     `<offset> + (frame_number - 1) * 2`. Report in both seconds (e.g. `110s`) and mm:ss (e.g. `01:50`).
   - Confidence: high, medium, or low
   - Whether the title matches the filename (if the filename contains an episode name)

6. **Clean up** extracted frames:

   ```bash
   rm -rf /tmp/title-frames
   ```

## Batch output

When scanning a directory, report a summary table:

```
| File                    | Title Card Text         | Timestamp | Matches Filename |
|-------------------------|-------------------------|-----------|------------------|
| Avatar S01E01.mp4       | The Boy in the Iceberg  | 01:15     | -                |
| Avatar S01E02.mp4       | The Avatar Returns      | 01:15     | -                |
| Avatar S01E03.mp4       | The Southern Air Temple | 01:15     | -                |
```

## Notes

- Not all shows have title cards. Report "No title card detected" rather than guessing.
- Some title cards use stylized art or fantasy fonts. Note low confidence if text is ambiguous.
- For live-action content, title cards often appear later (90-120s). Suggest a later offset if the first scan finds nothing.
