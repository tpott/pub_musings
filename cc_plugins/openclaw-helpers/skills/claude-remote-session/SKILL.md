---
name: claude-remote-session
description: Use when user asks for "a claude session", "give me a claude session", wants to connect from claude.ai/code or the Claude mobile app, or asks to start, restart, or check Claude Code remote control on this machine.
---

# Claude Remote Session

Goal: healthy `claude remote-control` in tmux session `claude-remote-control` → return claude.ai/code URL. No local port → read health from pane, not network. Old `Connected` = prime wedge suspect → prefer restart over trust.

## Settings
- tmux session: `claude-remote-control`
- launch dir: none default (run in session's current shell). Request names dir ("...in ~/openclaw") → cd there first.
- fresh window: 120s

## 1. State (read-only)
```bash
PID=$(pgrep -af remote-control | awk '$2=="claude"&&$3=="remote-control"{print $1}')
AGE=$([ -n "$PID" ] && ps -o etimes= -p "$PID" | tr -d ' ')
PANE=$(tmux capture-pane -t claude-remote-control -p 2>/dev/null)
```
Responding = live status block has `Connected`. Strip capture-pane blank padding first, else line slips out of window:
`printf '%s\n' "$PANE" | grep -vE '^[[:space:]]*$' | tail -8 | grep -q Connected`
Else (reconnecting, `Error:`, `Session failed:`, shell prompt) = not responding.

## 2. Decide
| State | Action |
|---|---|
| no `$PID` | start (3) |
| `$PID`, not responding | restart (3) |
| `$PID`, responding, `AGE` ≥ 120 | restart (3) |
| `$PID`, responding, `AGE` < 120 | ask user: "A fresh session already exists (started ${AGE}s ago). Restart it?" yes → restart; no → step 4 |

## 3. Start / restart
DIR = dir from request, else empty. No `--spawn` flags.
```bash
DIR=""   # set to requested dir (e.g. ~/openclaw), else leave empty
tmux has-session -t claude-remote-control 2>/dev/null \
  || tmux new-session -d -s claude-remote-control
[ -n "$PID" ] && { tmux send-keys -t claude-remote-control C-c; sleep 2; }
tmux send-keys -t claude-remote-control "${DIR:+cd $DIR && }claude remote-control" Enter
```
Pane shows `Choose [1/2]` (spawn prompt) → accept default:
```bash
tmux send-keys -t claude-remote-control Enter
```

## 4. Wait Connected → return URL
Poll ≤30s:
```bash
for i in $(seq 1 15); do
  PANE=$(tmux capture-pane -t claude-remote-control -p)
  printf '%s\n' "$PANE" | grep -vE '^[[:space:]]*$' | tail -8 | grep -q Connected && break
  sleep 2
done
printf '%s\n' "$PANE" | grep -oE 'https://claude\.ai/code\?environment=env_[A-Za-z0-9]+' | tail -1
```
Return URL as text + one-line status (e.g. `Connected · 2/32 · started 8s ago`). No `Connected` → return last ~8 pane lines so user sees error.

## Notes
- Session `claude-remote-control` = only managed home. Server outside it → mention PID/cwd, don't touch.
- Use process age (`ps -o etimes=`), never tmux session age.
- Restart preserves env → active sessions reconnect.
