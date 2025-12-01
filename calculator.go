package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	_ "github.com/joho/godotenv/autoload"
)

// *****************************************************************************************
// This function calculates the deck and creates the estimate
//
//	Options:
//	      deck -
//	      rails -
//	      stairs -
//	      demo -
//
// *****************************************************************************************
func calcHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	option := query.Get("option")

	log.Printf("Calc Option is %s", option)

	// Validate
	// if option == "" {
	//     http.Error(w, "Missing 'option' parameter. Use: ?option=deck|rails|stairs|demo", http.StatusBadRequest)
	//     return
	// }

	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Load estimate from session
	estimate := DeckEstimate{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		estimate = est
	}

	// Success: Route to correct calculator
	switch option {
	case "deck":
		handleDeckCalc(w, r, estimate)
	case "rails":
		handleRailsCalc(w, r, estimate)
	case "stairs":
		handleStairsCalc(w, r)
	case "demo":
		handleDemoCalc(w, r)
	default:
		handleFullCalc(w, r, estimate)
	}
}

func handleFullCalc(w http.ResponseWriter, r *http.Request, e DeckEstimate) {
	userAuth := getUserAuth(r)
	rd := renderData{
		Page:   &e,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("calculator.html").Funcs(funcMap).ParseFiles("templates/calculator.html",
		"templates/header.html", "templates/footer.html"))

	if err := tmpl.ExecuteTemplate(w, "calculator.html", rd); err != nil {
		log.Printf("handleFullCalc execute error: %v", err)
		panic(err)
	}
}

// Example handlers — expand with real logic
func handleDeckCalc(w http.ResponseWriter, r *http.Request, e DeckEstimate) {
	userAuth := getUserAuth(r)
	rd := renderData{
		Page:   &e,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("deck.html").Funcs(funcMap).ParseFiles("templates/calc/deck.html",
		"templates/header.html", "templates/calc/deckheader.html", "templates/footer.html"))

	if err := tmpl.ExecuteTemplate(w, "deck.html", rd); err != nil {
		log.Printf("handleDeckCalc execute error: %v", err)
		panic(err)
	}
}

func handleRailsCalc(w http.ResponseWriter, r *http.Request, e DeckEstimate) {
	userAuth := getUserAuth(r)
	rd := renderData{
		Page:   &e,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("rails.html").Funcs(funcMap).ParseFiles("templates/calc/rails.html",
		"templates/header.html", "templates/calc/deckheader.html", "templates/footer.html"))

	if err := tmpl.ExecuteTemplate(w, "rails.html", rd); err != nil {
		log.Printf("*** handleRailsCalc *** execute error: %v", err)
		panic(err)
	}
}

func handleStairsCalc(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Stairs Calculator\n")
	fmt.Fprintf(w, "→ Rise/run validation per WA/OR/ID code\n")
	fmt.Fprintf(w, "→ Material delivery: pressure-treated or composite\n")
}

func handleDemoCalc(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Demolition Estimate\n")
	fmt.Fprintf(w, "→ Safe removal of old deck/patio\n")
	fmt.Fprintf(w, "→ Waste haul included\n")
	fmt.Fprintf(w, "→ Site prep for new build\n")
}
