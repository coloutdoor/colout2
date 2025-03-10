package main

import (
	"encoding/gob"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
)

// Define template functions
var funcMap = template.FuncMap{
	"formatCost":            formatCost,
	"formatDeckDescription": formatDeckDescription,
}

// Session store - in-memory for now, single secret key
var store = sessions.NewCookieStore([]byte("super-secret-key-12345"))

// DeckEstimate holds all data for a deck cost estimate.
type DeckEstimate struct {
	Length        float64
	Width         float64
	Height        float64
	DeckArea      float64
	Material      string
	RailMaterial  string
	RailInfill    string
	TotalCost     float64
	DeckCost      float64
	RailCost      float64
	StairCost     float64
	Subtotal      float64
	FasciaCost    float64
	FasciaFeet    float64
	StairWidth    float64
	StairRailCost float64
	DemoCost      float64
	HasDemo       bool
	RailFeet      float64
	SalesTax      float64
	HasFascia     bool
	Customer      Customer
	SaveDate      time.Time
	Error         string
}

// renderEstimate executes the "estimate.html" template with the given estimate, handling errors.
func renderEstimate(w http.ResponseWriter, estimate DeckEstimate) {
	if err := tmpl.ExecuteTemplate(w, "estimate.html", estimate); err != nil {
		log.Printf("estimateHandler execute error: %v", err)
		panic(err)
	}
}

// tmpl is the global template for estimate.html, initialized at startup.
var tmpl *template.Template

func init() {
	gob.Register(DeckEstimate{})
	gob.Register(Customer{})
	tmpl = template.Must(template.New("estimate.html").Funcs(funcMap).ParseFiles("templates/estimate.html"))
}

// EstimatePageData holds data for the estimate page, including customer info.
type EstimatePageData struct {
	Estimate DeckEstimate
	Customer Customer
}

func estimateHandler(w http.ResponseWriter, r *http.Request) {
	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		renderEstimate(w, DeckEstimate{Error: "Session error"})
		return
	}

	// Load customer from session
	customer := Customer{}
	if cust, ok := session.Values["customer"].(Customer); ok {
		customer = cust
	} else {
		log.Printf("Session get error: %v", err)
	}

	// Load the estimate from session
	estimate := DeckEstimate{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		estimate = est
	}
	estimate.Customer = customer // Embed customer in estimate

	// ************* GET  ********************************
	if r.Method != http.MethodPost {
		// Load estimate from session for GET
		renderEstimate(w, estimate)
		return
	}

	// ************* POST - SAVE  ********************************
	if r.FormValue("save") == "true" {
		if estimate.TotalCost > 0 && estimate.Customer.FirstName != "" {
			estimate.SaveDate = time.Now()
			session.Values["estimate"] = estimate
			if err := session.Save(r, w); err != nil {
				log.Printf("Session save error: %v", err)
				renderEstimate(w, DeckEstimate{Error: "Session save error"})
				return
			}
			log.Printf("Estimate saved at %v", estimate.SaveDate)
		} else {
			renderEstimate(w, DeckEstimate{Error: "Please complete Customer and Estimate before Saving."})
			return
		}
		renderEstimate(w, estimate)
		return
	}

	// ************* POST - Data - calculate estimate ********************************
	length, err := strconv.ParseFloat(r.FormValue("length"), 64)
	if err != nil || length <= 0 {
		renderEstimate(w, DeckEstimate{Error: "Length must be a positive number"})
		return
	}

	width, err := strconv.ParseFloat(r.FormValue("width"), 64)
	if err != nil || width <= 0 {
		renderEstimate(w, DeckEstimate{Error: "Width must be a positive number"})
		return
	}

	height, err := strconv.ParseFloat(r.FormValue("height"), 64)
	if err != nil || height < 0 {
		renderEstimate(w, DeckEstimate{Error: "Height must be a non-negative number"})
		return
	}

	stairWidth, err := strconv.ParseFloat(r.FormValue("stairWidth"), 64)
	if err != nil || stairWidth < 0 {
		stairWidth = 0 // Default to 0 if invalid or not provided
	}

	estimate.Length = length
	estimate.Width = width
	estimate.Height = height
	estimate.Material = r.FormValue("material")
	estimate.RailMaterial = r.FormValue("railMaterial")
	estimate.RailInfill = r.FormValue("railInfill")
	estimate.DeckArea = length * width
	estimate.HasDemo = r.FormValue("hasDemo") == "on"
	estimate.HasFascia = r.FormValue("hasFascia") == "on"
	estimate.StairWidth = stairWidth

	materialCost, ok := costs.DeckMaterials[estimate.Material]
	if !ok { // Shouldn’t hit this—deck calc already checks
		materialCost = 0
	}

	deckCost, err := CalculateDeckCost(length, width, height, estimate.Material, costs)
	if err != nil {
		estimate.Error = err.Error()
		renderEstimate(w, estimate)
		return
	}
	estimate.DeckCost = deckCost

	stairCost, err := CalculateStairCost(height, stairWidth, materialCost)
	if err != nil {
		estimate.Error = err.Error()
		renderEstimate(w, estimate)
		return
	}
	estimate.StairCost = stairCost
	if stairCost > 0 {
		estimate.StairRailCost = CalculateStairRailCost(height, estimate.RailMaterial, costs)
	}

	railCost, err := CalculateRailCost(length, width, estimate.RailMaterial, estimate.RailInfill, costs)
	if err != nil {
		estimate.Error = err.Error()
		renderEstimate(w, estimate)
		return
	}
	estimate.RailCost = railCost
	estimate.RailFeet = (2 * length) + width

	estimate.DemoCost = 0
	if estimate.HasDemo {
		estimate.DemoCost = CalculateDemoCost(length*width, costs)
	}

	estimate.FasciaCost = 0
	if estimate.HasFascia {
		estimate.FasciaCost = CalculateFasciaCost(length, width, costs)
		estimate.FasciaFeet = (2 * length) + width
	}

	log.Printf("Estimate: %+v", estimate)

	estimate.Subtotal = estimate.DeckCost + estimate.RailCost + estimate.StairCost + estimate.StairRailCost + estimate.DemoCost + estimate.FasciaCost
	estimate.SalesTax = CalculateSalesTax(estimate.Subtotal)
	estimate.TotalCost = estimate.Subtotal + estimate.SalesTax

	// Save estimate to session
	session.Values["estimate"] = estimate
	if err := session.Save(r, w); err != nil {
		log.Printf("Session save error: %v", err)
	}

	// Pass both estimate and customer to template
	renderEstimate(w, estimate)
}
