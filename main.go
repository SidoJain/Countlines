package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
)

type stringSlice []string

func (s *stringSlice) String() string {
    return fmt.Sprint(*s)
}

func (s *stringSlice) Set(value string) error {
    *s = append(*s, value)
    return nil
}

func countLines(filename string) (int64, error) {
    file, err := os.Open(filename)
    if err != nil {
        return 0, err
    }
    defer file.Close()

    var count int64
    buf := make([]byte, 32 * 1024)
    for {
        n, err := file.Read(buf)
        if n > 0 {
            for i := range n {
                if buf[i] == '\n' {
                    count++
                }
            }
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return count + 1, err
        }
    }
    return count + 1, nil
}

func isBlacklisted(name string, blacklist stringSlice) bool {
    for _, pattern := range blacklist {
        matched, err := filepath.Match(pattern, name)
        if err == nil && matched {
            return true
        }
    }
    return false
}

func isGitHubRepo(url string) bool {
    match, _ := regexp.MatchString(`^https://github.com/.+/.+`, url)
    return match
}

func cloneRepo(url string) (string, error) {
    dir, err := os.MkdirTemp("", "countlines-")
    if err != nil {
        return "", err
    }
    cmd := exec.Command("git", "clone", "--depth", "1", url, dir)
	fmt.Printf("\033[90mCloning into '%s'...\033[0m\n", dir)
    if err := cmd.Run(); err != nil {
		fmt.Println("\033[91mERROR: Repository not found.")
		fmt.Println("fatal: Could not read from remote repository.")
		fmt.Printf("\nmake sure you have the correct perms and the repo exists.\033[0m\n")
        os.RemoveAll(dir)
        return "", err
    }
    return dir, nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var blacklist stringSlice
    flag.Var(&blacklist, "blacklist", "patterns of files or directories to exclude (can be specified multiple times)")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: countlines.exe [-blacklist pattern] <directory/url> [pattern1] [pattern2] ...\n")
        os.Exit(1)
    }

	isUrl := false
	input := flag.Arg(0)
	var root string
	var patterns stringSlice
	if isGitHubRepo(input) {
		isUrl = true
		tmp, err := cloneRepo(input)
		if err != nil {
			log.Fatalf("Error cloning repo: %v", err)
		}
		blacklist = append(blacklist, ".git")
		defer os.RemoveAll(tmp)
		root = tmp
		patterns = flag.Args()[1:]
	} else {
		root = input
		patterns = flag.Args()[1:]
	}

	if len(patterns) == 0 {
		patterns = stringSlice{"*"}
	}

	var totalLines int64
	var totalFiles int64
	var wg sync.WaitGroup
	filesChan := make(chan string, 100)
	resultsChan := make(chan int64, 100)

	// Worker goroutines to count lines
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range filesChan {
				lines, err := countLines(filename)
				relPath, relErr := filepath.Rel(root, filename)
				if relErr != nil {
					relPath = filename
				}
				if err == nil {
					fmt.Printf("\033[0;36mRead file: \033[0m%s\033[0m - \033[33m(%d)\033[0m\n", relPath, lines)
					resultsChan <- lines
				}
			}
		}()
	}

	// Walk files and send matching files to filesChan
	go func() {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			base := filepath.Base(path)
			if isBlacklisted(base, blacklist) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			for _, pattern := range patterns {
				match, err := filepath.Match(pattern, base)
				if err == nil && match {
					filesChan <- path
					break
				}
			}
			return nil
		})
		close(filesChan)
	}()

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for lines := range resultsChan {
		totalLines += lines
		totalFiles++
	}

	fmt.Println("\033[1;34mFile Count:", totalFiles)
	fmt.Println("\033[1;34mLine Count:", totalLines, "\033[0;37m")
	if isUrl {
		fmt.Println("\033[90mCloned repo has been deleted.\033[0m")
	}
}