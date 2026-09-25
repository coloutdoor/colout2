package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CoverDescription holds the construction spec for one patio cover type,
// loaded from static/cover_descriptions.yaml.
type CoverDescription struct {
	Type     string `yaml:"type"`
	Posts    string `yaml:"posts"`
	Headers  string `yaml:"headers"`
	Framing  string `yaml:"framing"`
	RoofDeck string `yaml:"roof-deck"`
	Roof     string `yaml:"roof"`
	Fascia   string `yaml:"fascia"`
	Gutters  string `yaml:"gutters"`
}

// coverDescriptions maps PatioType (e.g. "leanto") to its construction spec,
// loaded at startup.
var coverDescriptions map[string]CoverDescription

// loadCoverDescriptions reads and parses cover_descriptions.yaml into the map.
func loadCoverDescriptions() error {
	data, err := os.ReadFile("static/cover_descriptions.yaml")
	if err != nil {
		return fmt.Errorf("failed to read cover_descriptions.yaml: %v", err)
	}
	var list []CoverDescription
	if err := yaml.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("failed to parse cover_descriptions.yaml: %v", err)
	}
	coverDescriptions = make(map[string]CoverDescription, len(list))
	for _, cd := range list {
		coverDescriptions[cd.Type] = cd
	}
	return nil
}

// Summary joins the construction spec fields into one sentence, skipping any
// field left blank or set to "N/A" (e.g. pergola has no roof deck).
func (cd CoverDescription) Summary() string {
	var parts []string
	add := func(label, value string) {
		if value != "" && !strings.EqualFold(value, "N/A") {
			parts = append(parts, label+": "+value)
		}
	}
	add("Posts", cd.Posts)
	add("Headers", cd.Headers)
	add("Framing", cd.Framing)
	add("Roof deck", cd.RoofDeck)
	add("Roof", cd.Roof)
	add("Fascia", cd.Fascia)
	add("Gutters", cd.Gutters)
	return strings.Join(parts, "\n")
}
