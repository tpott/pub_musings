# test-agents

I really want to test https://code.claude.com/docs/en/sub-agents#resume-subagents

```
CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 claude --plugin-dir test-agents

> @"test-agents:happy (agent)" how many plugins are in this project?

...

> @"test-agents:happy (agent)" which is your favorite?
```

`export CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` is necessary. https://github.com/anthropics/claude-code/issues/38183 describes the issue in more detail

Docs:
* https://code.claude.com/docs/en/plugins#test-your-plugins-locally
* https://code.claude.com/docs/en/plugins-reference#agents
* https://code.claude.com/docs/en/sub-agents
