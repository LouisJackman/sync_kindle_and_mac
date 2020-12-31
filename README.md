# Synchronise Kindle & Workstation

[![pipeline status](https://gitlab.com/louis.jackman/sync-kindle-and-workstation/badges/master/pipeline.svg)](https://gitlab.com/louis.jackman/sync-kindle-and-workstation/-/commits/master)

Synchronise books between a workstation and a Kindle. In practice this
means synchronising a connected Kindle volume with PDFs and mobi files
in the specified documents directory.

The defaults are Linux-specific, e.g. assuming a udisks2-style
automount directory such as `/media/user/Kindle` for the destination
Kindle and defaulting to `~/Documents` for the source. However, if
these defaults are overridden with explicit values, it will likely
work on other OSes too.

```shell
$ go run sync-kindle-and-workstation.go -docs-dirs "$HOME/Documents/Books, Papers & Articles/Languages:$HOME/Documents/Books, Papers & Articles/Miscellaneous:$HOME/Documents/Books, Papers & Articles/Security:$HOME/Documents/Books, Papers & Articles/Technology:$HOME/Documents/Books, Papers & Articles/Travel"
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
GitLab.com](https://gitlab.com/louis.jackman/sync-kindle-and-workstation).
Official mirrors exist on
[SourceHut](https://git.sr.ht/~louisjackman/sync-kindle-and-workstation) and
[GitHub](https://github.com/LouisJackman/sync-kindle-and-workstation). At the
moment, GitLab is still the official hub for contributions such as PRs and
issues.

