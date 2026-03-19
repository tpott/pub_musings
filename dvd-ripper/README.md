# dvd-ripper

Automated DVD/Blu-ray ripping pipeline. Rips discs with MakeMKV, transcodes with HandBrake on a remote Mac, and syncs the result to a Jellyfin media server.

This project is 10x better with a self-hosted Jellyfin server and Jellyfin on a [Smart TV](https://github.com/Georift/install-jellyfin-tizen).

## How it works

1. A disc is inserted into `/dev/sr0`
2. udev triggers `rip-udev.sh`, which launches `rip.py` as a systemd user service
3. `rip.py` rips with MakeMKV, SCPs the largest title to the HandBrake host for transcoding, syncs the final `.mp4` to the backup host, and ejects the disc

## Hosts

| Alias | Role | Scripts |
|-------|------|---------|
| **Ripper** (your Linux box) | Rips discs, orchestrates the pipeline | `rip.py`, `rip-udev.sh`, `99-dvd-rip.rules` |
| **HandBrake host** (e.g. `qemuhost`) | Efficiently transcodes MKV to MP4 via `HandBrakeCLI` | `handbrake-receiver` |
| **Backup host** (e.g. `mini`) | Jellyfin media storage | `backup-receiver` |

## Configuration

Copy `rip.conf.example` to `rip.conf` and edit it:

```sh
cp rip.conf.example rip.conf
```

`rip.conf` is gitignored. All personal paths, hostnames, and encoding settings live there — see the example file for documentation of each variable.

## SSH config

The scripts use SSH host aliases defined in `~/.ssh/config` on the ripper. The host aliases must match `HANDBRAKE_HOST` and `BACKUP_HOST` in `rip.conf`. Each host uses a dedicated key so that the receiver scripts can be set as forced commands.

Example `~/.ssh/config` entries:

```
Host qemuhost
    HostName 192.168.1.100
    User youruser
    IdentityFile ~/.ssh/id_handbrake

Host mini
    HostName 192.168.1.101
    User youruser
    IdentityFile ~/.ssh/id_mini_backup
```

Generate the keys:

```sh
ssh-keygen -t ed25519 -f ~/.ssh/id_handbrake -C "dvd-ripper handbrake"
ssh-keygen -t ed25519 -f ~/.ssh/id_mini_backup -C "dvd-ripper backup"
```

## Receiver scripts

`handbrake-receiver` and `backup-receiver` are SSH forced-command scripts that run on the remote hosts. They restrict what the ripper can execute over SSH, so a compromised key can only perform ripping-related operations.

### HandBrake host setup

1. Copy the script to the HandBrake host:
   ```sh
   scp handbrake-receiver qemuhost:~/.ssh/handbrake-receiver
   ssh qemuhost "chmod +x ~/.ssh/handbrake-receiver"
   ```

2. Edit `WORK_DIR` at the top of the script on the remote host to match `HANDBRAKE_WORK_DIR` in your `rip.conf`.

3. Add the ripper's public key to `~/.ssh/authorized_keys` on the HandBrake host with a forced command:
   ```
   command="~/.ssh/handbrake-receiver",no-port-forwarding,no-agent-forwarding ssh-ed25519 AAAA... dvd-ripper handbrake
   ```

### Backup host setup

1. Copy the script to the backup host:
   ```sh
   scp backup-receiver mini:~/.ssh/backup-receiver
   ssh mini "chmod +x ~/.ssh/backup-receiver"
   ```

2. Edit `ALLOWED_PATH` at the top of the script on the remote host to match the parent of `BACKUP_DEST` in your `rip.conf`.

3. Add the ripper's public key to `~/.ssh/authorized_keys` on the backup host with a forced command:
   ```
   command="~/.ssh/backup-receiver",no-port-forwarding,no-agent-forwarding ssh-ed25519 AAAA... dvd-ripper backup
   ```

## udev rules

Update the path in `99-dvd-rip.rules` to match `RIP_DIR` in your `rip.conf`, then install:

```sh
sudo cp 99-dvd-rip.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

It triggers on both DVD (`ID_CDROM_MEDIA_DVD`) and Blu-ray (`ID_CDROM_MEDIA_BD`) disc insertion.

## Manual trigger

```sh
# Basic — uses disc label as the movie name
./rip.py

# With a custom Jellyfin-friendly name
MOVIE_NAME="The Lion King (2019)" ./rip.py

# As a systemd service (survives terminal close, logs to journald)
systemd-run --user --unit="dvd-rip-$(date +%s)" "$RIP_DIR/rip.py"
```

## Debugging

Check the rip service logs:

```sh
# Follow logs for all dvd-rip service instances
journalctl --user -u 'dvd-rip-*' -f

# Recent rip logs
journalctl --user -u 'dvd-rip-*' --no-pager -n 100

# udev daemon logs
journalctl -u systemd-udevd --no-pager -n 50

# Search for disc-related events
journalctl --no-pager -n 50 -g 'dvd-rip\|sr0'
```

Check udev environment variables for the drive (useful for verifying rule matches):

```sh
udevadm info --query=env --name=/dev/sr0 | grep ID_CDROM_MEDIA
```

Check receiver logs on the remote hosts:

```sh
# On the HandBrake host
cat /tmp/handbrake-receiver.log

# On the backup host
cat /tmp/backup-receiver.log
```

## Testing

```sh
python3 -m unittest discover tests
```

## TV show support

TV discs are auto-detected by analyzing title durations from `makemkvcon`. If 3+ titles cluster in the 15–65 minute range with similar durations, the disc is treated as TV. Episodes are deduplicated by preferring single-segment titles over bumper-prepended variants.

Output follows Jellyfin's expected structure:

```
TV/<Show Name>/Season 01/<Show Name> S01E01.mp4
TV/<Show Name>/Season 01/<Show Name> S01E02.mp4
```

The show name, season, and disc number are parsed from the disc label (e.g. `Avatar_Book_1_Disc_1` → show "Avatar", season 1, disc 1). Override with environment variables when the label isn't sufficient:

```sh
SHOW_NAME="Avatar The Last Airbender" SEASON=1 EPISODE_START=1 ./rip.py
```

Add `BACKUP_DEST_TV` to `rip.conf` for a separate Jellyfin TV library path. Falls back to `BACKUP_DEST` if not set.

## Future improvements

- **TMDb integration** — look up disc labels against [The Movie Database](https://www.themoviedb.org/) API to auto-detect proper titles, years, and movie-vs-TV classification. Replaces the manual `MOVIE_NAME` override.
- **Whisper subtitles** — use `whisper-cli` to generate `.srt` subtitle files from the audio track.
- **Extras handling** — identify and organize bonus features into Jellyfin-recognized subfolders (`featurettes/`, `behind the scenes/`, `deleted scenes/`).
