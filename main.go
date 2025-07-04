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

func generateCombinations(word string) []string {
	var combos []string
	word = strings.ToLower(word)
	n := len(word)
	for l := n; l >= 3; l-- {
		for i := 0; i <= n-l; i++ {
			combos = append(combos, word[i:i+l])
		}
	}
	return combos
}

type SearchChannelData struct {
	Path  string
	Combo string
}

func search(rootAddress string, searchChannel chan SearchChannelData, writerChannel chan string) {
	for data := range searchChannel {
		relPath, _ := filepath.Rel(rootAddress, data.Path)
		go func() {
			file, err := os.Open(data.Path)
			if err != nil {
				writerChannel <- fmt.Sprintf("Error reading file %s to match:%s : %v", relPath, data.Combo, err)
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
					actual := line[idx+pos : idx+pos+len(data.Combo)]
					output := fmt.Sprintf("%s | %s | line %d, char %d", actual, relPath, lineNum, idx+pos+1)
					writerChannel <- output
					idx += pos + 1
					if idx >= len(line) {
						break
					}
				}
				lineNum++
			}

			if err := scanner.Err(); err != nil {
				writerChannel <- fmt.Sprintf("Error reading file %s to match:%s : %v", data.Path, data.Combo, err)
			}
		}()
	}
}

func lookForMatches(rootAddress string, word string, searchChannel chan SearchChannelData) []string {
	failedCombos := make([]string, 0)
	for _, combo := range generateCombinations(word) {
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

func writeMatches(writerChannel chan string) {
	matchesFile, err := os.OpenFile("anyshape-matches.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Error opening matches file:", err)
		return
	}
	defer matchesFile.Close()

	for match := range writerChannel {
		if _, err := matchesFile.WriteString(match + "\n"); err != nil {
			fmt.Println("Saving match:", match, "failed:", err)
		}
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: anyshape <directory> <word>")
		return
	}
	root := os.Args[1]
	word := os.Args[2]
	workerLimit := 300

	if len(os.Args) > 3 {
		if limit, err := strconv.Atoi(os.Args[3]); err == nil && limit > 0 {
			workerLimit = limit
		}
	}
	writerChannel := make(chan string)
	go writeMatches(writerChannel)

	searchChannel := make(chan SearchChannelData, workerLimit)
	searchChannelCapacity := uint16(cap(searchChannel))

	for i := uint16(0); i < searchChannelCapacity; i++ {
		go search(root, searchChannel, writerChannel)
	}

	if failedCombos := lookForMatches(root, word, searchChannel); len(failedCombos) > 0 {

		writerChannel <- fmt.Sprintln("\n- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - \nCombo's failed Matching:", len(failedCombos))
		for _, combo := range failedCombos {
			writerChannel <- fmt.Sprintf("Failed to search for combo: %s", combo)
		}
	}
}
