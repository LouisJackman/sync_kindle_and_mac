package main

// Synchronise books between a Mac and a Kindle. In practice this means
// synchronising from PDFs in the iCloud Apple Books folder and optionally mobi
// files from a specified documents directory.
//
// It assumes that all PDFs are in the iCloud Apple Books folder, and all Mobi
// files, being unreadable by Apple Books, are in the specified documents
// directory.
//
// For now it'll warn about epub files in iCloud Books, warning that they cannot
// be synchronised with the Kindle due to being unreadable on it, and will skip
// but log PDF files outside of the iCloud Apple Books folder but inside the
// specified documents directory.
//
// Symlinks inside the documents directory are not followed.

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

var defaultKindleDir = path.Join("/", "Volumes", "Kindle")

const kindleDirHelp = "the directory into the the kindle is mounted"
const appleBooksDirHelp = "the directory containing the Apple Books library"
const docsDirsHelp = "the directories containing documents not managed by Apple Books, like ad hoc mobi files, separated by commas"
const dryRunHelp = "whether to just inform where files would be copied, rather than actually doing it"

type copyOperation struct {
	src, dest string
}

type args struct {
	kindleDir, appleBooksDir string
	docsDirs                 []string
	dryRun                   bool
}

func lookupHomeDir() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	home := user.HomeDir
	if len(home) == 0 {
		return "", fmt.Errorf("no home dir found for current user %s", user.Uid)
	}
	return home, nil
}

func lookupDefaultAppleBooksDir(home string) string {
	return path.Join(home, "Library", "Mobile Documents", "iCloud~com~apple~iBooks", "Documents")
}

func lookupDefaultDocsDirs(home string) []string {
	return []string{
		path.Join(home, "Documents"),
		path.Join(home, "Desktop"),
	}
}

func findBooks(srcDir, extToMatch string, booksToSync chan string, errors chan error, wg *sync.WaitGroup) uint {
	defer wg.Done()

	var matchCount uint
	err := filepath.Walk(srcDir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			errors <- err
		} else if filepath.Ext(path) == extToMatch {
			booksToSync <- path
			matchCount++
		}
		return nil
	})
	if err != nil {
		errors <- err
		return 0
	}
	return matchCount
}

func findAppleBooks(appleBooksDir string, booksToSync chan string, errors chan error, wg *sync.WaitGroup) {
	findBooks(appleBooksDir, ".pdf", booksToSync, errors, wg)
}

func findDocFiles(docsDirs []string, booksToSync chan string, errors chan error, wg *sync.WaitGroup) {
	for _, dir := range docsDirs {
		wg.Add(1)
		go findBooks(dir, ".mobi", booksToSync, errors, wg)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func copyBook(kindleDir, book string, wg *sync.WaitGroup, errors chan error) {
	defer wg.Done()

	src, err := os.Open(book)
	if err != nil {
		errors <- err
		return
	}
	defer src.Close()

	destPath := path.Join(kindleDir, path.Dir(book))
	if fileExists(destPath) {
		return
	}

	dest, err := os.Open(destPath)
	if err != nil {
		errors <- err
		return
	}
	defer dest.Close()

	_, err = io.Copy(dest, src)
	if err != nil {
		errors <- err
	}
}

func syncBooks(kindleDir, appleBooksDir string, docsDirs []string, errors chan error, dryRun bool, wg *sync.WaitGroup) {
	defer wg.Done()

	booksToSync := make(chan string)

	var syncWait sync.WaitGroup
	syncWait.Add(1)
	go findAppleBooks(appleBooksDir, booksToSync, errors, &syncWait)
	findDocFiles(docsDirs, booksToSync, errors, &syncWait)

	go func() {
		syncWait.Wait()
		close(booksToSync)
	}()

	var copyWait sync.WaitGroup
	for book := range booksToSync {
		if dryRun {
			log.Printf("would copy book at %s to the Kindle at %s\n", book, kindleDir)
		} else {
			copyWait.Add(1)
			go copyBook(kindleDir, book, &copyWait, errors)
		}
	}
	copyWait.Wait()
}

func missingArgPathErr(name, path string) error {
	return fmt.Errorf("for the %s, the path %s does not exist", name, path)
}

func parseArgs() (result args, err error) {
	var homeDir string
	homeDir, err = lookupHomeDir()
	if err != nil {
		return
	}

	defaultAppleBooksDir := lookupDefaultAppleBooksDir(homeDir)

	kindleDir := flag.String("kindle-dir", defaultKindleDir, kindleDirHelp)
	appleBooksDir := flag.String("apple-books-dir", defaultAppleBooksDir, appleBooksDirHelp)
	docsDirsStr := flag.String("docs-dirs", "", docsDirsHelp)
	dryRun := flag.Bool("dry-run", false, dryRunHelp)
	flag.Parse()

	var docsDirs []string
	if len(*docsDirsStr) <= 0 {
		docsDirs = lookupDefaultDocsDirs(homeDir)
	} else {
		docsDirs = strings.Split(*docsDirsStr, ",")
	}

	if !fileExists(*kindleDir) {
		err = fmt.Errorf(
			"the directory %s does not exist; are you sure your Kindle is plugged in? Double-check by opening Finder and seeing if it is connected",
			*kindleDir,
		)
	} else if !fileExists(*appleBooksDir) {
		err = missingArgPathErr("iCloud Apple Books", *appleBooksDir)
	} else {
		for _, docsDir := range docsDirs {
			if !fileExists(docsDir) {
				err = missingArgPathErr("document files", docsDir)
				break
			}
		}
		result = args{
			kindleDir:     *kindleDir,
			appleBooksDir: *appleBooksDir,
			docsDirs:      docsDirs,
			dryRun:        *dryRun,
		}
	}
	return
}

func main() {
	args, err := parseArgs()
	if err != nil {
		log.Fatalln(err)
	}

	errors := make(chan error)

	var wg sync.WaitGroup
	wg.Add(1)
	go syncBooks(args.kindleDir, args.appleBooksDir, args.docsDirs, errors, args.dryRun, &wg)

	finished := make(chan struct{})
	go func() {
		wg.Wait()
		finished <- struct{}{}
	}()

	select {
	case err := <-errors:
		fmt.Fprintln(os.Stderr, err)
		for err := range errors {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	case <-finished:
	}
}
