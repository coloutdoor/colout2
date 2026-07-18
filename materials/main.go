package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/joho/godotenv"
)

type section struct {
	key       string
	title     string
	rulesFile string
}

var sections = []section{
	{"footings", "Footings & Posts", "rules/footing_rules.txt"},
	{"framing", "Framing", "rules/framing_rules.txt"},
	{"decking", "Decking & Trim", "rules/decking_rules.txt"},
	{"rails", "Rails", "rules/rails_rules.txt"},
}

type result struct {
	index  int
	title  string
	output string
	err    error
}

func printHelp() {
	fmt.Println(`Usage: Build_Material <estimate_file> [--section <name>]

Arguments:
  <estimate_file>       Path to the estimate text file (required)
  --section <name>      Run only one section (optional)
  --help                Show this help message

Sections:
  footings              Footings, concrete, posts, headers
  framing               Joists, ledger, hangers, fasteners
  decking               Deck boards, fascia, stair treads
  rails                 Rail posts, infill, fasteners

Examples:
  ./Build_Material test/estimate_1031.txt
  ./Build_Material test/estimate_1031.txt --section framing
  ./Build_Material test/estimate_1031.txt --section decking`)
}

func main() {
	_ = godotenv.Load("../.env")

	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printHelp()
		return
	}

	estimateFile := os.Args[1]

	// Parse optional --section flag
	var sectionFilter string
	for i := 2; i < len(os.Args)-1; i++ {
		if os.Args[i] == "--section" {
			sectionFilter = strings.ToLower(os.Args[i+1])
		}
	}

	// Validate section name if provided
	if sectionFilter != "" {
		valid := false
		for _, s := range sections {
			if s.key == sectionFilter {
				valid = true
				break
			}
		}
		if !valid {
			fmt.Printf("Unknown section %q. Valid sections: footings, framing, decking, rails\n", sectionFilter)
			os.Exit(1)
		}
	}

	estimateBytes, err := os.ReadFile(estimateFile)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", estimateFile, err)
	}
	estimateText := string(estimateBytes)

	// Build list of sections to run
	var toRun []section
	for _, s := range sections {
		if sectionFilter == "" || s.key == sectionFilter {
			toRun = append(toRun, s)
		}
	}

	ctx := context.Background()
	client := anthropic.NewClient()

	results := make([]result, len(toRun))
	var wg sync.WaitGroup

	for i, s := range toRun {
		wg.Add(1)
		go func(i int, s section) {
			defer wg.Done()
			results[i] = callClaude(ctx, client, i, s, estimateText)
		}(i, s)
	}

	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			fmt.Printf("## %s\n\nERROR: %v\n\n", r.title, r.err)
			continue
		}
		fmt.Printf("## %s\n\n%s\n\n", r.title, r.output)
	}
}

func callClaude(ctx context.Context, client anthropic.Client, index int, s section, estimateText string) result {
	rulesBytes, err := os.ReadFile(s.rulesFile)
	if err != nil {
		return result{index: index, title: s.title, err: fmt.Errorf("read %s: %w", s.rulesFile, err)}
	}

	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: string(rulesBytes)}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(estimateText)),
		},
	})
	if err != nil {
		return result{index: index, title: s.title, err: err}
	}

	return result{index: index, title: s.title, output: msg.Content[0].Text}
}
