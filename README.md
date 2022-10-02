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
$ go run sync-kobo-and-workstation.go -docs-dirs "$HOME/Documents/Books, Papers & Articles/Languages:$HOME/Documents/Books, Papers & Articles/Miscellaneous:$HOME/Documents/Books, Papers & Articles/Security:$HOME/Documents/Books, Papers & Articles/Technology:$HOME/Documents/Books, Papers & Articles/Travel"
2020/12/31 13:11:55 found documents in the /home/user/Documents/Books, Papers & Articles/Languages directory: 2
2020/12/31 13:11:55 found documents in the /home/user/Documents/Books, Papers & Articles/Travel directory: 1
2020/12/31 13:11:55 found documents in the /home/user/Documents/Books, Papers & Articles/Miscellaneous directory: 19
2020/12/31 13:11:55 found documents in the /home/user/Documents/Books, Papers & Articles/Security directory: 207
2020/12/31 13:11:55 found documents in the /home/user/Documents/Books, Papers & Articles/Technology directory: 95
2020/12/31 13:11:55 books not copied because they already existed: 324
2020/12/31 13:11:55 books copied: 0
```

Symlinks inside the documents directory are not followed.

This repository is currently hosted [on
GitLab.com](https://gitlab.com/louis.jackman/sync-kobo-and-workstation).
Official mirrors exist on
[SourceHut](https://git.sr.ht/~louisjackman/sync-kobo-and-workstation) and
[GitHub](https://github.com/LouisJackman/sync-kobo-and-workstation). At the
moment, GitLab is still the official hub for contributions such as PRs and
issues.

