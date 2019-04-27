# Synchronise Kindle & Mac

Synchronise books between a Mac and a Kindle. In practice this means
synchronising from PDFs in the iCloud Apple Books folder and optionally mobi
files from a specified documents directory.

It assumes that all PDFs are in the iCloud Apple Books folder, and all Mobi
files, being unreadable by Apple Books, are in the specified documents
directory.

For now it'll warn about epub files in iCloud Books, warning that they cannot
be synchronised with the Kindle due to being unreadable on it, and will skip
but log PDF files outside of the iCloud Apple Books folder but inside the
specified documents directory.

Symlinks inside the documents directory are not followed.

