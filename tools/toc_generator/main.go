package main

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samber/lo"
)

func main() {
	scenarios, err := findStepsFiles()
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

func findStepsFiles() ([]scenario, error) {
	var scenarios []scenario

	matches, err := filepath.Glob("**/*/steps.md")
	if err != nil {
		log.Fatal(err)
	}

	for _, match := range matches {
		s := scenario{
			fullPath: match,
			section:  strings.Split(filepath.Clean(match), "/")[0],
			scenario: filepath.Base(filepath.Dir(match)),
		}
		scenarios = append(scenarios, s)
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
