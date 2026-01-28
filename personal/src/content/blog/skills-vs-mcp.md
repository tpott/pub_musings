---
title: "Skills vs MCP Tools"
description: "A personal explortation through Claude Skills. What they can and cannot do."
pubDate: 2026-01-28
tags: ["skills", "mcp", "claude"]
draft: true
---

I was recently trying to better understand Claude Skills after reading 
[Anthropic's blog post](https://claude.com/blog/equipping-agents-for-the-real-world-with-agent-skills).
On first glance, they sound like packaged up [markdown](https://www.markdownguide.org/basic-syntax/).
This by itself is helpful for making prompts or prompt templates easier to share. But on a closer
review of the [Skills docs](https://code.claude.com/docs/en/skills) I saw

> Skills can bundle and run scripts in any language

And this was a :mindblown: moment for me.

I knew that writing a Skill for how to use common CLI's that I wanted claude to run, or run in
a certain way, would be useful. I wrote [one](https://github.com/tpott/pub_musings/blob/trunk/cc_plugins/skills/transcribe-srt/SKILL.md)
to call `whisper-cli` and `ffmpeg` for adding subtitles to videos I had laying around. But I
wanted to try something a little more challenging. I wanted to understand why Microsoft's
[debug-gym](http://github.com/microsoft/debug-gym) was setup more like an
[MCP server](https://modelcontextprotocol.io/docs/getting-started/intro) and less like a Skill.

Up until that point, whenever I was debugging some code with AI I would 1) run it, 2) copy the error
or logs, 3) paste back to Claude/ChatGPT/Gemini. But when I try debugging code myself, I enjoy
stepping through a debugger. With python, that usually means `pdb`. With Go, that usually means
`delve`.

I started vibe-coding up a new `pdb-debugger` skill with Claude code and realized that pdb
had a pretty strong dependency on having a [pseudo-tty](https://en.wikipedia.org/wiki/Pseudoterminal).
This made sense because whenever I was running `pdb` myself it was a very _interactive_ experience.
After some twists and turns I ended up with [debug_sesion.py](https://github.com/tpott/pub_musings/tree/trunk/cc_plugins/skills/python-automated-debugging/debug_session.py).
This little python script takes a list of pdb commands and runs them with the python program.
Claude code then learns about new program behavior and can iterate. The Skill markdown ensures
the coding agent knows how to do some stateless debugging.

Now this was about the time I realized that MCP servers enable _state_. And any time I want a
coding agent to interact with something that has _state_ then it most likely needs to be done
via an MCP Server.

Well, I hope that little journey was interesting to you. Until next time, cheers.
