package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
)

type DeckEstimate struct {
	Length    float64
	Width     float64
	Height    float64
	Material  string
	TotalCost float64
	Error     string
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

	estimate := DeckEstimate{
		Length:   length,
		Width:    width,
		Height:   height,
		Material: material,
	}

	// Simple cost calculation (example rates per sq ft)
	area := length * width
	var costPerSqFt float64
	switch material {
	case "outdoorWood":
		costPerSqFt = 30.0
	case "cedar":
		costPerSqFt = 40.0
	case "timberTechPrime":
		costPerSqFt = 40.0
	case "timberTechProReserve":
		costPerSqFt = 45.0
	case "timberTechProLegacy":
		costPerSqFt = 50.0
	default:
		estimate.Error = "Please select a valid material."
		tmpl.Execute(w, estimate)
		return
	}
	// estimate.TotalCost = area * costPerSqFt

	baseCost := area * costPerSqFt

	// Height adjustment
	if height >= 20 {
		estimate.Error = "We can't build decks 20 feet or higher without additional information."
		tmpl.Execute(w, estimate)
		return
	} else if height > 4 {
		excessHeight := height - 4
		multiplier := 1 + (excessHeight * 0.01) // 1% per foot over 4
		estimate.TotalCost = baseCost * multiplier
	} else {
		estimate.TotalCost = baseCost
	}
	tmpl.Execute(w, estimate)
}

func main() {
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
