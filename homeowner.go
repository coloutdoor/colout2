package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

type Photo struct {
	URI         string `yaml:"uri"`
	Category    string `yaml:"category"`
	City        string `yaml:"city"`
	Description string `yaml:"description"`
}

type PhotoLibrary struct {
	Photos []Photo `yaml:"photos"`
}

func loadPhotos() []Photo {
	data, err := os.ReadFile("static/photos.yaml")
	if err != nil {
		log.Printf("loadPhotos: %v", err)
		return nil
	}
	var lib PhotoLibrary
	if err := yaml.Unmarshal(data, &lib); err != nil {
		log.Printf("loadPhotos unmarshal: %v", err)
		return nil
	}
	return lib.Photos
}

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

	if r.URL.Path != "/" {
		notFoundHandler(w, r)
		return
	}

	// fallback to normal Homeowner
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Decks & Outdoor Living"
	userAuth.Subtitle = "Quality decks and outdoor structures built right. Transparent pricing, expert craftsmanship."
	userAuth.MetaDesc = "Columbia Outdoor builds quality decks, patios, and outdoor structures across SW Washington. Transparent pricing, experienced builders, and expert project management."
	userAuth.CanonicalPath = "/"
	photos := loadPhotos()
	rd := renderData{
		Page:   photos,
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
