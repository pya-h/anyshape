package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func combine(word string, start int, k int, path []rune, results *[]string, excludingCombos Set) {
	// If we have a full combination, add to results
	if len(path) == k {
		if !excludingCombos.Includes(string(path)) {
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

func generateCombinations(input string, excludingCombos Set) []string {
	input = strings.ToLower(input)
	first := input[0]
	rest := input[1:]
	var results []string

	for k := len(input); k >= 3; k-- {
		combine(rest, 0, k, []rune{rune(first)}, &results, excludingCombos)
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

type Set map[string]struct{}
type SetItem struct{}

func (s Set) Add(item string) bool {
	if _, exists := s[item]; !exists {
		s[item] = SetItem{}
		return true
	}
	return false
}

func (s Set) Includes(item string) bool {
	_, exists := s[item]
	return exists
}

func NewSet() Set {
	return make(Set)
}

func search(rootAddress string, searchChannel chan SearchChannelData, writerChannel chan WriterChannelData) {
	for data := range searchChannel {
		relPath, _ := filepath.Rel(rootAddress, data.Path)
		go func() {
			file, err := os.Open(data.Path)
			if err != nil {
				writerChannel <- WriterChannelData{Output: fmt.Sprintf("Error reading file %s to match:%s : %v", relPath, data.Combo, err), Ident: ""}
				return
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 1
			for scanner.Scan() {
				line := scanner.Text()
				lowerLine := strings.ToLower(line)

				idx := 0
				for {
					pos := strings.Index(lowerLine[idx:], data.Combo)
					if pos == -1 {
						break
					}
					actual := lowerLine[idx+pos : idx+pos+len(data.Combo)]
					writerChannel <- WriterChannelData{
						Output: fmt.Sprintf("%s | %s | line %d, char %d", actual, relPath, lineNum, idx+pos+1),
						Ident:  fmt.Sprintf("%s:%d:%d", relPath, lineNum, idx+pos+1),
					}
					idx += pos + 1
					if idx >= len(line) {
						break
					}
				}
				lineNum++
			}

			if err := scanner.Err(); err != nil {
				writerChannel <- WriterChannelData{Output: fmt.Sprintf("Error reading file %s to match:%s : %v", data.Path, data.Combo, err), Ident: ""}
			}
		}()
	}
}

func searchWordByWord(rootAddress string, searchChannel chan SearchChannelData, writerChannel chan WriterChannelData) {
	for data := range searchChannel {
		relPath, _ := filepath.Rel(rootAddress, data.Path)
		go func() {
			file, err := os.Open(data.Path)
			if err != nil {
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
					if word[count-1] == '.' || word[count-1] == ',' || word[count-1] == ';' || word[count-1] == ':' {
						word = word[:count-1]
					}
					if strings.ToLower(word) == data.Combo {
						writerChannel <- WriterChannelData{
							Output: fmt.Sprintf("%s | %s | line %d, word %d", word, relPath, lineNum, cursor+1),
							Ident:  fmt.Sprintf("%s:%d:%d", relPath, lineNum, cursor+1),
						}
					}
				}
				lineNum++
			}

			if err := scanner.Err(); err != nil {
				writerChannel <- WriterChannelData{Output: fmt.Sprintf("Error reading file %s to match:%s : %v", data.Path, data.Combo, err), Ident: ""}
			}
		}()
	}
}

func lookForMatches(rootAddress string, searchChannel chan SearchChannelData, combos []string) []string {
	failedCombos := make([]string, 0)
	for _, combo := range combos {
		log.Println("Searching for combo:", combo, "...")
		if err := filepath.Walk(rootAddress, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			searchChannel <- SearchChannelData{Path: path, Combo: combo}
			return nil
		}); err != nil {
			failedCombos = append(failedCombos, combo)
		}
	}
	return failedCombos
}

func writeMatches(writerChannel chan WriterChannelData) {
	matchesFile, err := os.OpenFile("anyshape-matches.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error opening matches file:", err)
		return
	}
	defer matchesFile.Close()
	previousIdents := NewSet()
	for match := range writerChannel {
		if existed := !previousIdents.Add(match.Ident); !existed {
			if _, err := matchesFile.WriteString(match.Output + "\n"); err != nil {
				log.Println("Saving match:", match, "failed:", err)
			}
		}
	}
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalln("Usage: anyshape <directory> <word>")
	}
	root := os.Args[1]
	word := os.Args[2]
	workerLimit := 300
	wordByWordSearch := false
	excludingCombos := make(Set)
	if argsCount := len(os.Args); argsCount > 3 {
		for arg := 3; arg < argsCount; arg++ {
			if limit, err := strconv.Atoi(os.Args[arg]); err == nil && limit > 0 {
				workerLimit = limit
			} else if os.Args[arg] == "-w" {
				log.Println("Word by word search enabled")
				wordByWordSearch = true
			} else if os.Args[arg] == "-x" {
				for ; arg < argsCount && !strings.HasPrefix(os.Args[arg], "-"); arg++ {
					excludingCombos.Add(strings.ToLower(os.Args[arg]))
				}
			} else {
				log.Fatalln("Unknown argument:", os.Args[arg])
			}
		}
	}
	writerChannel := make(chan WriterChannelData)
	go writeMatches(writerChannel)

	searchChannel := make(chan SearchChannelData, workerLimit)
	searchChannelCapacity := uint16(cap(searchChannel))

	for i := uint16(0); i < searchChannelCapacity; i++ {
		if wordByWordSearch { // the reason behind not combining these functions, is to prevent unnecessary search mode checks on each file and each combo again and again.
			go searchWordByWord(root, searchChannel, writerChannel)
		} else {
			go search(root, searchChannel, writerChannel)
		}
	}
	combinations := generateCombinations(word, excludingCombos)
	if failedCombos := lookForMatches(root, searchChannel, combinations); len(failedCombos) > 0 {
		writerChannel <- WriterChannelData{Output: fmt.Sprintln("\n- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - \nCombo's failed Matching:", len(failedCombos)), Ident: ""}
		for _, combo := range failedCombos {
			writerChannel <- WriterChannelData{Output: fmt.Sprintf("Failed to search for combo: %s", combo), Ident: ""}
		}
	}
}
