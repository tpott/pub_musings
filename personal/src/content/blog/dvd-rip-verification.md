---
title: "DVD Rip Verification"
description: "I built an automatic, agentic verification of ripped DVDs for Jellyfin."
pubDate: 2026-04-21
draft: true
tags: ["skills", "claude", "Jellyfin", "makemkv", "ffmpeg", "openclaw"]
---

When did I start on dvd-ripper? Originally I had [openclaw](https://github.com/openclaw/openclaw) write the first bash version and it wasn't committed to git. I switched to Claude to cut costs and because I realized ripping movies had more nuance differences. I had setup Jellyfin to gain some media independence from Netflix and Disney. Netflix recently removed She-Ra (despite being originally marketed as a Netflix original).

A dozen movies later, I decided to try ripping a tv show, Avatar the Last Airbender. It turned out to be a lot more difficult because many files were of episode length, but some discs had extra features or combined episodes. I had to learn a little about dvd titles, segments, etc.

Like a lot of vibe coding, I ended up with more time spent testing, debugging and planning than in coding. I decided to write a [skill](/blog/skills-vs-mcp/) and automated the debugging. Then I added a [verification step](https://github.com/tpott/pub_musings/blob/74e4edb0ee0b3cb9c1bfa7953af30392385b31c1/dvd-ripper/verify.py) to my dvd ripping pipeline to trigger the skill automatically. It still doesn't work 100% of the time, but I have reduced how much debugging I need to do on my comp. I can usually run the suggested fix and it usually works. When it doesn't, the plan usually smells off, and I can choose to either (1) yolo, see if it works, or (2) step back into the debugging driver seat on my laptop. I like testing (1) so I can build systems to make it work in the future. But (2) is sometimes easier and more appealing.
