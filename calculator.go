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

	// Service type defaults to Full Service rather than whatever the last
	// session estimate used; a diyMode in the URL overrides it.
	estimate.DIYMode = 0
	if dm := query.Get("diyMode"); dm != "" {
		if v, err := strconv.Atoi(dm); err == nil && v >= 0 && v <= 2 {
			estimate.DIYMode = v
		}
	}

	// Success: Route to correct calculator
	switch option {
	case "rails":
		handleRailsCalc(w, r, estimate)
	default:
		handleDeckCalc(w, r, estimate)
	}
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
	userAuth.Title = "Free Deck Calculator"
	userAuth.Subtitle = "Instant deck cost estimates"
	userAuth.MetaDesc = "Free deck cost calculator for SW Washington. Get an instant estimate for deck size, materials, rails, stairs, and permits. No salesperson, no callbacks."
	userAuth.CanonicalPath = "/deck-calculator"
	queueGAEvent(w, r, "calculator_start", map[string]interface{}{"product_type": "deck"})
	recordNREvent("CalculatorStarted", map[string]interface{}{"product_type": "deck"})
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

// *****************************************************************************************
//
//	patioCalcData
//
//	  Minimal view model for the patio cover calculator mock-up. Deliberately kept separate
//	  from DeckEstimate since patio cover pricing/persistence doesn't exist yet.
//
// *****************************************************************************************
type patioCalcData struct {
	DIYMode int
}

// *****************************************************************************************
//
//	handlePatioCalc
//
//	  UI-only mock-up for the patio cover calculator. Does not post to /estimate or touch
//	  the DB — the form is a front-end preview until patio cover pricing exists.
//
// *****************************************************************************************
func handlePatioCalc(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Free Patio Cover Calculator"
	userAuth.Subtitle = "Instant patio cover cost estimates"
	userAuth.MetaDesc = "Free patio cover cost calculator for SW Washington. Get an instant estimate for pergola, lean-to, truss, and timberframe covers."
	userAuth.CanonicalPath = "/patio-cover-calculator"
	queueGAEvent(w, r, "calculator_start", map[string]interface{}{"product_type": "patio_cover"})
	recordNREvent("CalculatorStarted", map[string]interface{}{"product_type": "patio_cover"})

	data := patioCalcData{}
	if dm := r.URL.Query().Get("diyMode"); dm != "" {
		if v, err := strconv.Atoi(dm); err == nil && v >= 0 && v <= 2 {
			data.DIYMode = v
		}
	}

	rd := renderData{
		Page:   &data,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("patio.gohtml").Funcs(funcMap).ParseFiles("templates/calc/patio.gohtml",
		"templates/header.gohtml", "templates/calc/deckheader.gohtml", "templates/footer.gohtml"))

	if err := tmpl.ExecuteTemplate(w, "patio.gohtml", rd); err != nil {
		log.Printf("handlePatioCalc execute error: %v", err)
		panic(err)
	}
}

// *****************************************************************************************
//
//	calcResetHandler
//
//	  POST /calc/reset — the calculators' "Start over" link. Clears the in-progress
//	  estimates and customer info from the session (login is kept) and sends the user
//	  back to a blank calculator.
//
// *****************************************************************************************
func calcResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/calc", http.StatusSeeOther)
		return
	}
	sd, err := GetSession(r, w)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	sd.Estimate = DeckEstimate{}
	sd.PatioEstimate = PatioCoverEstimate{}
	sd.Customer = Customer{}
	sd.ActiveProduct = ""
	sd.PendingSave = false
	if err := sd.Save(r, w); err != nil {
		log.Printf("calcResetHandler: session save failed: %v", err)
	}

	// Only return to a known calculator page, never an arbitrary URL.
	target := "/calc"
	switch r.FormValue("return") {
	case "/deck-calculator", "/patio-cover-calculator":
		target = r.FormValue("return")
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// *****************************************************************************************
//
//	calcPickerHandler
//
//	  /calc — lets the homeowner choose a project type (deck or patio cover) before
//	  landing on the type-specific calculator.
//
// *****************************************************************************************
func calcPickerHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Free Deck & Patio Cover Cost Calculator"
	userAuth.Subtitle = "Choose a project type below to get an instant, itemized, no-obligation estimate."
	userAuth.MetaDesc = "Free instant deck and patio cover cost calculator for Clark and Cowlitz County, SW Washington. Price your project by size and materials — no account or sales call required."
	userAuth.CanonicalPath = "/calc"

	rd := renderData{
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("picker.gohtml").Funcs(funcMap).ParseFiles("templates/calc/picker.gohtml",
		"templates/calc/deckheader.gohtml", "templates/header.gohtml", "templates/footer.gohtml"))

	if err := tmpl.ExecuteTemplate(w, "picker.gohtml", rd); err != nil {
		log.Printf("calcPickerHandler execute error: %v", err)
		panic(err)
	}
}

// *****************************************************************************************
//
//	calcSlugRedirectHandler
//
//	  /calc/deck, /calc/cover — short aliases that 301 to the canonical calculator URLs.
//
// *****************************************************************************************
func calcSlugRedirectHandler(w http.ResponseWriter, r *http.Request) {
	var target string
	switch r.PathValue("slug") {
	case "deck":
		target = "/deck-calculator"
	case "cover":
		target = "/patio-cover-calculator"
	default:
		notFoundHandler(w, r)
		return
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
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
