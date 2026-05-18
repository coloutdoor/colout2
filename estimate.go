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
	resend "github.com/resend/resend-go/v2"
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
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	estimateIDStr := r.URL.Path[len("/estimate/send/"):]
	estimateID, _ := strconv.Atoi(estimateIDStr)

	sd, err := GetSession(r, w)
	if err != nil || !sd.UserAuth.IsAuthenticated {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Please log in first."})
		return
	}

	// Load fresh from DB so sections and costs are current
	estimate := getEstimate(estimateID)
	if estimate.Error != "" || estimate.EstimateID == 0 {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Estimate not found."})
		return
	}
	estimate.CalcAllCosts()

	// Read email address from request body
	var req struct{ Email string `json:"email"` }
	json.NewDecoder(r.Body).Decode(&req)
	toEmail := req.Email
	if toEmail == "" {
		toEmail = estimate.Customer.Email
	}

	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Printf("emailSendHandler: RESEND_API_KEY not set")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Email service is not configured. Please call us at (360) 787-8062.",
		})
		return
	}

	subject := fmt.Sprintf("Deck Estimate #%d-%d — %s", estimate.EstimateID, estimate.Version, estimate.Desc)
	html := buildEstimateEmailHTML(estimate)

	client := resend.NewClient(apiKey)
	params := &resend.SendEmailRequest{
		From:    "Columbia Outdoor <support@columbiaoutdoor.com>",
		To:      []string{toEmail},
		Subject: subject,
		Html:    html,
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		log.Printf("emailSendHandler: resend error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Email could not be sent. Please call us at (360) 787-8062.",
		})
		return
	}

	log.Printf("emailSendHandler: sent estimate %d to %s (id: %s)", estimate.EstimateID, toEmail, sent.Id)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "sent",
		"message": "Estimate emailed to " + toEmail,
	})
}

// buildEstimateEmailHTML builds a clean HTML email for the estimate.
func buildEstimateEmailHTML(e DeckEstimate) string {
	taxDesc := e.Customer.State + " sales tax"
	if e.Customer.State == "" {
		taxDesc = "Sales tax"
	}

	lineItems := []LineItem{
		newLineItem("Deck", formatDeckDescription(e), e.DeckCost),
		newLineItem("Demolition", formatDemoDescription(e), e.DemoCost),
		newLineItem("Rails", formatRailDescription(e), e.RailCost),
		newLineItem("Fascia", formatFasciaDescription(e), e.FasciaCost),
		newLineItem("Stairs", formatStairDescription(e), e.StairCost),
		newLineItem("Stair Rails", formatStairRailDescription(e), e.StairRailCost),
		newLineItem("Stair Fascia", formatStairFasciaDescription(e), e.StairFasciaCost),
		newLineItem("Toe Kicks", formatStairTKDescription(e), e.StairToeKickCost),
	}

	rows := ""
	for _, item := range lineItems {
		rows += fmt.Sprintf(`<tr>
			<td style="padding:8px;border-bottom:1px solid #eee;vertical-align:top">%s</td>
			<td style="padding:8px;border-bottom:1px solid #eee;vertical-align:top;color:#555;font-size:13px">%s</td>
			<td style="padding:8px;border-bottom:1px solid #eee;text-align:right;white-space:nowrap">%s</td>
		</tr>`, item.Name, strings.ReplaceAll(item.Description, "\n", "<br>"), formatCost(item.Cost))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"></head>
<body style="font-family:Arial,sans-serif;color:#333;max-width:640px;margin:0 auto;padding:20px">

  <div style="background:#2c6e9e;color:white;padding:24px;border-radius:6px 6px 0 0">
    <h1 style="margin:0;font-size:22px">Columbia Outdoor</h1>
    <p style="margin:4px 0 0;opacity:.8">(360) 787-8062 · columbiaoutdoor.com</p>
  </div>

  <div style="background:#f8f8f8;padding:16px;border-left:4px solid #2c6e9e">
    <strong>Estimate #%d-%d</strong> &mdash; %s<br>
    <span style="color:#888;font-size:13px">Prepared: %s &nbsp;·&nbsp; Expires: %s</span>
  </div>

  <div style="display:flex;gap:20px;margin:20px 0">
    <div style="flex:1">
      <strong>Customer</strong><br>
      %s %s<br>%s<br>%s, %s %s<br>%s<br>%s
    </div>
  </div>

  <h2 style="font-size:16px;border-bottom:2px solid #2c6e9e;padding-bottom:6px">Scope of Work</h2>
  <table style="width:100%%;border-collapse:collapse">
    <thead><tr style="background:#f0f0f0">
      <th style="padding:8px;text-align:left">Item</th>
      <th style="padding:8px;text-align:left">Description</th>
      <th style="padding:8px;text-align:right">Cost</th>
    </tr></thead>
    <tbody>%s</tbody>
    <tfoot>
      <tr><td colspan="2" style="padding:8px;text-align:right"><strong>Subtotal</strong></td>
          <td style="padding:8px;text-align:right">%s</td></tr>
      <tr><td colspan="2" style="padding:8px;text-align:right">%s</td>
          <td style="padding:8px;text-align:right">%s</td></tr>
      <tr style="background:#333;color:white">
        <td colspan="2" style="padding:12px;text-align:right"><strong>Total</strong></td>
        <td style="padding:12px;text-align:right"><strong>%s</strong></td>
      </tr>
    </tfoot>
  </table>

  <div style="margin-top:24px;padding:16px;background:#f9f9f9;border:1px solid #ddd;font-size:12px;color:#666">
    <strong>Terms &amp; Conditions</strong><br>
    This estimate is valid until %s. To accept, please contact us at (360) 787-8062 or reply to this email.
  </div>

</body></html>`,
		e.EstimateID, e.Version, e.Desc,
		e.SaveDate.Format("Jan 2, 2006"),
		e.ExpirationDate.Format("Jan 2, 2006"),
		e.Customer.FirstName, e.Customer.LastName,
		e.Customer.Address,
		e.Customer.City, e.Customer.State, e.Customer.Zip,
		e.Customer.PhoneNumber, e.Customer.Email,
		rows,
		formatCost(e.Subtotal),
		taxDesc, formatCost(e.SalesTax),
		formatCost(e.TotalCost),
		e.ExpirationDate.Format("Jan 2, 2006"),
	)
}
