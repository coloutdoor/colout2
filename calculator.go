package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
)

// *****************************************************************************************
// calcHandler
//
// This handler routes to a specific handler:
//
//	  /calc?option=deck
//
//		Options:
//		      deck -
//		      rails -
//		      stairs -
//		      demo -
//
// *****************************************************************************************
func calcHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	option := query.Get("option")

	log.Printf("Calc Option is %s", option)

	// Get session
	sessionData, err := GetSession(r, w)
	if err != nil {
		http.Error(w, "Session error - Calculator", http.StatusInternalServerError)
		return
	}

	// Load estimate from session
	estimate := sessionData.Estimate
	if estimate.Desc != "" {
		log.Printf("Using previous estimate for values from: %s", estimate.Desc)
	} else {
		log.Printf("New Estimate Calculator.")
	}

	// Carry DIY mode from URL into the estimate for the calculator template
	if dm := query.Get("diyMode"); dm != "" {
		if v, err := strconv.Atoi(dm); err == nil && v >= 0 && v <= 2 {
			estimate.DIYMode = v
		}
	}

	// Success: Route to correct calculator
	switch option {
	case "deck":
		handleDeckCalc(w, r, estimate)
	case "rails":
		handleRailsCalc(w, r, estimate)
		/*
			case "stairs":
				handleStairsCalc(w, r)
			case "demo":
				handleDemoCalc(w, r)
		*/
	default:
		handleFullCalc(w, r, estimate)
	}
}

// handleFullCalc This handler is the full - Detailed Deck estimate
func handleFullCalc(w http.ResponseWriter, r *http.Request, e DeckEstimate) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Deck Calculator Details"
	rd := renderData{
		Page:   &e,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("calculator.gohtml").Funcs(funcMap).ParseFiles("templates/calculator.gohtml",
		"templates/header.gohtml", "templates/footer.gohtml"))

	log.Printf("Calculator template loaded")
	if err := tmpl.ExecuteTemplate(w, "calculator.gohtml", rd); err != nil {
		log.Printf("handleFullCalc execute error: %v", err)
		panic(err)
	}
	log.Printf("Calculator template complete")
}

// *****************************************************************************************
//
//	handleDeckCalc
//
//	  This handler is base deck handler.
//
// *****************************************************************************************
func handleDeckCalc(w http.ResponseWriter, r *http.Request, e DeckEstimate) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Deck Calculator"
	userAuth.Subtitle = "Free and easy Estimate Calculator"
	userAuth.MetaDesc = "Free deck estimate calculator. Simple and easy to use for decks in SW Washington. Select deck, rails, stairs, and details."
	rd := renderData{
		Page:   &e,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("deck.gohtml").Funcs(funcMap).ParseFiles("templates/calc/deck.gohtml",
		"templates/header.gohtml", "templates/calc/deckheader.gohtml", "templates/footer.gohtml"))

	if err := tmpl.ExecuteTemplate(w, "deck.gohtml", rd); err != nil {
		log.Printf("handleDeckCalc execute error: %v", err)
		panic(err)
	}
}

func handleRailsCalc(w http.ResponseWriter, r *http.Request, e DeckEstimate) {
	userAuth := getUserAuth(r, w)
	rd := renderData{
		Page:   &e,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("rails.gohtml").Funcs(funcMap).ParseFiles("templates/calc/rails.gohtml",
		"templates/header.gohtml", "templates/calc/deckheader.gohtml", "templates/footer.gohtml"))

	if err := tmpl.ExecuteTemplate(w, "rails.gohtml", rd); err != nil {
		log.Printf("*** handleRailsCalc *** execute error: %v", err)
		panic(err)
	}
}

/*
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

*/
