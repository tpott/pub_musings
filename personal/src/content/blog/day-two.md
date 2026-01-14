---
title: "Day Two"
description: "I can't believe its been almost 12 hours since I started down this path"
pubDate: 2026-01-13
tags: ["time"]
---

I started on my journey making what I thought would be a simple personal website a couple nights ago. I chatted with claude about how to architect the site, how to handle deploys and testing. I iterated on the plan.

Then today I decided to try implementing the plan. It did not quite go according to the plan. I needed a lot of dependencies on my laptop, where I was doing development and testing. I had to install dependencies in the VM I was going to be doing the deploys in. I had to get accounts created and API keys setup. I had to debug filesystem permission issues and migrate old tunnels (the VM was previously used for some other side project...).

Now the day is almost over and I think I got the blog working. Running `npm run dev` gets hot reloads. I'm able to visit [http://localhost:4321/blog](http://localhost:4321/blog) in my browser and see the new post show up as soon as I save the file. I'm writing this partially to celebrate. Partially to test webhook consumption triggering auto deploys "in prod". Hopefully it works. But I'm just excited about how much new tech I was able to leverage today.
