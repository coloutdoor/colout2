package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var doDebug = false

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
	}

	// fallback to normal Homeowner

	// Read the YAML file
	data, err := os.ReadFile("static/homeowner.yaml")
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// Unmarshal YAML into Homeowner struct
	var homeowner Homeowner
	err = yaml.Unmarshal(data, &homeowner)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML: %v", err)
	}

	if doDebug {
		debugStrategy(homeowner)
	}

	userAuth := getUserAuth(r)
	userAuth.Title = "Decks"
	userAuth.Subtitle = "A trusted solution for your Outdoor Living - Decks, Patios, Covers."
	userAuth.MetaDesc = "Skip the 3-bid hassle. Columbia Outdoor delivers your dream deck, patio cover, or landscape with, fixed pricing, permits handled, and a dedicated project manager."
	rd := renderData{
		Page:   &homeowner,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("homeowner.html").Funcs(funcMap).
		ParseFiles("templates/homeowner.html", "templates/header.html", "templates/footer.html"))

	if err := tmpl.ExecuteTemplate(w, "homeowner.html", rd); err != nil {
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
