package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
)

type DeckEstimate struct {
	Length    float64
	Width     float64
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

	length, _ := strconv.ParseFloat(r.FormValue("length"), 64)
	width, _ := strconv.ParseFloat(r.FormValue("width"), 64)
	material := r.FormValue("material")

	estimate := DeckEstimate{
		Length:   length,
		Width:    width,
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
	estimate.TotalCost = area * costPerSqFt
	tmpl.Execute(w, estimate)
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/estimate", estimateHandler)
	fmt.Println("Server starting on :8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
