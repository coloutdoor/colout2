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

	"github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
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
	Length         float64
	Width          float64
	Height         float64
	DeckArea       float64
	Material       string
	RailMaterial   string
	RailInfill     string
	TotalCost      float64
	DeckCost       float64
	RailCost       float64
	StairCost      float64
	Subtotal       float64
	FasciaCost     float64
	FasciaFeet     float64
	StairWidth     float64
	StairRailCost  float64
	DemoCost       float64
	HasDemo        bool
	RailFeet       float64
	SalesTax       float64
	HasFascia      bool
	Customer       Customer
	EstimateID     int
	ExpirationDate time.Time
	SaveDate       time.Time
	AcceptDate     time.Time
	Terms          string
	Error          string
}

// renderEstimate executes the "estimate.html" template with the given estimate, handling errors.
func renderEstimate(w http.ResponseWriter, estimate DeckEstimate) {
	// Terms is not part of session
	terms, err := os.ReadFile("static/t_and_c.txt")
	if err != nil {
		// Fallback if file is missing
		terms = []byte("Terms and Conditions not available.")
	}
	estimate.Terms = string(terms)

	if err := tmpl.ExecuteTemplate(w, "estimate.html", estimate); err != nil {
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
        length REAL,
        width REAL,
        height REAL,
        material TEXT,
        rail_material TEXT,
        rail_infill TEXT,
        stair_width REAL,
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
	gob.Register(time.Time{})
	tmpl = template.Must(template.New("estimate.html").Funcs(funcMap).ParseFiles("templates/estimate.html"))
}

// saveEstimate updates the estimate with save details and persists it to the session.
func saveEstimate(w http.ResponseWriter, r *http.Request, estimate *DeckEstimate, session *sessions.Session) {
	estimate.SaveDate = time.Now()
	// estimate.EstimateID = 1000                                           // Static ID for now
	estimate.ExpirationDate = estimate.SaveDate.Add(30 * 24 * time.Hour) // Today + 30 days

	// Get next EstimateID from DB - Set to 1000, if it does not exist
	var nextID int
	err := db.QueryRow("SELECT COALESCE(MAX(estimate_id), 999) + 1 FROM estimates").Scan(&nextID)
	if err != nil {
		log.Printf("Failed to get next EstimateID: %v", err)
		renderEstimate(w, DeckEstimate{Error: "Database ID error"})
		return
	}
	estimate.EstimateID = nextID

	// Save to SQLite database
	insertSQL := `
        INSERT INTO estimates (
            estimate_id, length, width, height, material, rail_material, rail_infill,
            stair_width, has_demo, has_fascia, total_cost, first_name, last_name,
            address, city, state, zip, phone_number, email, 
            save_date, accept_date, expiration_date
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(insertSQL,
		estimate.EstimateID, estimate.Length, estimate.Width, estimate.Height,
		estimate.Material, estimate.RailMaterial, estimate.RailInfill,
		estimate.StairWidth, estimate.HasDemo, estimate.HasFascia, estimate.TotalCost,
		estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address,
		estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip,
		estimate.Customer.PhoneNumber, estimate.Customer.Email,
		estimate.SaveDate.Format("2006-01-02 15:04:05"),
		nil, // accept_date - null until accepted
		estimate.ExpirationDate.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		log.Printf("Failed to save estimate to DB: %v", err)
		renderEstimate(w, DeckEstimate{Error: "Database save error"})
		return
	}

	session.Values["estimate"] = *estimate
	if err := session.Save(r, w); err != nil {
		log.Printf("Session save error: %v", err)
		renderEstimate(w, DeckEstimate{Error: "Session save error"})
		return
	}
	log.Printf("Estimate saved: ID=%d, SaveDate=%v, ExpirationDate=%v", estimate.EstimateID, estimate.SaveDate, estimate.ExpirationDate)
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
			saveEstimate(w, r, &estimate, session)
		} else {
			renderEstimate(w, DeckEstimate{Error: "Please complete Customer and Estimate before Saving."})
			return
		}
		renderEstimate(w, estimate)
		return
	}

	// ************* POST - Accept  - After Save ********************************
	if r.FormValue("accept") == "true" && !estimate.SaveDate.IsZero() {
		estimate.AcceptDate = time.Now()
		session.Values["estimate"] = estimate
		if err := session.Save(r, w); err != nil {
			log.Printf("Session save error: %v", err)
			renderEstimate(w, DeckEstimate{Error: "Session save error"})
			return
		}
		log.Printf("Estimate accepted at %v", estimate.AcceptDate)
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

	// Unsave - if it was previously saved - It is changed :(
	estimate.SaveDate = time.Time{}
	estimate.EstimateID = 0 // Static ID for now
	estimate.ExpirationDate = time.Time{}
	estimate.AcceptDate = time.Time{}

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
