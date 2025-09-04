package main

import (
	"flag"
	"fmt"
	"bufio"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sort"
	"strings"
	"encoding/csv"
)

type stringSlice []string

type colors struct {
    RESET       string
    GRAY        string
    RED   		string
    YELLOW      string
    CYAN        string
    BLUE	  	string
}

type fileLineInfo struct {
	path  string
	lines int64
}

func (str *stringSlice) String() string {
    return fmt.Sprint(*str)
}

func (str *stringSlice) Set(value string) error {
    parts := strings.SplitSeq(value, ",")
    for part := range parts {
        trimmed := strings.TrimSpace(part)
        if trimmed != "" {
            *str = append(*str, trimmed)
        }
    }
    return nil
}

func getColors(noColor bool) colors {
    var c colors
    if noColor {
        c.RESET  = ""
        c.GRAY 	 = ""
        c.RED 	 = ""
        c.YELLOW = ""
        c.CYAN 	 = ""
        c.BLUE 	 = ""
    } else {
        c.RESET  = "\033[0m"
        c.GRAY   = "\033[90m"
        c.RED 	 = "\033[91m"
        c.YELLOW = "\033[33m"
        c.CYAN 	 = "\033[0;36m"
        c.BLUE 	 = "\033[1;34m"
    }
    return c
}

func countLines(filename string) (int64, error) {
    file, err := os.Open(filename)
    if err != nil {
        return 0, err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var count int64
    for scanner.Scan() {
        count++
    }
    return count, scanner.Err()
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

func cloneRepo(url string, branch string, commit string, noColor bool) (string, error) {
    dir, err := os.MkdirTemp("", "countlines-")
    if err != nil {
        return "", err
    }
	color := getColors(noColor)

	var cloneArgs []string
    if commit == "" && branch != "" {
        cloneArgs = []string{"clone", "--depth", "1", "-b", branch, "--single-branch", url, dir}
    } else {
        cloneArgs = []string{"clone", url, dir}
    }

	fmt.Printf("%sCloning into '%s'...%s\n", color.GRAY, dir, color.RESET)
    cmd := exec.Command("git", cloneArgs...)
    if err := cmd.Run(); err != nil {
        fmt.Println(color.RED + "ERROR: Failed to clone repository.", color.RESET)
        os.RemoveAll(dir)
        return "", err
    }

	if branch != "" && commit != "" {
        cmdBranch := exec.Command("git", "-C", dir, "checkout", branch)
        if err := cmdBranch.Run(); err != nil {
            fmt.Println(color.RED + "ERROR: Failed to checkout branch.", color.RESET)
            os.RemoveAll(dir)
            return "", err
        }
    }

	if commit != "" {
        cmdCommit := exec.Command("git", "-C", dir, "checkout", commit)
        if err := cmdCommit.Run(); err != nil {
            fmt.Println(color.RED + "ERROR: Failed to checkout commit.", color.RESET)
            os.RemoveAll(dir)
            return "", err
        }
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

	var blacklist stringSlice
	outputCsv := flag.Bool("output-csv", false, "write output summary to output.csv file")
	noColor := flag.Bool("no-color", false, "disable colored output")
	branch := flag.String("branch", "", "git branch to clone (for GitHub repos)")
    commit := flag.String("commit", "", "git commit (SHA) to checkout (for GitHub repos)")
    flag.Var(&blacklist, "blacklist", "patterns of files or directories to exclude (comma separated)")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: countlines.exe [-flags] <directory/url> [pattern1] [pattern2] ...\n")
        os.Exit(1)
    }

	colors := getColors(*noColor)
	isUrl := false
	input := flag.Arg(0)
	var root string
	var patterns stringSlice
	if isGitHubRepo(input) {
		isUrl = true
		tmp, err := cloneRepo(input, *branch, *commit, *noColor)
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
	resultsChan := make(chan fileLineInfo, 100)

	// Worker goroutines to count lines
	for range make([]struct{}, runtime.NumCPU()) {
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
					fmt.Printf("%sRead file:%s %s - %s(%s)%s\n", colors.CYAN, colors.RESET, relPath, colors.YELLOW, formatNumber(lines), colors.RESET)
					resultsChan <- fileLineInfo{path: filename, lines: lines}
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

	linesByExt := make(map[string]int64)
	filesByExt := make(map[string]int64)
	namesUsed := make(map[string]struct{})
	getLabel := func(path string) string {
		ext := filepath.Ext(path)
		if ext == "" {
			return filepath.Base(path)
		}
		return ext
	}

	for fileInfo := range resultsChan {
		totalLines += fileInfo.lines
		totalFiles++

		label := getLabel(fileInfo.path)
		if _, exists := namesUsed[fileInfo.path]; !exists {
			linesByExt[label] += fileInfo.lines
			filesByExt[label]++
			namesUsed[fileInfo.path] = struct{}{}
		}
	}

	maxLen := 0
	for label := range linesByExt {
		if len(label) > maxLen {
			maxLen = len(label)
		}
	}

	fmt.Println(colors.BLUE + "File Count:", formatNumber(totalFiles))
	fmt.Println("Line Count:", formatNumber(totalLines), colors.RESET)

    fmt.Printf("\nLines by file extension:\n")
    exts := make([]string, 0, len(linesByExt))
	for label := range linesByExt {
		exts = append(exts, label)
	}
    sort.Strings(exts)
	formatStr := fmt.Sprintf("%s  %%-%ds %s: %%-12s %s(%%d files)%s\n", colors.CYAN, maxLen, colors.RESET, colors.YELLOW, colors.RESET)
    for _, label := range exts {
		fmt.Printf(formatStr, label, formatNumber(linesByExt[label]), filesByExt[label])
	}

	if *outputCsv {
        file, err := os.Create("output.csv")
        if err != nil {
            log.Fatalf("Failed to create output.csv: %v", err)
        }
        defer file.Close()

        writer := csv.NewWriter(file)
        defer writer.Flush()

        writer.Write([]string{"Total Files", formatNumber(totalFiles)})
        writer.Write([]string{"Total Lines", formatNumber(totalLines)})
        writer.Write([]string{})
        writer.Write([]string{"Extension/File", "Line Count", "File Count"})

        for _, label := range exts {
            writer.Write([]string{label, fmt.Sprintf("%d", linesByExt[label]), fmt.Sprintf("%d", filesByExt[label])})
        }
        fmt.Println("Output saved to output.csv")
    }

	if isUrl {
		fmt.Println(colors.GRAY + "Cloned repo has been deleted.", colors.RESET)
	}
}