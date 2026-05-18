package main

import (
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Define template functions
var funcMap = template.FuncMap{
	"formatCost":                   formatCost,
	"formatDeckDescription":        formatDeckDescription,
	"formatDemoDescription":        formatDemoDescription,
	"formatRailDescription":        formatRailDescription,
	"formatStairDescription":       formatStairDescription,
	"formatFasciaDescription":      formatFasciaDescription,
	"formatStairRailDescription":   formatStairRailDescription,
	"formatStairFasciaDescription": formatStairFasciaDescription,
	"formatStairTKDescription":     formatStairTKDescription,
	"currentYear":                  func() int { return time.Now().Year() },
	// jsStr encodes a string as a JavaScript string literal, safe inside <script> tags.
	"jsStr": func(s string) template.JS {
		b, _ := json.Marshal(s)
		return template.JS(b)
	},
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
	Contractor       ContractorInfo
	ContractorID     int64
	EstimateID       int
	ExpirationDate   time.Time
	SaveDate         time.Time
	AcceptDate       time.Time
	Terms            string
	Error            string
	EmailModalShown  bool  // Flag to indicate if email modal should be shown
	UserId           int64 // FK to UserAuth
	Status           string // Accepted, Expired, or Pending — computed in renderEstimate
	Version          int    // Increments on each save
	Sections         []EstimateSection
	RailFeetOverride float64 // 0 = auto-calculate from primary section
}

type EstimateSection struct {
	ID         int64
	EstimateID int
	Label      string
	Length     float64
	Width      float64
	SortOrder  int
}

type ContractorInfo struct {
	ID           int64
	CompanyName  string
	Phone        string
	Website      string
	LicenseNum   string
	LicenseState string
}

var tmpl *template.Template // tmpl is the global template for estimate.gohtml, initialized at startup.
var db *sql.DB              // db is the SQLite database connection

func init() {
	gob.Register(DeckEstimate{})
	gob.Register(EstimateSection{})
	gob.Register(Customer{})
	gob.Register(UserAuth{})
	gob.Register(time.Time{})
	tmpl = template.Must(template.New("estimate.gohtml").Funcs(funcMap).ParseFiles("templates/estimate.gohtml",
		"templates/header.gohtml", "templates/footer.gohtml"))
}

// renderEstimate executes the "estimate.gohtml" template with the given estimate, handling errors.
func renderEstimate(w http.ResponseWriter, r *http.Request, estimate DeckEstimate) {
	// Compute status
	if !estimate.AcceptDate.IsZero() {
		estimate.Status = "Accepted"
	} else if estimate.EstimateID > 0 && !estimate.ExpirationDate.IsZero() && estimate.ExpirationDate.Before(time.Now()) {
		estimate.Status = "Expired"
	} else if estimate.EstimateID > 0 {
		estimate.Status = "Pending"
	}

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
	if err := tmpl.ExecuteTemplate(w, "estimate.gohtml", rd); err != nil {
		log.Printf("estimateHandler execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ***************************************************************************************************
//
//	 getEstimate
//			Get the estimated from the DB
//
// ***************************************************************************************************
func getEstimate(estimateID int) DeckEstimate {
	dbURL := os.Getenv("DATABASE_URL") // We'll set this to the Neon string

	log.Printf("Finding estimate %d from DB ", estimateID)

	if dbURL == "" {
		log.Printf("DATABASE_URL environment variable is required")
		return DeckEstimate{Error: "Database Env - not set up."}
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("Unable to connect to database: %v", err)
		return DeckEstimate{Error: "Database Connect failed."}
	}

	var de DeckEstimate
	var acceptDate sql.NullTime
	err = db.QueryRow(`
        SELECT e.estimate_id, e.description, e.height, e.material, e.rail_material, e.rail_infill, e.stair_width,
        e.stair_rail_count, e.has_demo, e.has_fascia, e.total_cost, e.has_stair_fascia, e.has_stair_tk,
        e.first_name, e.last_name, e.address, e.city, e.state, e.zip, e.phone_number, e.email,
        e.save_date, e.accept_date, e.expiration_date, e.user_id, e.contractor_id, e.version,
        COALESCE(cp.company_name,''), COALESCE(cp.phone,''), COALESCE(cp.website,''),
        COALESCE(cp.license_number,''), COALESCE(cp.license_state,''), COALESCE(cp.id,1)
        FROM estimates e
        LEFT JOIN contractor_profile cp ON cp.id = e.contractor_id
        WHERE e.estimate_id = $1`, estimateID).Scan(
		&de.EstimateID, &de.Desc, &de.Height, &de.Material, &de.RailMaterial, &de.RailInfill, &de.StairWidth,
		&de.StairRailCount, &de.HasDemo, &de.HasFascia, &de.TotalCost, &de.HasStairFascia, &de.HasStairTK,
		&de.Customer.FirstName, &de.Customer.LastName, &de.Customer.Address, &de.Customer.City, &de.Customer.State,
		&de.Customer.Zip, &de.Customer.PhoneNumber, &de.Customer.Email,
		&de.SaveDate, &acceptDate, &de.ExpirationDate, &de.UserId, &de.ContractorID, &de.Version,
		&de.Contractor.CompanyName, &de.Contractor.Phone, &de.Contractor.Website,
		&de.Contractor.LicenseNum, &de.Contractor.LicenseState, &de.Contractor.ID)

	if err != nil {
		fmt.Println("GetEstimate Query Error: ", err)
		err = db.Close()

		return DeckEstimate{Error: "Estimate not found"}
	}
	log.Printf("Found estimate: %d", estimateID)

	if acceptDate.Valid {
		de.AcceptDate = acceptDate.Time
	}

	// Load sections (before closing DB)
	srows, serr := db.Query(`
		SELECT id, estimate_id, label, length, width, sort_order
		FROM estimate_sections WHERE estimate_id = $1
		ORDER BY sort_order, id`, estimateID)
	if serr == nil {
		for srows.Next() {
			var s EstimateSection
			if err := srows.Scan(&s.ID, &s.EstimateID, &s.Label, &s.Length, &s.Width, &s.SortOrder); err == nil {
				de.Sections = append(de.Sections, s)
			}
		}
		srows.Close()
	}

	db.Close()

	// Derive L/W from primary section so existing calculations still work
	if len(de.Sections) > 0 {
		de.Length = de.Sections[0].Length
		de.Width  = de.Sections[0].Width
	}

	de.Error = ""
	return de
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
		_ = db.Close()
		renderEstimate(w, r, DeckEstimate{Error: "Database Connect failed."})
		return
	}

	// Before saving, see if the user is authenticated
	sessionData, err := GetSession(r, w)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		_ = db.Close()
		return
	}
	if !sessionData.UserAuth.IsAuthenticated {
		sessionData.UserAuth.Message = "Please Login to save estimate"
		if err = sessionData.Save(r, w); err != nil {
			log.Printf("Unable to save sessionData")
		}
		loginUrl := "/login?rurl=/estimate"
		_ = db.Close()
		http.Redirect(w, r, loginUrl, http.StatusSeeOther)
	}

	estimate.UserId = sessionData.UserAuth.ID
	estimate.SaveDate = time.Now()
	estimate.ExpirationDate = estimate.SaveDate.Add(30 * 24 * time.Hour)

	// Set contractor_id: use the contractor's profile if they're a contractor, else default to 1
	if estimate.ContractorID == 0 {
		estimate.ContractorID = 1
		if sessionData.UserAuth.Role == "contractor" {
			db2, err2 := sql.Open("pgx", dbURL)
			if err2 == nil {
				db2.QueryRow(`SELECT id FROM contractor_profile WHERE user_id = $1`, estimate.UserId).Scan(&estimate.ContractorID)
				db2.Close()
			}
		}
	}

	// Update Existing estimate
	if estimate.EstimateID > 0 {
		log.Printf("Updating existing estimate ID=%d", estimate.EstimateID)
		stmt := `UPDATE estimates
SET
    description = $1,
    height = $2,
    material = $3,
    rail_material = $4,
    rail_infill = $5,
    stair_width = $6,
    stair_rail_count = $7,
    has_demo = $8,
    has_fascia = $9,
    total_cost = $10,
    first_name = $11,
    last_name = $12,
    address = $13,
    city = $14,
    state = $15,
    zip = $16,
    phone_number = $17,
    email = $18,
    save_date = $19,
    accept_date = $20,
    expiration_date = $21,
    has_stair_fascia = $22,
    has_stair_tk = $23,
    user_id = $24,
    contractor_id = $25,
    version = version + 1
WHERE estimate_id = $26
RETURNING estimate_id, version`
		var updatedID int64
		err = db.QueryRow(stmt, estimate.Desc, estimate.Height, //2
			estimate.Material, estimate.RailMaterial, estimate.RailInfill, //5
			estimate.StairWidth, estimate.StairRailCount, estimate.HasDemo, estimate.HasFascia, estimate.TotalCost, //10
			estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address, //13
			estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip, //16
			estimate.Customer.PhoneNumber, estimate.Customer.Email, //18
			estimate.SaveDate.Format("2006-01-02 15:04:05"), //19
			func() interface{} {
				if estimate.AcceptDate.IsZero() {
					return nil
				}
				return estimate.AcceptDate.Format("2006-01-02 15:04:05")
			}(), //20
			estimate.ExpirationDate.Format("2006-01-02 15:04:05"), //21
			estimate.HasStairFascia, estimate.HasStairTK,          //23
			estimate.UserId,       //24
			estimate.ContractorID, //25
			estimate.EstimateID).Scan(&updatedID, &estimate.Version)

		if err != nil {
			log.Printf("Failed to prepare statement to update estimate: %v", err)
			_ = db.Close()
			renderEstimate(w, r, DeckEstimate{Error: "Database error: Update Estimate failed."})
			return
		}

		_ = db.Close()
		log.Printf("Estimate updated: ID=%d, SaveDate=%v, ExpirationDate=%v", estimate.EstimateID, estimate.SaveDate, estimate.ExpirationDate)
	} else {

		// Create NEW Estimate
		log.Printf("Inserting new estimate")
		//Prepared Statement - PostgreSQL handle the ID
		stmt := `INSERT INTO estimates (
			description, height,
			material, rail_material, rail_infill,
			stair_width, stair_rail_count, has_demo, has_fascia, total_cost,
			first_name, last_name, address,
			city, state, zip, phone_number, email,
			save_date, accept_date, expiration_date, has_stair_fascia, has_stair_tk, user_id, contractor_id, version)
		VALUES (
		$1, $2,
		$3, $4, $5,
		$6, $7, $8, $9, $10,
		$11, $12, $13, $14, $15, $16, $17, $18,
		$19, $20, $21,
		$22, $23, $24, $25, 1
		) RETURNING estimate_id`
		var newID int64
		err = db.QueryRow(stmt,
			estimate.Desc, estimate.Height, //2
			estimate.Material, estimate.RailMaterial, estimate.RailInfill, //5
			estimate.StairWidth, estimate.StairRailCount, estimate.HasDemo, estimate.HasFascia, estimate.TotalCost, //10
			estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address, //13
			estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip, estimate.Customer.PhoneNumber, estimate.Customer.Email, //18
			estimate.SaveDate.Format("2006-01-02 15:04:05"),
			nil,
			estimate.ExpirationDate.Format("2006-01-02 15:04:05"),
			estimate.HasStairFascia, estimate.HasStairTK, //23
			estimate.UserId,       //24
			estimate.ContractorID).Scan(&newID) //27
		if err != nil {
			log.Printf("Failed to save estimate to DB: %v", err)
			_ = db.Close()
			renderEstimate(w, r, DeckEstimate{Error: "Database error: Save Estimate failed."})
			return
		}
		estimate.EstimateID = int(newID)
		_ = db.Close()
	}

	// Save sections — open fresh connection
	if len(estimate.Sections) > 0 {
		db2, err2 := sql.Open("pgx", dbURL)
		if err2 == nil {
			db2.Exec(`DELETE FROM estimate_sections WHERE estimate_id = $1`, estimate.EstimateID)
			for i, s := range estimate.Sections {
				db2.Exec(`INSERT INTO estimate_sections (estimate_id, label, length, width, sort_order)
					VALUES ($1, $2, $3, $4, $5)`,
					estimate.EstimateID, s.Label, s.Length, s.Width, i)
			}
			db2.Close()
		}
	}

	_ = db.Close()

	estimate.EmailModalShown = true // Show the email modal after saving
	sd.Estimate = *estimate
	err = sd.Save(r, w)
	if err != nil {
		log.Printf("Failed to save Session Data in Deck Estimate - saveEstimate()")
	}

	log.Printf("Estimate saved: ID=%d, SaveDate=%v, ExpirationDate=%v", estimate.EstimateID, estimate.SaveDate, estimate.ExpirationDate)
}

// **********************************************************************************
// estimateDBHandler
//
//	Get the estimate from the specific URI
//	     /estimate/{EstimateID}
//
// **********************************************************************************
func estimateDBHandler(w http.ResponseWriter, r *http.Request) {
	// Get session

	sd, err := GetSession(r, w)
	if err != nil {
		log.Printf("Session failed: %v", err)
		renderEstimate(w, r, DeckEstimate{Error: "estimateDBHandler: Session error"})
		return
	}

	// Get Estimate ID from the URI...
	idStr := strings.TrimPrefix(r.URL.Path, "/estimate/")
	if idStr == r.URL.Path { // didn't match prefix
		renderEstimate(w, r, DeckEstimate{Error: "estimateDBHandler - URI not found!"})
		return
	}

	// Check if user is authenticated?
	//   If not logged in, redirect to the user auth page
	if !sd.UserAuth.IsAuthenticated {
		sd.UserAuth.Message = "Please Login to view estimate " + idStr
		_ = sd.Save(r, w)
		loginUrl := "/login?rurl=/estimate/" + idStr
		http.Redirect(w, r, loginUrl, http.StatusSeeOther)
		return
	}

	// Read Estimate from DB.
	idInt, _ := strconv.Atoi(idStr)
	de := getEstimate(idInt)
	if de.Error != "" {
		renderEstimate(w, r, de)
	}

	// Is this the owner of the estimate, or an admin?
	if de.UserId != sd.UserAuth.ID && !isAdminUser(sd.UserAuth.Email) {
		renderEstimate(w, r, DeckEstimate{Error: "Unauthorized."})
	}

	// Calculate the costs
	de.CalcAllCosts()
	if de.Error != "" {
		renderEstimate(w, r, de)
		return
	}

	// Sync session with the loaded estimate so /customer pre-fills correctly
	sd.Estimate  = de
	sd.Customer  = de.Customer
	_ = sd.Save(r, w)

	// Render the estimate
	renderEstimate(w, r, de)
}

// **********************************************************************************
// estimateHandler
//
//  Data can be posted to this page from either
//
//   Calculator  - Full details
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
		// For saved estimates always reload from DB to ensure fresh sections/costs
		if estimate.EstimateID > 0 {
			fresh := getEstimate(estimate.EstimateID)
			fresh.CalcAllCosts()
			fresh.Customer = estimate.Customer // keep session customer if set
			renderEstimate(w, r, fresh)
			return
		}
		renderEstimate(w, r, estimate)
		return
	}

	// ************* POST - SAVE  ********************************
	if r.FormValue("save") == "true" {
		// Pick up inline-edited description
		if desc := r.FormValue("desc"); desc != "" {
			estimate.Desc = desc
			sd.Estimate.Desc = desc
		}
		// Pick up sections from JS (JSON array)
		if sectionsJSON := r.FormValue("sections"); sectionsJSON != "" {
			var parsed []struct {
				Label  string  `json:"label"`
				Length float64 `json:"length"`
				Width  float64 `json:"width"`
			}
			if err := json.Unmarshal([]byte(sectionsJSON), &parsed); err == nil && len(parsed) > 0 {
				estimate.Sections = nil
				for i, s := range parsed {
					estimate.Sections = append(estimate.Sections, EstimateSection{
						Label:     s.Label,
						Length:    s.Length,
						Width:     s.Width,
						SortOrder: i,
					})
				}
				// Keep L/W in sync with primary section
				estimate.Length = estimate.Sections[0].Length
				estimate.Width  = estimate.Sections[0].Width
			}
		}
		if estimate.TotalCost > 0 && estimate.Customer.FirstName != "" {
			saveEstimate(w, r, &estimate, sd)
			http.Redirect(w, r, fmt.Sprintf("/estimate/%d", estimate.EstimateID), http.StatusSeeOther)
		} else {
			renderEstimate(w, r, DeckEstimate{Error: "Please complete Customer and Estimate before Saving."})
		}
		return
	}

	// ************* POST - Accept  - After Save ********************************
	if r.FormValue("accept") == "true" {
		estimateIDStr := r.FormValue("estimate_id")
		if estimateIDStr != "" {
			eid, err := strconv.Atoi(estimateIDStr)
			if err == nil {
				estimate = getEstimate(eid)
				estimate.CalcAllCosts()
			}
		}
		if !estimate.SaveDate.IsZero() {
			estimate.AcceptDate = time.Now()
			saveEstimate(w, r, &estimate, sd)
			log.Printf("Estimate %d accepted at %v", estimate.EstimateID, estimate.AcceptDate)
		}
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
	// Set the materials and selections based on the Deck options:
	// *****************************************************************************
	if r.FormValue("finish") != "" {
		log.Printf("Setting Finish Level to: %s", r.FormValue("finish"))
		log.Printf("Setting Stairs to: %s", r.FormValue("hasStairs"))
		// TODO - Make this a function and yaml settings
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

	// Build initial section from L/W for new estimates from calculator
	estimate.Sections = []EstimateSection{{
		Label:  "Main Deck",
		Length: estimate.Length,
		Width:  estimate.Width,
	}}

	// Calculate the costs
	estimate.CalcAllCosts()
	if estimate.Error != "" {
		renderEstimate(w, r, estimate)
		return
	}

	// Unsave - if it was previously saved - It is changed :(
	estimate.SaveDate = time.Time{}
	estimate.EstimateID = 0 // Static ID for now
	estimate.ExpirationDate = time.Time{}
	estimate.AcceptDate = time.Time{}
	estimate.Error = ""

	log.Printf("Estimate: %+v", estimate)
	// Save estimate to session
	sd.Estimate.EmailModalShown = false // Reset email modal flag
	sd.Estimate = estimate
	err = sd.Save(r, w)
	if err != nil {
		log.Printf("Estimate Handler - Save Session failed.")
	} else {
		log.Printf("estimateHandler - Session Saved.")
	}

	// Pass both estimate and customer to template
	renderEstimate(w, r, estimate)
}

func (estimate *DeckEstimate) CalcAllCosts() {

	estimate.CalculateDeckCost(costs)
	if estimate.Error != "" {
		return
	}

	estimate.CalcStairCost(costs)
	if estimate.Error != "" {
		return
	}

	estimate.CalculateRailCost(costs)
	if estimate.Error != "" {
		return
	}

	estimate.CalculateStairRailCost(costs)
	estimate.CalcStairFasciaCost(costs)
	estimate.CalcStairToeKickCost(costs)
	estimate.CalculateDemoCost(costs)
	estimate.CalculateFasciaCost(costs)
	estimate.Subtotal = estimate.DeckCost + estimate.RailCost + estimate.StairCost + estimate.StairRailCost + estimate.DemoCost + estimate.FasciaCost + estimate.StairFasciaCost + estimate.StairToeKickCost
	estimate.SalesTax = CalculateSalesTax(estimate.Subtotal, estimate.Customer.State)
	estimate.TotalCost = estimate.Subtotal + estimate.SalesTax

}

// ***********************************************************************************************
// emailSendHandler
//
//	handles the /estimate/send/{estimateID}
//	 POST - endpoint to send and render the email confirmation template.
//
// ***********************************************************************************************
func emailSendHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("emailSendHandler called")
	// This is only POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get estimateID from URL
	estimateIDStr := r.URL.Path[len("/estimate/send/"):]
	estimateID, _ := strconv.Atoi(estimateIDStr)

	// Get session data
	sd, err := GetSession(r, w)
	if err != nil {
		log.Printf("Email Handler - Get Session failed: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Check if estimate exists in session
	estimate := sd.Estimate
	if estimate.EstimateID == 0 {
		log.Printf("Email Handler - Missing estimate in session")
		http.Error(w, "No estimate available", http.StatusNotFound)
		return
	}

	// Check to see if the Estimate ID matches the URI request to email
	if estimate.EstimateID != estimateID {
		log.Printf("Email Handler - Estimate ID mismatch: URL ID=%d, Session ID=%d", estimateID, estimate.EstimateID)
		http.Error(w, "Estimate ID mismatch", http.StatusBadRequest)
		return
	}

	// Check if user is authenticated?
	// TODO - Is this the owner of the estimate?
	if !sd.UserAuth.IsAuthenticated {
		sd.UserAuth.Message = "Please Login to send estimate via Email"
		_ = sd.Save(r, w)
		loginUrl := "/login?rurl=/estimate"
		http.Redirect(w, r, loginUrl, http.StatusSeeOther)
		return
	}

	// Prepare email content
	// subject := fmt.Sprintf("Your Columbia Outdoor Deck Estimate %d – %s", estimate.EstimateID, estimate.SaveDate.Format("January 2, 2006"))
	// Subject is set in SendGrid Dynamic Template

	from := mail.NewEmail("Columbia Outdoor", "support@columbiaoutdoor.com") // Your verified SendGrid sender
	to := mail.NewEmail(estimate.Customer.FirstName+" "+estimate.Customer.LastName, estimate.Customer.Email)

	// Auto-reply using your Dynamic Template (replace with your real template ID)
	customerMessage := mail.NewV3Mail()
	customerMessage.SetFrom(from)
	customerMessage.SetReplyTo(from)
	customerMessage.SetTemplateID("d-1e52e20550794276a8b914536ee8131f") // SendGrid Dynamic Template - Deck Estimate

	// Terms and Conditions
	// Terms is not part of session
	terms, err := os.ReadFile("static/t_and_c.txt")
	if err != nil {
		// Fallback if file is missing
		terms = []byte("Terms and Conditions not available.")
	}
	estimate.Terms = string(terms)

	// TODO - Add 'cc' and 'bcc' if needed
	p := mail.NewPersonalization()
	p.AddTos(mail.NewEmail(estimate.Customer.FirstName+" "+estimate.Customer.LastName, estimate.Customer.Email))
	p.SetDynamicTemplateData("TotalCost", formatCost(estimate.TotalCost))
	p.SetDynamicTemplateData("EstimateID", estimate.EstimateID)
	p.SetDynamicTemplateData("Customer", estimate.Customer)
	p.SetDynamicTemplateData("Terms", estimate.Terms)

	// This is the list of line items to pass to the template
	var items []LineItem
	// First
	items = append(items, newLineItem("Description", estimate.Desc, 0.0))
	// 1. Main Deck
	items = append(items, newLineItem("Decking", formatDeckDescription(estimate), estimate.DeckCost))

	// 2. Demo
	items = append(items, newLineItem("Demolition", formatDemoDescription(estimate), estimate.DemoCost))

	// 3. Rails
	items = append(items, newLineItem("Deck Rails", formatRailDescription(estimate), estimate.RailCost))

	// 4. Stairs
	items = append(items, newLineItem("Stairs", formatStairDescription(estimate), estimate.StairCost))

	// 5. Stair Rails
	items = append(items, newLineItem("Stair Rails", formatStairRailDescription(estimate), estimate.StairRailCost))

	// 6. Fascia
	items = append(items, newLineItem("Deck Fascia", formatFasciaDescription(estimate), estimate.FasciaCost))
	items = append(items, newLineItem("Stair Fascia", formatStairFasciaDescription(estimate), estimate.StairFasciaCost))

	// 7. Toe Kicks
	items = append(items, newLineItem("Toe Kicks", formatStairTKDescription(estimate), estimate.StairToeKickCost))

	// 8. Subtotal
	items = append(items, newLineItem("Subtotal", "", estimate.Subtotal))

	// 9. Sales Tax
	taxDesc := estimate.Customer.State + " (estimated) sales tax"
	if estimate.Customer.State == "" {
		taxDesc = "Sales tax"
	}
	items = append(items, newLineItem("Sales Tax", taxDesc, estimate.SalesTax))
	p.SetDynamicTemplateData("LineItems", items)
	customerMessage.AddPersonalizations(p)

	log.Printf("Preparing to send estimate %d email to: %s", estimate.EstimateID, estimate.Customer.Email)
	log.Printf("From: %s To: %s", from.Address, to.Address)

	// SendGrid email sending logic goes here
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		log.Printf("Unable to send email: SENDGRID_API_KEY is required")
		w.WriteHeader(http.StatusRequestTimeout) // failed
		return
	}

	// Send emails in background
	go func() {
		sg = sendgrid.NewSendClient(apiKey)
		customerRR, err := sg.Send(customerMessage)
		if err != nil {
			log.Printf("Auto-reply failed: %v", err)
		}
		if customerRR.StatusCode >= 300 {
			log.Printf("customer send response is: %v", customerRR)
		}

		log.Printf("emailHandler - Completed successfully for Estimate ID=%d", estimate.EstimateID)
		// w.WriteHeader(http.StatusOK) // or just write JSON (defaults to 200)
		// w.WriteHeader(http.StatusOK)
	}()

	// Return a message / 200 back to JavaScript function
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "queued",
		"message": "Estimate is being sent",
	})

}
