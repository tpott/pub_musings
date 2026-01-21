# subtitler

Lets design a fun website for discovering subtitles for our content. 

## Core Function

When a user navigates to /upload or uses a drag-and-drop they should see real time
excerpts of their video overlayed instead of a spinner. As a user, I want to upload
my video, see the transcribed subtitles, and I want to be impressed by the accuracy
of the subtitles. I also want to have fun improving the alignment of subtitles, so
they start and end at the correct time or easily changing a single word.

If I upload video that I know the full transcript for, then I want to be able to paste
that in after the video processing has already started/finished. I want the server to
be smart and match the words it transcribed correctly and replace the words it missed.
I want the server to be smart about how it adjusts or aligns the subtitles with the
tokens that whisper-server detects.

As a user, I should be able to upload up to two videos before requiring registration.
Unregistered users should have their videos persisted for up to 48 hours before being
deleted. Registered users should have their videos/audio persisted for up to 90 days.

As a user, I should be able to view the subtitles for my uploads. They should be easy
to read. I should also be able to download .srt files, or other similar file formats.
I should also be able to add subtitles back to the video file. Or add an option in my
user settings that automatically adds subtitles back to my video so that I can download
my video with subtitles included in it.

## Non-Functional Requirements

Everything about this application needs to be fast. As a user, I should get very fast
visual feedback that my upload is happening. I should get interesting information that
indicates text from my upload is getting transcribed. I should have the most accurate
transcription possible.

TODO write a plan for evaluating competitors on accuracy and speed.

## Design

The initial website should have a simple aesthetic. Consider https://www.anthropic.com/
or https://ampcode.com/ for inspiration. Over time, we should experiment with other
designs and see what resonates more with customers.

## Architecture

The frontend should be in Astro. It should have a proxy config to pass /api/* requests to
the backend. The frontend should forward all console.log, console.warning, etc messages
to the backend to make debugging easier when run with `npm run dev`. That should not happen
in production.

The backend should be written in Go. The backend should either use a remote whisper-server
or it should run whisper-server itself. Tests should use a smaller, faster model.
Production should use a larger, more accurate model. Cost vs speed tradeoff is still TBD.
Default to using the fastest whisper model you can. Allow for overriding the whisper model
via an env var. Allow for overriding the whisper server so we can run whisper server on
a baremetal Mac Mini. TODO document how to passthrough the whisper server IP:port into
the qemu VM and what the env var the backend needs to use that for transcoding. If
running the whisper server process, then pipe all whisper-server logs to the backend logs
so its easier for debugging. Make sure to document all useful env vars in `backend/README.md`

Whisper cpp's source code is available in https://github.com/ggml-org/whisper.cpp . I
probably checked it out locally at ~/Github/whisper.cpp/. You may want to pull the latest
trunk/main/master branch. You may need to download new model files. Check
`pub_musings/cc_plugins/skills/*` for how to run whisper-cli locally and to add subtitles
to a video. Add documentation for what's useful.

File storage should be encrypted at rest using `age`. Max file size should be 500 MB to
start. The database should be sqlite. Authentication should be based on email + password.
As a user, I should be able to add 2-factor auth to my account via apps like Google
Authenticator on my phone. Bonus points if I can use passkeys.

Email service should be Resend API. You are blocked on me adding a real Resend API key.
You should search for and use a good Resend mock library for unit tests. If you can't
find one, then you should write one. You should similarly search or write a mock
implementation that can be used in integration tests.

Every third party API we add should have a mock library for unit tests and a mock
implementation for integration tests. Some API providers provide "sandbox" environments
that are useful to include in our [documentation](#Documentation).

Environment variables should be encrypted with `sops`.

Deploys are TBD. Writing a plan is TODO. Implementing requires human intervention. I plan
to run the website on a Mac Mini in a qemu VM, and ideally in a docker container inside
of the VM. I would like to figure out how to passthrough Mac Metal via MoltenVK to qemu.
Write a plan for that is TODO. Write a plan for how to leverage
`pub_musings/webhook-deployer/` which was described in
`pub_musings/personal/001_INITIALIZATION.md` to trigger deploys for subtitler, frontend
and backend.

Production deploys will leverage astro built static files with Caddy as the frontend load
balancer. Caddy can route all /api/* requests to the backend.

## Competitors

TODO research their landing pages, new user signup flows, pricing and performance.

## Documentation

Write documentation after every meaningful change. If documentation gets too big, then
move old documentation out of frequently checked docs and add links to it. Make .md files
link to each other.
