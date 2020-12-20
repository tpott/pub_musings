# qrmodem

A simple PoC experiment to see if QR codes can be used to send and receive data between
two devices. The basic idea is that multiple QR codes can be rendered as a video as a
"transmitter", and a camera can recognize the codes as a receiver.

[Version 40-L](https://en.wikipedia.org/wiki/QR_code#Storage) QR Codes have a "low"
amount of error-correction, and have a 177x177 grid of dots. This allows for up to
2953 Bytes, or 23624 bits.

Most LED/LCD screens support refresh rates of 60 Hz. Newer models support up to 90 Hz,
or maybe even as high as 120 Hz. But assuming a slow rate of 5 Hz, we should be able
to transfer ~118 kb/s. 50 Hz gets more useful, i.e. around ~1.1 Mb/s. And if we we're
to encode one frame in red, one in green, and one in blue, then we should be able to
increase that rate further. Alternatively, also increasing the grid size.

# Dependencies

* https://github.com/neocotic/qrious but note that it depends on https://github.com/neocotic/qrious-core
* https://github.com/nimiq/qr-scanner
* https://github.com/mrdoob/stats.js
