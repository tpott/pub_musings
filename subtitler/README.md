# subtitler

Data directories (nested inside `data/`)

* `downloads/` for the initial download
* `audios/` the audio extracted from the video
* `outputs/` raw JSON output from AWS transcribe
* `tsvs/` TSV parsed data derived from the JSON output. May be modified to improve classification.
* `subtitles/` formatted SubRip subtitles files
* `final/` final formatted video with subtitles
* [`tmp/` only for utterance_server.py]
* `video_ids` maps video ids to jobs and names
* `video_names` maps video names to jobs and ids
* `models` stores sklearn serialized (pickled) model files
* `lyrics` stores optional plain lyric files

Create them via `mkdir -p data/downloads data/audios data/outputs data/tsvs data/subtitles data/final data/tmp data/video_ids data/video_names data/models data/lyrics`

# Dependencies

* awscli, version >= 1.18 (probably install via pip and export PATH=$PATH:~/.local/bin)
* ffmpeg
* jq
* scipy (pip)
* youtube-dl (pip)
* [mlr]

Run `sudo apt-get install ffmpeg jq miller python3-pip`

# Development

I would suggest installing this basket of packages (list from https://www.scipy.org/install.html#pip-install)

* numpy
* matplotlib
* ipython
* jupyter
* pandas
* sympy
* nose
* scikit-learn

Run `pip3 install numpy matplotlib ipython jupyter pandas sympy nose scikit-learn youtube_dl awscli`

`mlr` is also a really handy command-line tool. It can be pretty powerful when
combined with `jq` in the modern age where everything is stored in json.

You can then generate a new TLS certificate by running `openssl req -nodes -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 30`.
Note that this command will prompt you to fill in several fields for the certificate.
Those are entirely optional. Because you will not be getting this certificate signed,
you will see the "Not secure" warning in Chrome/other browsers.

To set a secure password for your jupyter server, you can run `python3 -c "print(open(\"/dev/urandom\", \"rb\").read(16).hex())"`
to generate a long, hexicode password. Finally, run `jupyter notebook password` and
paste the password you generated.

To run a jupyter server, I would then recommend running
`jupyter notebook --certfile cert.pem --keyfile key.pem --notebook-dir notebooks/ 2> logs &`
This will run the jupyter process in the [background](https://en.wikipedia.org/wiki/Background_process).
Or you can start a screen (for example: `screen -S jup`), and run the jupyter server there.
Same command but replace `2> logs &` with `2>&1 | tee logs`

# utterance server

Tested by running `python3 utterance_server.py` and then opening [https://localhost:8000/label?video=wr2sVPTacTE&utterance=28](https://localhost:8000/label?video=wr2sVPTacTE&utterance=28)

Writes to `tmp/`
