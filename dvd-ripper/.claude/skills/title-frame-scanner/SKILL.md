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

3. **For each video file**, extract frames:

   ```bash
   mkdir -p /tmp/title-frames
   ffmpeg -y -ss <offset> -i "<video_path>" -vframes 10 -r 1 -q:v 2 /tmp/title-frames/%03d.jpg
   ```

   If ffmpeg fails (e.g., video is shorter than the offset), retry at 30 seconds, then 10 seconds. If all attempts fail, report "Could not extract frames" for that file and continue to the next.

4. **Read each extracted frame** using the Read tool. Look for frames containing episode title text — typically large, centered text on a solid or stylized background.

5. **Report findings** for each file:
   - The detected title text exactly as shown on screen
   - Which frame number contained the title card (e.g., frame 003 = offset + 3 seconds)
   - Confidence: high, medium, or low
   - Whether the title matches the filename (if the filename contains an episode name)

6. **Clean up** extracted frames:

   ```bash
   rm -rf /tmp/title-frames
   ```

## Batch output

When scanning a directory, report a summary table:

```
| File                    | Title Card Text         | Matches Filename |
|-------------------------|-------------------------|------------------|
| Avatar S01E01.mp4       | The Boy in the Iceberg  | -                |
| Avatar S01E02.mp4       | The Avatar Returns      | -                |
| Avatar S01E03.mp4       | The Southern Air Temple | -                |
```

## Notes

- Not all shows have title cards. Report "No title card detected" rather than guessing.
- Some title cards use stylized art or fantasy fonts. Note low confidence if text is ambiguous.
- For live-action content, title cards often appear later (90-120s). Suggest a later offset if the first scan finds nothing.
