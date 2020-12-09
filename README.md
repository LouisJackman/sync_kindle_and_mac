# Synchronise Kindle & Mac

[![pipeline status](https://gitlab.com/louis.jackman/sync-kindle-and-workstation/badges/master/pipeline.svg)](https://gitlab.com/louis.jackman/sync-kindle-and-workstation/-/commits/master)

Synchronise books between a Mac and a Kindle. In practice this means
synchronising from PDFs in the iCloud Apple Books folder and optionally mobi
files from a specified documents directory.

```shell
$ go run sync-kindle-and-mac.go
2019/04/30 05:52:12 found Mobi files in the /Users/User/Desktop directory: 0
2019/04/30 05:52:12 found books in Apple Books iCloud Folder: 190
2019/04/30 05:52:12 found Mobi files in the /Users/User/Documents directory: 2
2019/04/30 05:52:15 books not copied because they already existed: 170
2019/04/30 05:52:15 books copied: 22
```

It assumes that all PDFs are in the iCloud Apple Books folder, and all Mobi
files, being unreadable by Apple Books, are in the specified documents
directory.

For now it'll skip epub files in iCloud Books, as they cannot be synchronised
with the Kindle due to being unreadable on it, and will skip PDF files outside
of the iCloud Apple Books folder but inside the specified documents directory,
as it expects those to be managed solely by Apple Books.

Symlinks inside the documents directory are not followed.

This repository is currently hosted [on
GitLab.com](https://gitlab.com/louis.jackman/sync-kindle-and-workstation).
Official mirrors exist on
[SourceHut](https://git.sr.ht/~louisjackman/sync-kindle-and-workstation) and
[GitHub](https://github.com/LouisJackman/sync-kindle-and-workstation). At the
moment, GitLab is still the official hub for contributions such as PRs and
issues.

