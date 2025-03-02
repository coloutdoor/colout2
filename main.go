package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
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
	DemoCost      float64            `yaml:"demo_cost"`
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
	Length        float64
	Width         float64
	Height        float64
	DeckArea      float64
	Material      string
	RailMaterial  string
	RailInfill    string
	TotalCost     float64
	DeckCost      float64 // Split for breakdown
	RailCost      float64
	StairCost     float64
	Subtotal      float64
	DemoCost      float64
	HasDemo       bool
	StairWidth    float64
	StairRailCost float64
	RailFeet      float64 // Lineal feet of rails
	SalesTax      float64 // TODO Dynamic lookup
	Error         string
}

// Define template functions
var funcMap = template.FuncMap{
	"formatCost": formatCost,
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// tmpl := template.Must(template.ParseFiles("templates/index.html"))
	// tmpl := template.Must(template.ParseFiles("templates/index.html").Funcs(funcMap))
	tmpl := template.Must(template.New("index").Funcs(funcMap).ParseFiles("templates/index.html"))
	if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		log.Printf("homeHandler execute error: %v", err)
		panic(err)
	}
}

func estimateHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("estimate").Funcs(funcMap).ParseFiles("templates/estimate.html"))

	if r.Method != http.MethodPost {
		tmpl.ExecuteTemplate(w, "estimate.html", nil)
	}

	height, err := strconv.ParseFloat(r.FormValue("height"), 64)
	if err != nil || height < 0 {
		tmpl.ExecuteTemplate(w, "estimate.html", DeckEstimate{Error: "Height must be a non-negative number"})
		return
	}

	stairWidth, err := strconv.ParseFloat(r.FormValue("stairWidth"), 64)
	if err != nil || stairWidth < 0 {
		stairWidth = 0 // Default to 0 if invalid or not provided
	}
	fmt.Printf("stairWidth is set to: %.2f\n", stairWidth)

	length, _ := strconv.ParseFloat(r.FormValue("length"), 64)
	width, _ := strconv.ParseFloat(r.FormValue("width"), 64)
	material := r.FormValue("material")
	railMaterial := r.FormValue("railMaterial")
	railInfill := r.FormValue("railInfill")
	hasDemo := r.FormValue("hasDemo") == "on" // Checkbox returns "on" if checked

	estimate := DeckEstimate{
		Length:       length,
		Width:        width,
		Height:       height,
		Material:     material,
		RailMaterial: railMaterial,
		RailInfill:   railInfill,
		DeckArea:     length * width,
		HasDemo:      hasDemo,
		StairWidth:   stairWidth,
	}

	// Deck cost
	deckCost, err := CalculateDeckCost(length, width, height, material, costs)
	if err != nil {
		estimate.Error = err.Error()
		tmpl.ExecuteTemplate(w, "estimate.html", estimate)
		return
	}
	estimate.DeckCost = deckCost

	// Rail cost
	railCost, err := CalculateRailCost(length, width, railMaterial, railInfill, costs)
	if err != nil {
		estimate.Error = err.Error()
		tmpl.ExecuteTemplate(w, "estimate.html", estimate)
		return
	}

	// Demo Cost - if required
	if estimate.HasDemo {
		estimate.DemoCost = CalculateDemoCost(length*width, costs)
	} else {
		estimate.DemoCost = 0
	}

	// This is used by the stair calculator
	materialCost, ok := costs.DeckMaterials[material]
	if !ok { // Shouldn’t hit this—deck calc already checks
		materialCost = 0
	}
	stairCost, err := CalculateStairCost(height, stairWidth, materialCost)
	if err != nil {
		estimate.Error = err.Error()
		tmpl.ExecuteTemplate(w, "estimate.html", estimate)
		return
	}
	estimate.RailCost = railCost
	estimate.RailFeet = (2 * length) + width // Match rails.go calc
	estimate.StairCost = stairCost
	estimate.StairRailCost = CalculateStairRailCost(height, railMaterial, costs)
	estimate.Subtotal = estimate.DeckCost + estimate.RailCost + estimate.StairCost + estimate.StairRailCost + estimate.DemoCost
	estimate.SalesTax = CalculateSalesTax(estimate.Subtotal)
	estimate.TotalCost = estimate.Subtotal + estimate.SalesTax

	/* Debug output to console */
	fmt.Printf("Debug:  Estimate: %+v\n", estimate)

	tmpl.ExecuteTemplate(w, "estimate.html", estimate)
}

func main() {
	if err := loadCosts(); err != nil {
		fmt.Println("Error loading costs:", err)
		os.Exit(1)
	}
	devMode := flag.Bool("dev", false, "Run in development mode (localhost only)")
	flag.Parse()

	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
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
