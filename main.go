package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"

	_ "github.com/joho/godotenv/autoload"
)

type Costs struct {
	DeckMaterials map[string]float64 `yaml:"deck_materials"`
	RailMaterials map[string]float64 `yaml:"rail_materials"`
	RailInfills   map[string]float64 `yaml:"rail_infills"`
}

var costs Costs

func loadCosts() error {
	data, err := os.ReadFile("costs.yaml")
	if err != nil {
		return fmt.Errorf("failed to read costs.yaml: %v", err)
	}
	if err := yaml.Unmarshal(data, &costs); err != nil {
		return fmt.Errorf("failed to parse costs.yaml: %v", err)
	}
	return nil
}

type DeckEstimate struct {
	Length       float64
	Width        float64
	Height       float64
	Material     string
	RailMaterial string
	RailInfill   string
	TotalCost    float64
	DeckCost     float64 // Split for breakdown
	RailCost     float64
	Error        string
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, nil)
}

func estimateHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	if r.Method != http.MethodPost {
		tmpl.Execute(w, nil)
		return
	}
	height, err := strconv.ParseFloat(r.FormValue("height"), 64)
	if err != nil || height < 0 {
		tmpl.Execute(w, DeckEstimate{Error: "Height must be a non-negative number"})
		return
	}

	length, _ := strconv.ParseFloat(r.FormValue("length"), 64)
	width, _ := strconv.ParseFloat(r.FormValue("width"), 64)
	material := r.FormValue("material")
	railMaterial := r.FormValue("railMaterial")
	railInfill := r.FormValue("railInfill")

	estimate := DeckEstimate{
		Length:       length,
		Width:        width,
		Height:       height,
		Material:     material,
		RailMaterial: railMaterial,
		RailInfill:   railInfill,
	}

	// Deck cost
    deckCost, err := CalculateDeckCost(length, width, height, material, costs)
    if err != nil {
        estimate.Error = err.Error()
        tmpl.Execute(w, estimate)
        return
    }
    estimate.DeckCost = deckCost

	// Rail cost
	railCost, err := CalculateRailCost(length, width, railMaterial, railInfill, costs)
	if err != nil {
		estimate.Error = err.Error()
		tmpl.Execute(w, estimate)
		return
	}
	estimate.RailCost = railCost
	estimate.TotalCost = estimate.DeckCost + estimate.RailCost

	// Debug output to console
	fmt.Printf("DeckCost: $%.2f, RailCost: $%.2f, TotalCost: $%.2f, RailMaterial: %s, RailInfill: %s\n",
		estimate.DeckCost, estimate.RailCost, estimate.TotalCost, estimate.RailMaterial, estimate.RailInfill)

	tmpl.Execute(w, estimate)
}

func main() {
	if err := loadCosts(); err != nil {
		fmt.Println("Error loading costs:", err)
		os.Exit(1)
	}
	devMode := flag.Bool("dev", false, "Run in development mode (localhost only)")
	flag.Parse()

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/estimate", estimateHandler)
	//fmt.Println("Server starting on :8080...")
	// err := http.ListenAndServe(":8080", nil)
	addr := ":8080"
	if envAddr := os.Getenv("SERVER_ADDR"); envAddr != "" {
		addr = envAddr
		fmt.Printf("Server starting on %s (from env)...\n", addr)
	} else if *devMode {
		addr = "127.0.0.1:8080"
		fmt.Println("Server starting on localhost:8080 (dev mode)...")
	} else {
		fmt.Println("Default Server starting on :8080...")
	}
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
