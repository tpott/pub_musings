# subtitler

* downloads/
* audios/
* job-start-command.json
* outputs/
* subtitles/
* final/

# Dependencies

* awscli, version >= 1.18 (probably install via pip and export PATH=$PATH:~/.local/bin)
* ffmpeg
* jq
* scipy
* youtube-dl

# Development

I would suggest installing this basket of packages (list from https://www.scipy.org/install.html#pip-install)

* numpy
* matplotlib
* ipython
* jupyter
* pandas
* sympy
* nose

You can then generate a new TLS certificate by running `openssl req -nodes -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 30`.
Note that this command will prompt you to fill in several fields for the certificate.
Those are entirely optional. Because you will not be getting this certificate signed,
you will see the "Not secure" warning in Chrome/other browsers.

To run a jupyter server, I would then recommend running `jupyter notebook --certfile cert.pem --keyfile key.pem --notebook-dir notebooks/ 2> logs &`
This will run the jupyter process in the [background](https://en.wikipedia.org/wiki/Background_process).
To set a secure password, you can run `python3 -c "print(open(\"/dev/urandom\", \"rb\").read(16).hex())"`
to generate a long, hexicode password. Finally, run `jupyter notebook password` and
paste the password you generated.

