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

	// _ "github.com/mattn/go-sqlite3" // SQLite driver
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver
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
	EmailModalShown  bool // Flag to indicate if email modal should be shown
}

var tmpl *template.Template      // tmpl is the global template for estimate.html, initialized at startup.
var emailtmpl *template.Template // tmpl is the global template for email-confirm.html, initialized at startup.
var db *sql.DB                   // db is the SQLite database connection

func init() {
	gob.Register(DeckEstimate{})
	gob.Register(Customer{})
	gob.Register(UserAuth{})
	gob.Register(time.Time{})
	tmpl = template.Must(template.New("estimate.html").Funcs(funcMap).ParseFiles("templates/estimate.html",
		"templates/header.html", "templates/footer.html"))
	emailtmpl = template.Must(template.New("email-confirm.html").Funcs(funcMap).ParseFiles("templates/email-confirm.html",
		"templates/header.html", "templates/footer.html"))
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

	userAuth := getUserAuth(r, w)
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

// saveEstimate updates the estimate with save details and persists it to the session.
func saveEstimate(w http.ResponseWriter, r *http.Request, estimate *DeckEstimate, sd *SessionData) {
	// In your init or main
	dbURL := os.Getenv("DATABASE_URL") // We'll set this to the Neon string

	if dbURL == "" {
		log.Printf("DATABASE_URL environment variable is required")
		renderEstimate(w, r, DeckEstimate{Error: "Database Env - not set up."})
		return
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("Unable to connect to database: %v", err)
		renderEstimate(w, r, DeckEstimate{Error: "Database Connect failed."})
		return
	}

	// Before saving, see if the user is authenticated
	sessionData, err := GetSession(r, w)
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
	estimate.ExpirationDate = estimate.SaveDate.Add(30 * 24 * time.Hour) // Today + 30 days

	if estimate.EstimateID > 0 {
		log.Printf("Updating existing estimate ID=%d", estimate.EstimateID)
		stmt := `UPDATE estimates 
SET 
    description = $1,
    length = $2,
    width = $3,
    height = $4,
    material = $5,
    rail_material = $6,
    rail_infill = $7,
    stair_width = $8,
    stair_rail_count = $9,
    has_demo = $10,
    has_fascia = $11,
    total_cost = $12,
    first_name = $13,
    last_name = $14,
    address = $15,
    city = $16,
    state = $17,
    zip = $18,
    phone_number = $19,
    email = $20,
    save_date = $21,
    accept_date = $22,
    expiration_date = $23
WHERE estimate_id = $24
RETURNING estimate_id`
		var updatedID int64
		err = db.QueryRow(stmt, estimate.Desc, estimate.Length, estimate.Width, estimate.Height, //4
			estimate.Material, estimate.RailMaterial, estimate.RailInfill, //7
			estimate.StairWidth, estimate.StairRailCount, estimate.HasDemo, estimate.HasFascia, estimate.TotalCost, //12
			estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address, //15
			estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip, //18
			estimate.Customer.PhoneNumber, estimate.Customer.Email, //20
			estimate.SaveDate.Format("2006-01-02 15:04:05"),
			estimate.AcceptDate.Format("2006-01-02 15:04:05"),
			estimate.ExpirationDate.Format("2006-01-02 15:04:05"),
			estimate.EstimateID).Scan(&updatedID)

		if err != nil {
			log.Printf("Failed to prepare statement to update estimate: %v", err)
			renderEstimate(w, r, DeckEstimate{Error: "Database error: Update Estimate failed."})
			return
		}

		log.Printf("Estimate updated: ID=%d, SaveDate=%v, ExpirationDate=%v", estimate.EstimateID, estimate.SaveDate, estimate.ExpirationDate)
	} else {

		log.Printf("Inserting new estimate")
		//Prepared Statement - PostgreSQL handle the ID
		stmt := `INSERT INTO estimates (
    	description, length, width, height, material, rail_material, rail_infill,
    	stair_width, stair_rail_count, has_demo, has_fascia, total_cost,
    	first_name, last_name, address, city, state, zip, phone_number, email,
    	save_date, accept_date, expiration_date) 
		VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
        $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23	
		) RETURNING estimate_id`
		var newID int64
		err = db.QueryRow(stmt, estimate.Desc, estimate.Length, estimate.Width, estimate.Height,
			estimate.Material, estimate.RailMaterial, estimate.RailInfill,
			estimate.StairWidth, estimate.StairRailCount, estimate.HasDemo, estimate.HasFascia, estimate.TotalCost,
			estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address,
			estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip,
			estimate.Customer.PhoneNumber, estimate.Customer.Email,
			estimate.SaveDate.Format("2006-01-02 15:04:05"),
			nil,
			estimate.ExpirationDate.Format("2006-01-02 15:04:05")).Scan(&newID)
		if err != nil {
			log.Printf("Failed to save estimate to DB: %v", err)
			renderEstimate(w, r, DeckEstimate{Error: "Database error: Save Estimate failed."})
			return
		}
		estimate.EstimateID = int(newID) // Add the new Estimate ID to the Struct
	}

	estimate.EmailModalShown = true // Show the email modal after saving
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

	sd, err := GetSession(r, w)

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

	// Pass both estimate and customer to template
	renderEstimate(w, r, estimate)

	// Save estimate to session
	sd.Estimate.EmailModalShown = false // Reset email modal flag
	sd.Estimate = estimate
	err = sd.Save(r, w)
	if err != nil {
		log.Printf("Estimate Handler - Save Session failed")
	}

}

func emailHandler(w http.ResponseWriter, r *http.Request) {
	// Get session data
	sd, err := GetSession(r, w)
	if err != nil {
		log.Printf("Email Handler - Get Session failed: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Check if estimate exists in session
	if sd.Estimate.EstimateID == 0 {
		log.Printf("Email Handler - Missing estimate in session")
		http.Error(w, "No estimate available", http.StatusNotFound)
		return
	}

	rd := renderData{
		Page:   sd.Estimate,
		Header: sd.UserAuth,
	}

	if err := emailtmpl.ExecuteTemplate(w, "email-confirm.html", rd); err != nil {
		log.Printf("emailHandler execute error: %v", err)
		panic(err)
	}
	log.Printf("emailHandler - Complated successfully for Estimate ID=%d", sd.Estimate.EstimateID)
}
