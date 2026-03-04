# Escalation Protocol

When stuck after multiple attempts, escalate rather than thrash.

## Signs You Should Escalate

- Same error after 3+ different fix attempts
- Unclear requirements (spec is ambiguous)
- Need access/permissions you don't have
- Architectural question with no clear answer

## Escalation Actions

1. **Document the blocker** in HELP.md:
   ```markdown
   ## Blocked: {brief description}

   **Task:** {task from TASKS.jsonl}

   **Attempts:**
   1. Tried X → failed because Y
   2. Tried A → failed because B

   **Need:** {what would unblock you}
   ```

2. **Mark task** as `blocked` in TASKS.jsonl (add `blocked_reason` field)

3. **Pick different task** — don't thrash on the same problem

4. **Human will address** HELP.md and update you
