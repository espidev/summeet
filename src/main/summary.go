package main

import (
	"fmt"
	"gopkg.in/neurosnap/sentences.v1/english"
	"log"
	"sort"
	"strings"
)

type sentenceEntry struct {
	weight float64
	s string
}

func getSummary (text string) string {

	tokenizer, err := english.NewSentenceTokenizer(nil)
	if err != nil {
		panic(err)
	}

	m := make(map[string]float64)

	sentences := tokenizer.Tokenize(text)
	maxN := 0.0
	for _, s := range sentences {
		str := s.Text
		str = strings.ReplaceAll(str, " and", "")
		str = strings.ReplaceAll(str, " the", "")
		str = strings.ReplaceAll(str, " a", "")
		str = strings.ReplaceAll(str, " was", "")
		str = strings.ReplaceAll(str, " in", "")

		for _, s := range strings.Fields(str) {
			s = strings.ToLower(s)
			m[s]++
			if maxN < m[s] {
				maxN = m[s]
			}
		}
	}

	for k, v := range m {
		m[k] = v/maxN
		log.Println(k + " " + fmt.Sprintf("%f", m[k]))
	}

	var lines []sentenceEntry
	for _, s := range sentences {
		score := 0.0
		for _, word := range strings.Fields(s.String()) {
			score += m[strings.ToLower(word)]
		}
		lines = append(lines, sentenceEntry{
			weight: score,
			s:      s.Text,
		})
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].weight < lines[i].weight
	})

	finalS := ""
	for _, v := range lines {
		finalS += v.s + "\n"
	}

	return finalS
}
