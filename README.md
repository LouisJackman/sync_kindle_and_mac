# Synchronise Kobo & Workstation

[![pipeline status](https://gitlab.com/louis.jackman/sync-kobo-and-workstation/badges/master/pipeline.svg)](https://gitlab.com/louis.jackman/sync-kobo-and-workstation/-/commits/master)

Synchronise books between a workstation and a Kobo e-book reader. In practice,
this means synchronising a connected Kobo volume with EPUB and PDF files in the
specified local documents directory.

The defaults are Linux-specific, assuming a udisks2-style automount directory
as `/var/media/user/KOBOeReader` for the destination Kobo and defaulting to
`~/Documents` for the source. However, if these defaults are overridden with
explicit values, it will likely work on other OSes too.

```shell
$ go build -o sync-kobo-and-workstation main.go
$ ./sync-kobo-and-workstation
2022/10/02 20:32:04 found documents in the /home/user/Documents directory: 305
2022/10/02 20:32:07 books not copied because they already existed on the destination Kobo: 299
2022/10/02 20:32:07 books copied: 6
```

Symlinks inside the documents directory are not followed.

This repository is currently hosted [on
GitLab.com](https://gitlab.com/louis.jackman/sync-kobo-and-workstation).
Official mirrors exist on
[SourceHut](https://git.sr.ht/~louisjackman/sync-kobo-and-workstation) and
[GitHub](https://github.com/LouisJackman/sync-kobo-and-workstation). At the
moment, GitLab is still the official hub for contributions such as PRs and
issues.

