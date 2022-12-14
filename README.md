# Synchronise Kobo & Workstation

[![pipeline status](https://gitlab.com/louis.jackman/sync-kobo-and-workstation/badges/master/pipeline.svg)](https://gitlab.com/louis.jackman/sync-kobo-and-workstation/-/commits/master)

Synchronise books between a workstation and a Kobo e-book reader. In practice,
this means synchronising a connected Kobo volume with EPUB and PDF files in the
specified local documents directories.

The defaults are Linux-specific, assuming a udisks2-style automount directory
such as `/var/media/user/KOBOeReader` for the destination Kobo and defaulting to
just `~/Documents` for the source. However, if these defaults are overridden
with explicit values, it will likely work on other OSes too.

```shell
$ cd sync-kobo-and-workstation
$ cargo build --release
$ # Now, move the built executable at `target/release/sync-kobo-and-workstation`
$ # wherever you like and run it like so:
$ sync-kobo-and-workstation --documents-directories="$HOME/Documents"
Book ./tmp/Paradigms of Artificial Intelligence Programming.epub already exists on the destination; will not copy across
Book ./tmp/debian-handbook.epub already exists on the destination; will not copy across

Found documents in documents directory at /var/home/user/Documents: 2
Books not copied because they already exist on the destination Kobo: 2
Book copied: 0
```

Symlinks inside the documents directories are not followed.

This repository is currently hosted [on
GitLab.com](https://gitlab.com/louis.jackman/sync-kobo-and-workstation).
Official mirrors exist on
[SourceHut](https://git.sr.ht/~louisjackman/sync-kobo-and-workstation) and
[GitHub](https://github.com/LouisJackman/sync-kobo-and-workstation). At the
moment, GitLab is still the official hub for contributions such as PRs and
issues.

