---
title: "Building This Blog"
description: "How I went from an idea to a working blog in about 12 hours, with help from Claude."
pubDate: 2026-01-13
tags: ["introduction", "astro", "ai"]
---

Welcome to my blog! I hope you find it interesting and enjoy reading a post or two. I've been delaying this post and getting this site off the ground for too long now. I've felt nervous and unsure about why I'm writing a blog, so I'm hoping to answer that a bit here. I am hoping that this blog will force me to think clearly about what I'm writing. In the process of building this blog I have already learned about new technical frameworks ([Astro](https://astro.build/), [Caddy](https://caddyserver.com/)). Finally, with the tech industry changing as fast as it has been, having a space to document what I'm learning felt like a good investment.

So a couple nights ago I sat down and started planning. I chatted with Claude about how to architect the site -- what static site generator to use, how to handle deploys and testing, where to host it. We iterated on the plan until it felt right.

Then I told Claude to implement the plan. It did not quite go as I expected. I needed dependencies on my laptop for development ([sops](https://getsops.io/), [age](https://github.com/FiloSottile/age)).. I had to install dependencies in the VM I was going to use for deploys. I had to create accounts and get API keys set up. I had to debug filesystem permission issues (`caddy` created a separate user and couldn't read the static files it needed) and migrate old cloudflared tunnels from a previous side project that had been running on the same VM.

About 12 (wall-)hours later (kids wanted to play in the middle somewhere), I had a working blog! Running `npm run dev` on my laptop gets hot reloads. I can visit the blog in my browser and see new posts show up as soon as I save the file. And for me, the killer feature is auto-deploys trigger from Github webhooks when I push to trunk. I liked how simple it was to add new markdown files for blog posts in Astro. If you're curious, the source code is available on [GitHub](https://github.com/tpott/pub_musings/tree/trunk/personal).

I plan to write about software engineering, tools and technologies I'm exploring, and updates on personal projects. I've delayed these first couple of posts too long, but I hope to get in a rhythm of weekly posts going forwards. Lets check back in 2027 and see how I did?

Until next time, cheers.

--Trevor
