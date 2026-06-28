# peter rabbit

This dir only contains the example design plan and the plain HTML. It doesn't include the images
because you can fetch them yourself from [project gutenberg](https://www.gutenberg.org/ebooks/14838).

## Page budget & blank covers

This book follows the shared convention (see the
[main README](../../README.md#page-budget--blank-covers)): the inside front cover
(page 2) and inside back cover (page 31) are **blank**, as is the back cover
(page 32). It is a larger book than the minimum — 32 pages (8 folded sheets):
4 front-matter pages, 26 numbered story pages, and the 2 trailing blank pages.

## Ink-saving image modes

Print-only treatments that use less printer ink (`ink-lite` = lightened grayscale photo, `pencil` = bold line-art sketch; omit for full tone). They change nothing on screen.

Toggle per print without editing the file by appending a `#hash` to the URL:

| URL | Mode |
|-----|------|
| `peter-rabbit.html` | full tone (default) |
| `peter-rabbit.html#ink-lite` | ink-lite |
| `peter-rabbit.html#pencil` | pencil |

To change the *default* (the bare-URL mode), set the class on `<body>` instead:

```diff
-<body>
+<body class="ink-lite">
```
