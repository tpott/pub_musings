# FEEDBACK

This is Human provided feedback for Ralph.

## specs/subtitler.md

IMPORTANT our subtitler mission has been updated to be more focused. I want you to build the best website possible for generating subtitles for music videos or learning videos in different languages. In the case of popular songs, we can easily find lyrics online. When a user uploads the lyrics for a video they already uploaded then I want you to figure out how to efficiently fix the transcription. File a task to do this if its too big to do right now.

Regarding the "Accuracy Evaluation Plan", what other metrics should we incorporate? When I'm looking at the subtitles produced by large-v3-turbo I see sentences many seconds before singing (speech) starts. I want some way to measure how well "aligned" (my word) the subtitles are with the actual speech. 

How can we setup a "clean room" to evaluate different open source models? So not even thinking about competitors. I want to compare large-v3 vs large-v3-turbo vs medium. I want to know 1) how long they take to process files, and 2) how accurate they are. WER sounds like one way to evaluate accuracy. I want something to assess timing accuracy (alignment). You may think of other accuracy measures. The clean room evaluation should be something I can run on my laptop with files that don't need to be committed. You should figure out what files you can legally test and iterate on. You need to test the clean room.

I think there are some open source text to speech models that we can potentially leverage? And instead of committing large audio files to the repo, we should be able to commit the "seed" data we use to produce the audio files. There may be some complexity in ensuring consistent outputs, but idk maybe TTS models can be controlled better than LLMs.

Regarding the "Design" bit, can you add a task to visit these websites with playwright? I want you to take a screenshot, record the HAR and also the HAR content. I want you to study what makes these websites so easy to navigate. I want you to produce a design template that we can apply to our eventual website landing page.

Re: all the "see task N" notes, these should be markdown links. Referencing TASKS.jsonl here is not a good reference. If the task has a spec, then reference the spec. 

Re: "Competitive Advantages We Can Offer" most of this is wrong. You are iterating on a self hosted model, but I eventually want to deploy your code so we can offer a website that people can sign into and use. If most competitors are charing per-minute fees then that's likely what customers expect.

File tasks for anything that sounds complex or has a lot of ambiguity. You shouldn't be afraid to add new tasks to TASKS.jsonl

## backend/README.md

The backend/README.md file shouldn't have any references to qemu. This should probably live in specs/deployment.md.

## specs/deployment.md

Why does the qemu-system need to have hostfwd ports? It should only need SSH.

Also, this spec should mention that cloudflared is running inside the qemu VM and routing traffic to Caddy via a cloudflare tunnel.

Can you add a comment explaining how the guest qemu VM (ubuntu) is going to access the host (10.0.2.2)?

Also, Mac M1 runs ARM so qemu-system-x86_64 should be qemu-system-aarch64

## specs/metal-moltenvk.md

I think you misunderstood my intention. Based on reading https://github.com/KhronosGroup/MoltenVK , you should clone it and study it too, that we would compile a custom qemu binary. My understanding is that qemu would then be able to expose a Vulkan compatible GPU to guest VMs? Try to break this down to smaller pieces and figure out how to string them together. At the end of the day, we need to be able to develop applications in the guest VM that leverage GPU APIs. whisper.cpp is able to leverage GPUs, and you can see if it is when running whisper-cli. So you could write a test plan for how I would help you with this.

## specs/totp.md

Can we generate our own QR codes? I'm sure there's a golang library for generating qr codes. I really don't like making changes to our frontend and needing to allow cross-origin content from qrserver.com.

## When done

When you're done addressing all the FEEDBACK in this file then please delete it.
