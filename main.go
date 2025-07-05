package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	goset "github.com/pydea-rs/goset"
)

var separatorSigns = goset.New(
	'~', '!', '@', '#', '$', '%', '^', '&',
	'*', '(', ')', '+', ',', '.', '/', ';',
	'\'', '"', ' ', '\t', '\n', '\r', '\\',
	'|', '{', '}', '[', ']', '<', '>',
	'?', '=', ':', '`',
)

func combine(word string, start int, k int, path []rune, results *[]string, excludingCombos goset.Set) {
	// If we have a full combination, add to results
	if len(path) == k {
		if !excludingCombos.Has(string(path)) {
			*results = append(*results, string(path))
		}
		return
	}

	for i := start; i < len(word); i++ {
		path = append(path, rune(word[i]))
		combine(word, i+1, k, path, results, excludingCombos)
		path = path[:len(path)-1]
	}
}

func (config *SearchConfig) GenerateCombinations() []string {
	input := strings.ToLower(config.TargetWord)
	first := input[0]
	rest := input[1:]
	var results []string

	for k := len(input); k >= 3; k-- {
		combine(rest, 0, k, []rune{rune(first)}, &results, config.ExcludingCombos)
	}

	return results
}

type SearchChannelData struct {
	Path  string
	Combo string
}

type WriterChannelData struct {
	Output string
	Ident  string
}

type SearchConfig struct {
	CaseSensitive       bool
	IncludeFilenames    bool
	IncludeFileContents bool
	RootAddress         string
	TargetWord          string
	WorkerLimit         uint16
	OutputFile          string
	WordByWordSearch    bool
	ExcludingCombos     goset.Set
}

func getDefaultSearchConfig(root string, word string) SearchConfig {
	return SearchConfig{
		CaseSensitive:       false,
		IncludeFilenames:    false,
		IncludeFileContents: true,
		RootAddress:         root,
		TargetWord:          word,
		WorkerLimit:         200,
		OutputFile:          "matches.txt",
		WordByWordSearch:    false,
		ExcludingCombos:     goset.New(),
	}
}

func (config *SearchConfig) LoadExtraArgs() {
	if argsCount := len(os.Args); argsCount > 3 {
		for arg := 3; arg < argsCount; arg++ {
			if limit, err := strconv.Atoi(os.Args[arg]); err == nil && limit > 0 {
				config.WorkerLimit = uint16(limit)
			} else if os.Args[arg] == "-w" {
				log.Println("Word by word search enabled")
				config.WordByWordSearch = true
			} else if os.Args[arg] == "-x" {
				for arg++; arg < argsCount && !strings.HasPrefix(os.Args[arg], "-") && !strings.HasPrefix(os.Args[arg], "+"); arg++ {
					config.ExcludingCombos.Add(strings.ToLower(os.Args[arg]))
				}
				if config.ExcludingCombos.Count() == 0 {
					log.Fatalln("No excluding combos specified: Usage: anyshape <directory> <word> [...] -x <combo1> <combo2> ... [...]")
				}
				arg--
			} else if os.Args[arg] == "+fn" {
				config.IncludeFilenames = true
			} else if os.Args[arg] == "+ct" {
				config.IncludeFileContents = true
			} else if os.Args[arg] == "-fn" {
				config.IncludeFilenames = false
			} else if os.Args[arg] == "-ct" {
				config.IncludeFileContents = false
			} else if os.Args[arg] == "-o" {
				if arg >= argsCount-1 {
					log.Fatalln("No output file specified: Usage: anyshape <directory> <word> [...] -o <output_file> [...]")
				}
				config.OutputFile = os.Args[arg+1]
				arg++
			} else {
				log.Fatalln("Unknown argument:", os.Args[arg])
			}
		}
	}
	if !config.IncludeFilenames && !config.IncludeFileContents {
		log.Fatalln("At least one of the following options must be enabled: +fn, +ct")
	}
}

func search(rootAddress string, searchChannel chan SearchChannelData, writerChannel chan WriterChannelData, waiter *sync.WaitGroup) {
	for data := range searchChannel {
		relPath, _ := filepath.Rel(rootAddress, data.Path)
		go func() {
			file, err := os.Open(data.Path)
			if err != nil {
				waiter.Add(1)
				writerChannel <- WriterChannelData{Output: fmt.Sprintf("Error reading file %s to match:%s : %v", relPath, data.Combo, err), Ident: ""}
				return
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 1
			comboLength := len(data.Combo)
			for scanner.Scan() {
				line := scanner.Text()
				lowerLine := strings.ToLower(line)

				idx := 0
				for {
					pos := strings.Index(lowerLine[idx:], data.Combo)
					if pos == -1 {
						break
					}
					start := idx + pos
					if start > 0 && !separatorSigns.Has(rune(lowerLine[start])) {
						for start > 0 && !separatorSigns.Has(rune(lowerLine[start-1])) {
							start--
						}
					}

					end := idx + pos + comboLength
					for lineLength := len(lowerLine); end < lineLength && !separatorSigns.Has(rune(lowerLine[end])); {
						end++
					}
					actual := lowerLine[start:end]
					waiter.Add(1)
					writerChannel <- WriterChannelData{
						Output: fmt.Sprintf("%s | %s | %s | line %d, char %d", data.Combo, actual, relPath, lineNum, idx+pos+1),
						Ident:  fmt.Sprintf("%s:%d:%d", relPath, lineNum, idx+pos+1),
					}
					idx += pos + 1
					if idx >= len(line) {
						break
					}
				}
				lineNum++
			}
			waiter.Done()
			if err := scanner.Err(); err != nil {
				waiter.Add(1)
				writerChannel <- WriterChannelData{Output: fmt.Sprintf("Error reading file %s to match:%s : %v", data.Path, data.Combo, err), Ident: ""}
			}
		}()
	}
}

func searchWordByWord(rootAddress string, searchChannel chan SearchChannelData, writerChannel chan WriterChannelData, waiter *sync.WaitGroup) {
	for data := range searchChannel {
		relPath, _ := filepath.Rel(rootAddress, data.Path)
		go func() {
			file, err := os.Open(data.Path)
			if err != nil {
				waiter.Add(1)
				writerChannel <- WriterChannelData{Output: fmt.Sprintf("Error reading file %s to match:%s : %v", relPath, data.Combo, err), Ident: ""}
				return
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 1
			for scanner.Scan() {
				line := scanner.Text()

				for cursor, word := range strings.Fields(line) {
					count := len(word)
					if separatorSigns.Has(rune(word[count-1])) {
						word = word[:count-1]
					}
					if strings.ToLower(word) == data.Combo {
						waiter.Add(1)
						writerChannel <- WriterChannelData{
							Output: fmt.Sprintf("%s | %s | line %d, word %d", word, relPath, lineNum, cursor+1),
							Ident:  fmt.Sprintf("%s:%d:%d", relPath, lineNum, cursor+1),
						}
					}
				}
				lineNum++
			}
			waiter.Done()
			if err := scanner.Err(); err != nil {
				waiter.Add(1)
				writerChannel <- WriterChannelData{Output: fmt.Sprintf("Error reading file %s to match:%s : %v", data.Path, data.Combo, err), Ident: ""}
			}
		}()
	}
}

func lookForMatches(config SearchConfig, searchChannel chan SearchChannelData,
	writerChannel chan WriterChannelData, waiter *sync.WaitGroup) []string {
	failedCombos := make([]string, 0)

	for _, combo := range config.GenerateCombinations() {
		log.Println("Searching for combo:", combo, "...")
		if err := filepath.Walk(config.RootAddress, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if config.IncludeFilenames {
				if strings.Contains(strings.ToLower(info.Name()), combo) {
					relativePath, err := filepath.Rel(config.RootAddress, path)
					if err != nil {
						relativePath = path
					}
					entityType := "File"
					ident := relativePath
					if info.IsDir() {
						ident += "#D"
						entityType = "Directory"
					} else {
						ident += "#F"
					}
					waiter.Add(1)
					writerChannel <- WriterChannelData{Output: fmt.Sprintf("%s | %s | %s", combo, relativePath, entityType), Ident: ident}
				}
			}
			if info.IsDir() {
				return nil
			}
			if config.IncludeFileContents {
				// TODO: Add case sensitive search logic too
				waiter.Add(1)
				searchChannel <- SearchChannelData{Path: path, Combo: combo}
			}
			return nil
		}); err != nil {
			failedCombos = append(failedCombos, combo)
		}
	}
	waiter.Wait()
	return failedCombos
}

func writeMatches(config SearchConfig, writerChannel chan WriterChannelData, waiter *sync.WaitGroup) {
	matchesFile, err := os.OpenFile(config.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Println("Error opening matches file:", err)
		return
	}
	defer matchesFile.Close()
	previousIdents := goset.New()
	for match := range writerChannel {
		if existed := !previousIdents.Add(match.Ident); !existed {
			if _, err := matchesFile.WriteString(match.Output + "\n"); err != nil {
				log.Println("Saving match:", match, "failed:", err)
			}
		}
		waiter.Done()
	}
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalln("Usage: anyshape <directory> <word>")
	}
	start := time.Now()
	config := getDefaultSearchConfig(os.Args[1], os.Args[2])
	config.LoadExtraArgs()

	writerChannel := make(chan WriterChannelData)
	waiter := new(sync.WaitGroup)
	go writeMatches(config, writerChannel, waiter)

	searchChannel := make(chan SearchChannelData, config.WorkerLimit)
	searchChannelCapacity := uint16(cap(searchChannel))

	if config.IncludeFileContents {
		for i := uint16(0); i < searchChannelCapacity; i++ {
			if config.WordByWordSearch { // the reason behind not combining these functions, is to prevent unnecessary search mode checks on each file and each combo again and again.
				go searchWordByWord(config.RootAddress, searchChannel, writerChannel, waiter)
			} else {
				go search(config.RootAddress, searchChannel, writerChannel, waiter)
			}
		}
	}

	if failedCombos := lookForMatches(config, searchChannel, writerChannel, waiter); len(failedCombos) > 0 {
		waiter.Add(1)
		writerChannel <- WriterChannelData{
			Output: fmt.Sprintln("\n- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - \nCombo's failed Matching:", len(failedCombos)),
			Ident:  "",
		}
		for _, combo := range failedCombos {
			waiter.Add(1)
			writerChannel <- WriterChannelData{Output: fmt.Sprintf("Failed to search for combo: %s", combo), Ident: ""}
		}
	}
	close(writerChannel)
	close(searchChannel)
	log.Println("Search Time:", time.Since(start).String())
}
