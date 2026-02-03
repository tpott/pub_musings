* I want to make a web app that includes some speech-to-text, LLM processing, and text-to-speech, similar to how a google home speaker works
* I want to focus on the prompts: "show me a cat", "show me a dog", "show me a giraffe"
* I want the web app to show me a photo and play audio or play a video clip
* there should be a database of photos, audios and videos associated with common kid topics, like "cat", "dog", etc
* I want to setup deploys in pub_musings/webhook-deployer/config.yaml similar to how we did for pub_musings/subtitler
* I don't want wake words. I want a simple button that turns the app on and it can be toggled to turn the microphone off.
* I want to invest in tuning the speech-to-text. I want full sentences streamed plus words/phrases/phonemes so that the LLM can guess what is being said
* Ideally we use whisper stream, and build a fork that supports webhooks and get it deployed similar to how I run whisper outside the vm in pub_musings/subtitler/specs/{something}.md
