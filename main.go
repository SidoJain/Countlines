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

const (
    RESET      	= "\033[0m"
    GRAY       	= "\033[90m"
    BRIGHT_RED  = "\033[91m"
    YELLOW     	= "\033[33m"
    CYAN       	= "\033[0;36m"
    BRIGHT_BLUE = "\033[1;34m"
)

type stringSlice []string

func (str *stringSlice) String() string {
    return fmt.Sprint(*str)
}

func (str *stringSlice) Set(value string) error {
    *str = append(*str, value)
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

func cloneRepo(url string, RESET string, GRAY string, BRIGHT_RED string) (string, error) {
    dir, err := os.MkdirTemp("", "countlines-")
    if err != nil {
        return "", err
    }
    cmd := exec.Command("git", "clone", "--depth", "1", url, dir)
	fmt.Printf("%sCloning into '%s'...%s\n", GRAY, dir, RESET)
    if err := cmd.Run(); err != nil {
		fmt.Println(BRIGHT_RED + "ERROR: Repository not found.")
		fmt.Println("fatal: Could not read from remote repository.")
		fmt.Printf("\nmake sure you have the correct perms and the repo exists.%s\n", RESET)
        os.RemoveAll(dir)
        return "", err
    }
    return dir, nil
}

func formatNumber(n int64) string {
    str := fmt.Sprintf("%d", n)
    nStr := ""
    for i, ch := range str {
        if i > 0 && (len(str) - i) % 3 == 0 {
            nStr += ","
        }
        nStr += string(ch)
    }
    return nStr
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	noColor := flag.Bool("no-color", false, "disable colored output")
	var blacklist stringSlice
    flag.Var(&blacklist, "blacklist", "patterns of files or directories to exclude (can be specified multiple times)")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: countlines.exe [-flags] <directory/url> [pattern1] [pattern2] ...\n")
        os.Exit(1)
    }

	getColor := func(color string) string {
        if *noColor {
            return ""
        }
        return color
    }

    RESET := getColor("\033[0m")
    GRAY := getColor("\033[90m")
    BRIGHT_RED := getColor("\033[91m")
    YELLOW := getColor("\033[33m")
    CYAN := getColor("\033[0;36m")
    BRIGHT_BLUE := getColor("\033[1;34m")

	isUrl := false
	input := flag.Arg(0)
	var root string
	var patterns stringSlice
	if isGitHubRepo(input) {
		isUrl = true
		tmp, err := cloneRepo(input, RESET, GRAY, BRIGHT_RED)
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
					fmt.Printf("%sRead file:%s %s - %s(%s)%s\n", CYAN, RESET, relPath, YELLOW, formatNumber(lines), RESET)
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

	fmt.Println(BRIGHT_BLUE + "File Count:", formatNumber(totalFiles))
	fmt.Println("Line Count:", formatNumber(totalLines), RESET)
	if isUrl {
		fmt.Println(GRAY + "Cloned repo has been deleted.", RESET)
	}
}