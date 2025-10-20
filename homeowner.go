package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

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

// ownerStrategy
//
//	This is the main page for Homeowner - LandingPage
//	This was created from Bulma Templates
func ownerHandler(w http.ResponseWriter, r *http.Request) {
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

	tmpl := template.Must(template.New("homeowner.html").Funcs(funcMap).ParseFiles("templates/homeowner.html"))
	if err := tmpl.ExecuteTemplate(w, "homeowner.html", homeowner); err != nil {
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
