package main

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed data/decensor.csv
var decensorCSV string

var (
	decensorOnce  sync.Once
	decensorPairs [][2]string
)

func initDecensor() {
	decensorOnce.Do(func() {
		lines := strings.Split(strings.TrimSpace(decensorCSV), "\n")
		decensorPairs = make([][2]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) != 2 {
				continue
			}
			decensorPairs = append(decensorPairs, [2]string{parts[0], parts[1]})
		}
	})
}

// decensor replaces censored terms in the input string with their uncensored equivalents.
func decensor(s string) string {
	initDecensor()
	for _, pair := range decensorPairs {
		s = strings.ReplaceAll(s, pair[0], pair[1])
	}
	return s
}

// decensorPtr applies decensor to a *string, returning nil if input is nil.
func decensorPtr(s *string) *string {
	if s == nil {
		return nil
	}
	result := decensor(*s)
	return &result
}
