package main

import (
	"errors"
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

const (
	koboDirHelp  = "the destination directory into the the Kobo is mounted"
	docsDirsHelp = "the source directories containing documents, separated by colons"
	dryRunHelp   = "whether to just inform where files would be copied, rather than actually doing it"

	docsDirsArgSplitChar = ":"
)

type (
	stats struct {
		category string
		count    uint64
	}

	copyOperation struct {
		src, dest string
		dryRun    bool
	}

	copyResult struct {
		wg                        *sync.WaitGroup
		errors                    chan error
		skippedCount, copiedCount *uint64
	}

	args struct {
		koboDir  string
		docsDirs []string
		dryRun   bool
	}

	bookSearch struct {
		category, srcDir string
		extsToMatch      []string
	}

	foundBooks struct {
		matches chan string
		errors  chan error
		wg      *sync.WaitGroup
		count   *uint64
		stats   chan stats
	}

	syncOperation struct {
		docsDirs []string
		koboDir  string
		dryRun   bool
	}

	syncResults struct {
		errors chan error
		wg     *sync.WaitGroup
		stats  chan stats
	}
)

var extsToMatch = []string{".epub", ".pdf"}

func lookupDefaultKoboDir() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	return path.Join("/", "media", user.Username, "KOBOeReader"), nil
}

func lookupHomeDir() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	home := user.HomeDir
	if len(home) == 0 {
		return "", fmt.Errorf("no home dir found for current user %s", user.Username)
	}
	return home, nil
}

func lookupDefaultDocsDirs(home string) []string {
	return []string{
		path.Join(home, "Documents"),
	}
}

func findBooks(search bookSearch, found foundBooks) {
	defer found.wg.Done()

	var count uint64

	err := filepath.Walk(search.srcDir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			found.errors <- err
		} else {
			for _, extToMatch := range search.extsToMatch {
				if filepath.Ext(path) == extToMatch {
					found.matches <- path
					count++
					break
				}
			}
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

func findDocFiles(docsDirs []string, found foundBooks) {
	for _, dir := range docsDirs {
		category := fmt.Sprintf("found documents in the %s directory", dir)
		search := bookSearch{
			srcDir:      dir,
			extsToMatch: extsToMatch,
			category:    category,
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
		log.Printf("would copy book at %s to the Kobo at %s\n", operation.src, destPath)
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
			dest:   operation.koboDir,
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
		category: "books not copied because they already existed on the destination Kobo",
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

	var defaultKoboDir string
	defaultKoboDir, err = lookupDefaultKoboDir()
	if err != nil {
		return
	}

	koboDir := flag.String("kobo-dir", defaultKoboDir, koboDirHelp)
	docsDirsStr := flag.String("docs-dirs", "", docsDirsHelp)
	dryRun := flag.Bool("dry-run", false, dryRunHelp)
	flag.Parse()

	var docsDirs []string
	if len(*docsDirsStr) <= 0 {
		docsDirs = lookupDefaultDocsDirs(homeDir)
	} else {
		docsDirs = strings.Split(*docsDirsStr, docsDirsArgSplitChar)
	}

	if !fileExists(*koboDir) {
		err = fmt.Errorf(
			"the directory %s does not exist; are you sure your Kobo is plugged in and mounted? Double-check by opening Files and seeing whether it is connected",
			*koboDir,
		)
	} else {
		docsDirSet := make(map[string]bool)
		for _, docsDir := range docsDirs {
			if !fileExists(docsDir) {
				err = missingArgPathErr("document files", docsDir)
				return
			}

			if _, exists := docsDirSet[docsDir]; exists {
				err = errors.New("duplicate source document directory: " + docsDir)
				return
			}
			docsDirSet[docsDir] = true
		}

		result = args{
			koboDir:  *koboDir,
			docsDirs: docsDirs,
			dryRun:   *dryRun,
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
		koboDir:  args.koboDir,
		docsDirs: args.docsDirs,
		dryRun:   args.dryRun,
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
