package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samber/lo"
)

func main() {
	scenarios, err := findStepsFiles(".")
	if err != nil {
		log.Fatalf("error finding steps files: %v", err)
	}

	printScenarios(scenarios)
}

type scenario struct {
	fullPath string
	section  string
	scenario string
}

func findStepsFiles(directory string) ([]scenario, error) {
	var scenarios []scenario

	visit := func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if info.Name() == "steps.md" {
			s := scenario{
				fullPath: path,
				section:  strings.Split(filepath.Clean(path), "/")[0],
				scenario: filepath.Base(filepath.Dir(path)),
			}
			scenarios = append(scenarios, s)
		}

		return nil
	}

	if err := filepath.Walk(directory, visit); err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return scenarios, nil
}

func printScenarios(scenarios []scenario) {
	groupedScenarios := lo.GroupBy(scenarios, func(s scenario) string {
		return s.section
	})

	var keys []string
	for k := range groupedScenarios {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	keys = lo.Uniq(keys)

	fmt.Printf("### Table of Contents\n")
	for _, key := range keys {
		fmt.Printf("\n##### %s\n", key)

		scenarios := groupedScenarios[key]
		for _, s := range scenarios {
			fmt.Printf("* [%s](%s)\n", s.scenario, s.fullPath)
		}
	}
}
