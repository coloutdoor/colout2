package main

import (
	"database/sql"
	"encoding/gob"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Define template functions
var funcMap = template.FuncMap{
	"formatCost":            formatCost,
	"formatDeckDescription": formatDeckDescription,
	"formatDemoDescription": formatDemoDescription,
	"currentYear":           func() int { return time.Now().Year() },
}

// DeckEstimate holds all data for a deck cost estimate.
type DeckEstimate struct {
	Desc             string
	Length           float64
	Width            float64
	Height           float64
	DeckArea         float64
	Material         string
	RailMaterial     string
	RailInfill       string
	TotalCost        float64
	DeckCost         float64
	RailCost         float64
	StairCost        float64
	Subtotal         float64
	HasFascia        bool
	FasciaCost       float64
	FasciaFeet       float64
	StairWidth       float64
	StairRailCount   float64
	StairRailCost    float64
	HasStairFascia   bool
	StairFasciaCost  float64
	StairToeKickCost float64
	HasStairTK       bool
	DemoCost         float64
	HasDemo          bool
	RailFeet         float64
	SalesTax         float64
	Customer         Customer
	EstimateID       int
	ExpirationDate   time.Time
	SaveDate         time.Time
	AcceptDate       time.Time
	Terms            string
	Error            string
}

// renderEstimate executes the "estimate.html" template with the given estimate, handling errors.
func renderEstimate(w http.ResponseWriter, r *http.Request, estimate DeckEstimate) {
	// Terms is not part of session
	terms, err := os.ReadFile("static/t_and_c.txt")
	if err != nil {
		// Fallback if file is missing
		terms = []byte("Terms and Conditions not available.")
	}
	estimate.Terms = string(terms)

	userAuth := getUserAuth(r)
	userAuth.Title = "Deck Estimate"
	rd := renderData{
		Page:   &estimate,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "estimate.html", rd); err != nil {
		log.Printf("estimateHandler execute error: %v", err)
		panic(err)
	}
}

var tmpl *template.Template // tmpl is the global template for estimate.html, initialized at startup.
var db *sql.DB              // db is the SQLite database connection

func init() {
	// Initialize SQLite database
	var err error
	dbDir := os.Getenv("DB_DIR")
	if dbDir == "" {
		dbDir = "./db" // Default to ./db if DB_DIR not set
	}
	// Initialize SQLite database
	db, err = sql.Open("sqlite3", dbDir+"/estimates.db")
	if err != nil {
		log.Fatalf("Failed to open SQLite DB: %v", err)
	}

	// Create Estimates table
	createTableSQL := `
    CREATE TABLE IF NOT EXISTS estimates (
        estimate_id INTEGER PRIMARY KEY,
		desc TEXT,
        length REAL,
        width REAL,
        height REAL,
        material TEXT,
        rail_material TEXT,
        rail_infill TEXT,
        stair_width REAL,
		stair_rail_count REAL,
        has_demo BOOLEAN,
        has_fascia BOOLEAN,
        total_cost REAL,
        first_name TEXT,
        last_name TEXT,
        address TEXT,
        city TEXT,
        state TEXT,
        zip TEXT,
        phone_number TEXT,
        email TEXT,
        save_date TEXT,
        accept_date TEXT,
        expiration_date TEXT
    );`
	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create Estimates table: %v", err)
	}

	gob.Register(DeckEstimate{})
	gob.Register(Customer{})
	gob.Register(UserAuth{})
	gob.Register(time.Time{})
	tmpl = template.Must(template.New("estimate.html").Funcs(funcMap).ParseFiles("templates/estimate.html",
		"templates/header.html", "templates/footer.html"))
}

// saveEstimate updates the estimate with save details and persists it to the session.
func saveEstimate(w http.ResponseWriter, r *http.Request, estimate *DeckEstimate, sd *SessionData) {

	// Before saving, see if the user is authenticated
	sessionData, err := GetSession(r)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	if !sessionData.UserAuth.IsAuthenticated {
		sessionData.UserAuth.Message = "Please Login to save estimate"
		sessionData.Save(r, w)
		loginUrl := "/login?rurl=/estimate"
		http.Redirect(w, r, loginUrl, http.StatusSeeOther)
	}

	estimate.SaveDate = time.Now()
	// estimate.EstimateID = 1000                                           // Static ID for now
	estimate.ExpirationDate = estimate.SaveDate.Add(30 * 24 * time.Hour) // Today + 30 days

	// Get next EstimateID from DB - Set to 1000, if it does not exist
	var nextID int
	err = db.QueryRow("SELECT COALESCE(MAX(estimate_id), 999) + 1 FROM estimates").Scan(&nextID)
	if err != nil {
		log.Printf("Failed to get next EstimateID: %v", err)
		renderEstimate(w, r, DeckEstimate{Error: "Database ID error"})
		return
	}
	estimate.EstimateID = nextID

	// Save to SQLite database
	insertSQL := `
        INSERT INTO estimates (
            estimate_id, desc, length, width, height, material, rail_material, rail_infill,
            stair_width, stair_rail_count, has_demo, has_fascia, total_cost, first_name, last_name,
            address, city, state, zip, phone_number, email, 
            save_date, accept_date, expiration_date
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(insertSQL,
		estimate.EstimateID, estimate.Desc, estimate.Length, estimate.Width, estimate.Height,
		estimate.Material, estimate.RailMaterial, estimate.RailInfill,
		estimate.StairWidth, estimate.StairRailCount, estimate.HasDemo, estimate.HasFascia, estimate.TotalCost,
		estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address,
		estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip,
		estimate.Customer.PhoneNumber, estimate.Customer.Email,
		estimate.SaveDate.Format("2006-01-02 15:04:05"),
		nil, // accept_date - null until accepted
		estimate.ExpirationDate.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		log.Printf("Failed to save estimate to DB: %v", err)
		renderEstimate(w, r, DeckEstimate{Error: "Database save error"})
		return
	}

	sd.Estimate = *estimate
	err = sd.Save(r, w)
	if err != nil {
		log.Printf("Failed to save Session Data in Deck Estimate - saveEstimate()")
	}

	log.Printf("Estimate saved: ID=%d, SaveDate=%v, ExpirationDate=%v", estimate.EstimateID, estimate.SaveDate, estimate.ExpirationDate)
}

// EstimatePageData holds data for the estimate page, including customer info.
type EstimatePageData struct {
	Estimate DeckEstimate
	Customer Customer
}

// **********************************************************************************
// estimateHandler
//
//  Data can be posted to this page from either
//
//   Calculater  - Full details
//   /calc/deck  - /calc?option=deck - Basic Deck with Finish Level
// **********************************************************************************

func estimateHandler(w http.ResponseWriter, r *http.Request) {
	// Get session

	sd, err := GetSession(r)

	if err != nil {
		log.Printf("Session failed: %v", err)
		renderEstimate(w, r, DeckEstimate{Error: "Session error"})
		return
	}

	// Load customer from session
	customer := sd.Customer
	estimate := sd.Estimate
	estimate.Customer = customer // Embed customer in estimate

	// ************* GET  ********************************
	if r.Method != http.MethodPost {
		// Load estimate from session for GET
		renderEstimate(w, r, estimate)
		return
	}

	// ************* POST - SAVE  ********************************
	if r.FormValue("save") == "true" {
		if estimate.TotalCost > 0 && estimate.Customer.FirstName != "" {
			saveEstimate(w, r, &estimate, sd)
		} else {
			renderEstimate(w, r, DeckEstimate{Error: "Please complete Customer and Estimate before Saving."})
			return
		}
		renderEstimate(w, r, estimate)
		return
	}

	// ************* POST - Accept  - After Save ********************************
	if r.FormValue("accept") == "true" && !estimate.SaveDate.IsZero() {
		estimate.AcceptDate = time.Now()
		saveEstimate(w, r, &estimate, sd)
		log.Printf("Estimate accepted at %v", estimate.AcceptDate)
		renderEstimate(w, r, estimate)
		return
	}

	// ************* POST - Data - calculate estimate ********************************
	length, err := strconv.ParseFloat(r.FormValue("length"), 64)
	if err != nil || length <= 0 {
		renderEstimate(w, r, DeckEstimate{Error: "Deck Length must be a positive number"})
		return
	}

	width, err := strconv.ParseFloat(r.FormValue("width"), 64)
	if err != nil || width <= 0 {
		renderEstimate(w, r, DeckEstimate{Error: "Deck Width must be a positive number"})
		return
	}

	height, err := strconv.ParseFloat(r.FormValue("height"), 64)
	if err != nil || height < 0 {
		renderEstimate(w, r, DeckEstimate{Error: "Deck Height must be a non-negative number"})
		return
	}

	stairWidth, err := strconv.ParseFloat(r.FormValue("stairWidth"), 64)
	if err != nil || stairWidth < 0 {
		stairWidth = 0 // Default to 0 if invalid or not provided
	}

	//
	stairRailCount, err := strconv.ParseFloat(r.FormValue("stairRailCount"), 64)
	if err != nil || stairRailCount < 0 {
		stairRailCount = 0 // Default to 0 if invalid or not provided
	}

	estimate.Desc = r.FormValue("desc")
	estimate.Length = length
	estimate.Width = width
	estimate.Height = height
	estimate.DeckArea = length * width
	estimate.Material = r.FormValue("material")
	estimate.RailMaterial = r.FormValue("railMaterial")
	estimate.RailInfill = r.FormValue("railInfill")
	estimate.HasDemo = r.FormValue("hasDemo") == "on"
	estimate.HasFascia = r.FormValue("hasFascia") == "on"
	estimate.StairWidth = stairWidth
	estimate.StairRailCount = stairRailCount
	estimate.HasStairFascia = r.FormValue("hasStairFascia") == "on"
	estimate.HasStairTK = r.FormValue("hasStairTK") == "on"

	// ************** POST - Finish Level from /calc/deck **************************
	//
	// Set the matials and selections based on the Deck options:
	// *****************************************************************************
	if r.FormValue("finish") != "" {
		log.Printf("Setting Finish Level to: %s", r.FormValue("finish"))
		log.Printf("Settign Stairs to: %s", r.FormValue("hasStairs"))
		// TODO - Make this a funtion and yaml settings
		switch r.FormValue("finish") {
		//economy
		case "1":
			estimate.Material = "outdoorWood"
			estimate.RailMaterial = "wood"
			estimate.RailInfill = "balusters"
			estimate.HasFascia = false
			estimate.StairWidth = 3.0
			estimate.StairRailCount = 2
			estimate.HasStairFascia = false
			estimate.HasStairTK = false

		case "2":
			estimate.Material = "cedar"
			estimate.RailMaterial = "wood"
			estimate.RailInfill = "balusters"
			estimate.HasFascia = false
			estimate.StairWidth = 3.0
			estimate.StairRailCount = 2
			estimate.HasStairFascia = false
			estimate.HasStairTK = false

		case "3":
			estimate.Material = "timberTechPrime"
			estimate.RailMaterial = "aluminum"
			estimate.RailInfill = "balusters"
			estimate.HasFascia = false
			estimate.StairWidth = 3.5
			estimate.StairRailCount = 2
			estimate.HasStairFascia = false
			estimate.HasStairTK = true

		// TODO - Add Picture Framing and Joist Spacing and Butyl Tape
		case "4":
			estimate.Material = "timberTechProReserve"
			estimate.RailMaterial = "aluminum"
			estimate.RailInfill = "cable"
			estimate.HasFascia = true
			estimate.StairWidth = 4.0
			estimate.StairRailCount = 2
			estimate.HasStairFascia = false
			estimate.HasStairTK = true

		// TODO - Add Stair Picture Framing
		case "5":
			estimate.Material = "timberTechProLegacy"
			estimate.RailMaterial = "composite"
			estimate.RailInfill = "glass"
			estimate.HasFascia = true
			estimate.StairWidth = 4.0
			estimate.StairRailCount = 2
			estimate.HasStairFascia = true
			estimate.HasStairTK = true
		}

		// Only add rails if greater than 30" by default
		if estimate.Height < 2.5 {
			estimate.RailMaterial = ""
			estimate.RailInfill = ""
			estimate.StairRailCount = 0
		}

		// Stairs are optional for decks
		if r.FormValue("hasStairs") != "on" {
			estimate.StairWidth = 0.0
			estimate.StairRailCount = 0.0
			estimate.HasStairFascia = false
			estimate.HasStairTK = false
		}
	}

	// Unsave - if it was previously saved - It is changed :(
	estimate.SaveDate = time.Time{}
	estimate.EstimateID = 0 // Static ID for now
	estimate.ExpirationDate = time.Time{}
	estimate.AcceptDate = time.Time{}
	estimate.Error = ""

	estimate.CalculateDeckCost(costs)
	if estimate.Error != "" {
		renderEstimate(w, r, estimate)
		return
	}

	estimate.CalcStairCost(costs)
	if estimate.Error != "" {
		renderEstimate(w, r, estimate)
		return
	}
	estimate.CalculateRailCost(costs)
	if estimate.Error != "" {
		renderEstimate(w, r, estimate)
		return
	}

	estimate.CalculateStairRailCost(costs)
	estimate.CalcStairFasciaCost(costs)
	estimate.CalcStairToeKickCost(costs)
	estimate.CalculateDemoCost(costs)
	estimate.CalculateFasciaCost(costs)

	log.Printf("Estimate: %+v", estimate)

	estimate.Subtotal = estimate.DeckCost + estimate.RailCost + estimate.StairCost + estimate.StairRailCost + estimate.DemoCost + estimate.FasciaCost + estimate.StairFasciaCost
	estimate.SalesTax = CalculateSalesTax(estimate.Subtotal)
	estimate.TotalCost = estimate.Subtotal + estimate.SalesTax

	// Save estimate to session
	sd.Estimate = estimate
	err = sd.Save(r, w)
	if err != nil {
		log.Printf("Estimate Handler - Save Session failed")
	}

	// Pass both estimate and customer to template
	renderEstimate(w, r, estimate)
}
