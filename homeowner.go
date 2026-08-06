package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

// Homeowner represents the structure of the homeowner marketing strategy
type Homeowner struct {
	Objective           string     `yaml:"objective"`
	TargetAudience      string     `yaml:"target_audience"`
	KeyValueProposition string     `yaml:"key_value_proposition"`
	Strategies          []Strategy `yaml:"strategies"`
}

// Strategy represents each marketing strategy with its details
type Strategy struct {
	Strategy   string   `yaml:"strategy"`
	Messaging  string   `yaml:"messaging"`
	PainPoints []string `yaml:"pain_points"`
	Advantage  string   `yaml:"advantage"`
}

// RenderData
type renderData struct {
	Page   any
	Header any // or *HeaderData
	// Footer any
}

// ownerStrategy
//
//	This is the main page for Homeowner - LandingPage
//	This was created from Bulma Templates
func ownerHandler(w http.ResponseWriter, r *http.Request) {

	// City specific landing pages ...
	tmpPath := strings.ToLower(r.URL.Path)
	if strings.HasPrefix(tmpPath, "/deck-builders-") ||
		strings.HasPrefix(tmpPath, "/patio-cover-") ||
		strings.HasPrefix(tmpPath, "/trex-deck-") ||
		strings.HasPrefix(tmpPath, "/timbertech-deck-") ||
		strings.HasPrefix(tmpPath, "/composite-decking-") ||
		strings.HasPrefix(tmpPath, "/outdoor-kitchen-builders-") ||
		strings.HasPrefix(tmpPath, "/pergola-builders-") ||
		strings.HasPrefix(tmpPath, "/outdoor-living-") {
		//	log.Printf("We got a city request... %s", tmpPath)
		cityHandler(w, r)
		return
	} else if r.URL.Path != "/" {
		// This is a 404
		notFoundHandler(w, r)
		return
	}

	// fallback to normal Homeowner
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Decks & Outdoor Living"
	userAuth.Subtitle = "Quality decks and outdoor structures built right. Transparent pricing, expert craftsmanship."
	userAuth.MetaDesc = "Columbia Outdoor builds quality decks, patios, and outdoor structures across SW Washington. Transparent pricing, experienced builders, and expert project management."
	rd := renderData{
		Page:   nil,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("homeowner.gohtml").Funcs(funcMap).
		ParseFiles("templates/homeowner.gohtml", "templates/header.gohtml", "templates/footer.gohtml"))

	if err := tmpl.ExecuteTemplate(w, "homeowner.gohtml", rd); err != nil {
		log.Printf("ownerHandler execute error: %v", err)
		panic(err)
	}
}

func debugStrategy(homeowner Homeowner) {
	// Print the parsed data to verify
	fmt.Printf("Objective: %s\n", homeowner.Objective)
	fmt.Printf("Target Audience: %s\n", homeowner.TargetAudience)
	fmt.Printf("Key Value Proposition: %s\n", homeowner.KeyValueProposition)
	fmt.Println("Strategies:")
	for i, strategy := range homeowner.Strategies {
		fmt.Printf("Strategy %d:\n", i+1)
		fmt.Printf("  Strategy: %s\n", strategy.Strategy)
		fmt.Printf("  Messaging: %s\n", strategy.Messaging)
		fmt.Println("  Pain Points:")
		for j, point := range strategy.PainPoints {
			fmt.Printf("    %d. %s\n", j+1, point)
		}
		fmt.Printf("  Advantage: %s\n", strategy.Advantage)
	}
}
