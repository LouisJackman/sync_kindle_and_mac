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
	"sync/atomic"
)

var defaultKindleDir = path.Join("/", "Volumes", "Kindle", "documents", "PDFs")

const kindleDirHelp = "the directory into the the kindle is mounted"
const appleBooksDirHelp = "the directory containing the Apple Books library"
const docsDirsHelp = "the directories containing documents not managed by Apple Books, like ad hoc mobi files, separated by commas"
const dryRunHelp = "whether to just inform where files would be copied, rather than actually doing it"

type stats struct {
	category string
	count    uint64
}

type copyOperation struct {
	src, dest string
	dryRun    bool
}

type copyResult struct {
	wg                        *sync.WaitGroup
	errors                    chan error
	skippedCount, copiedCount *uint64
}

type args struct {
	kindleDir, appleBooksDir string
	docsDirs                 []string
	dryRun                   bool
}

type bookSearch struct {
	category, srcDir, extToMatch string
}

type foundBooks struct {
	matches chan string
	errors  chan error
	wg      *sync.WaitGroup
	count   *uint64
	stats   chan stats
}

type syncOperation struct {
	kindleDir, appleBooksDir string
	docsDirs                 []string
	dryRun                   bool
}

type syncResults struct {
	errors chan error
	wg     *sync.WaitGroup
	stats  chan stats
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

func findBooks(search bookSearch, found foundBooks) {
	defer found.wg.Done()

	var count uint64

	err := filepath.Walk(search.srcDir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			found.errors <- err
		} else if filepath.Ext(path) == search.extToMatch {
			found.matches <- path
			count++
		}
		return nil
	})
	if err != nil {
		found.errors <- err
	}

	found.stats <- stats{
		category: search.category,
		count:    count,
	}
}

func findAppleBooks(appleBooksDir string, found foundBooks) {
	search := bookSearch{
		srcDir:     appleBooksDir,
		extToMatch: ".pdf",
		category:   "found books in Apple Books iCloud Folder",
	}
	findBooks(search, found)
}

func findDocFiles(docsDirs []string, found foundBooks) {
	for _, dir := range docsDirs {
		category := fmt.Sprintf("found Mobi files in the %s directory", dir)
		search := bookSearch{
			srcDir:     dir,
			extToMatch: ".mobi",
			category:   category,
		}

		found.wg.Add(1)
		go findBooks(search, found)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func copyBook(operation copyOperation, result *copyResult) {
	defer result.wg.Done()

	destPath := path.Join(operation.dest, path.Base(operation.src))
	if fileExists(destPath) {
		atomic.AddUint64(result.skippedCount, 1)
		return
	}

	if operation.dryRun {
		log.Printf("would copy book at %s to the Kindle at %s\n", operation.src, destPath)
		atomic.AddUint64(result.copiedCount, 1)
		return
	}

	src, err := os.Open(operation.src)
	if err != nil {
		result.errors <- err
		return
	}
	defer src.Close()

	mode := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	dest, err := os.OpenFile(destPath, mode, 0644)
	if err != nil {
		result.errors <- err
		return
	}
	defer dest.Close()

	_, err = io.Copy(dest, src)
	if err != nil {
		result.errors <- err
		return
	}

	atomic.AddUint64(result.copiedCount, 1)
}

func syncBooks(operation syncOperation, results syncResults) {
	defer results.wg.Done()

	booksToSync := make(chan string)

	var syncWait sync.WaitGroup

	var appleBooksCount uint64
	foundAppleBooks := foundBooks{
		matches: booksToSync,
		errors:  results.errors,
		wg:      &syncWait,
		count:   &appleBooksCount,
		stats:   results.stats,
	}
	syncWait.Add(1)
	go findAppleBooks(operation.appleBooksDir, foundAppleBooks)

	var docFilesCount uint64
	foundDocFiles := foundBooks{
		matches: booksToSync,
		errors:  results.errors,
		wg:      &syncWait,
		count:   &docFilesCount,
		stats:   results.stats,
	}
	findDocFiles(operation.docsDirs, foundDocFiles)

	go func() {
		syncWait.Wait()
		close(booksToSync)
	}()

	var skippedCount, copiedCount uint64
	var copyWait sync.WaitGroup
	for book := range booksToSync {
		copyWait.Add(1)
		operation := copyOperation{
			src:    book,
			dest:   operation.kindleDir,
			dryRun: operation.dryRun,
		}
		result := copyResult{
			errors:       results.errors,
			wg:           &copyWait,
			skippedCount: &skippedCount,
			copiedCount:  &copiedCount,
		}
		go copyBook(operation, &result)
	}
	copyWait.Wait()

	results.stats <- stats{
		category: "books not copied because they already existed",
		count:    skippedCount,
	}

	var copiedStatsCategory string
	if operation.dryRun {
		copiedStatsCategory = "books that would be copied"
	} else {
		copiedStatsCategory = "books copied"
	}
	results.stats <- stats{
		category: copiedStatsCategory,
		count:    copiedCount,
	}
	close(results.stats)
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
	stats := make(chan stats)

	var wg sync.WaitGroup

	operation := syncOperation{
		kindleDir:     args.kindleDir,
		appleBooksDir: args.appleBooksDir,
		docsDirs:      args.docsDirs,
		dryRun:        args.dryRun,
	}
	results := syncResults{
		errors: errors,
		wg:     &wg,
		stats:  stats,
	}

	wg.Add(1)
	go syncBooks(operation, results)

	go func() {
		for stat := range stats {
			log.Printf("%s: %d\n", stat.category, stat.count)
		}
		wg.Wait()
		close(errors)
	}()

	var errFound bool
	for err := range errors {
		fmt.Fprintln(os.Stderr, err)
		errFound = true
	}

	if errFound {
		os.Exit(1)
	}
}
