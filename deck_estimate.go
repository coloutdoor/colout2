package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() {
	gob.Register(DeckEstimate{})
}

// DeckEstimate holds all data for a deck cost estimate.
type DeckEstimate struct {
	Desc             string
	Length           float64
	Width            float64
	Height           float64
	DeckArea         float64
	ProductType      string // "deck" or "patio_cover" — discriminator for future product types
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
	TermsHTML        template.HTML
	Error            string
	EmailModalShown  bool  // Flag to indicate if email modal should be shown
	UserId           int64 // FK to UserAuth
	Status           string // Accepted, Expired, or Pending — computed in renderEstimate
	Version          int    // Increments on each save
	Sections         []EstimateSection
	RailFeetOverride float64 // 0 = auto-calculate from primary section
	AccessToken      string  // Random token for customer view/accept link
	IsPublicView     bool    // True when accessed via token link — hides edit controls
	AcceptURL        string  // Form action for accept modal; defaults to /estimate
	DiscountCode     string
	DiscountAmount   float64
	PermitLevel      int     // 0=none, 1=design, 2=design+eng, 3=design+eng+permits
	PermitCost       float64
	CustomItems      []EstimateCustomItem
	CustomItemsTotal float64
	DIYMode          int // 0=Full Service, 1=Plans+Materials, 2=Plans Only
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
        COALESCE(e.rail_feet_override, 0),
        COALESCE(cp.company_name,''), COALESCE(cp.phone,''), COALESCE(cp.website,''),
        COALESCE(cp.license_number,''), COALESCE(cp.license_state,''), COALESCE(cp.id,1),
        COALESCE(e.deck_cost, 0), COALESCE(e.deck_area, 0), COALESCE(e.rail_cost, 0), COALESCE(e.rail_feet, 0),
        COALESCE(e.stair_cost, 0), COALESCE(e.stair_rail_cost, 0), COALESCE(e.fascia_cost, 0), COALESCE(e.fascia_feet, 0),
        COALESCE(e.stair_fascia_cost, 0), COALESCE(e.stair_toe_kick_cost, 0), COALESCE(e.demo_cost, 0),
        COALESCE(e.subtotal, 0), COALESCE(e.sales_tax, 0),
        COALESCE(e.access_token, ''),
        COALESCE(e.discount_code, ''), COALESCE(e.discount_amount, 0),
        COALESCE(e.permit_level, 0), COALESCE(e.permit_cost, 0),
        COALESCE(e.diy_mode, 0), e.product_type
        FROM estimates e
        LEFT JOIN contractor_profile cp ON cp.id = e.contractor_id
        WHERE e.estimate_id = $1`, estimateID).Scan(
		&de.EstimateID, &de.Desc, &de.Height, &de.Material, &de.RailMaterial, &de.RailInfill, &de.StairWidth,
		&de.StairRailCount, &de.HasDemo, &de.HasFascia, &de.TotalCost, &de.HasStairFascia, &de.HasStairTK,
		&de.Customer.FirstName, &de.Customer.LastName, &de.Customer.Address, &de.Customer.City, &de.Customer.State,
		&de.Customer.Zip, &de.Customer.PhoneNumber, &de.Customer.Email,
		&de.SaveDate, &acceptDate, &de.ExpirationDate, &de.UserId, &de.ContractorID, &de.Version, &de.RailFeetOverride,
		&de.Contractor.CompanyName, &de.Contractor.Phone, &de.Contractor.Website,
		&de.Contractor.LicenseNum, &de.Contractor.LicenseState, &de.Contractor.ID,
		&de.DeckCost, &de.DeckArea, &de.RailCost, &de.RailFeet,
		&de.StairCost, &de.StairRailCost, &de.FasciaCost, &de.FasciaFeet,
		&de.StairFasciaCost, &de.StairToeKickCost, &de.DemoCost,
		&de.Subtotal, &de.SalesTax,
		&de.AccessToken, &de.DiscountCode, &de.DiscountAmount,
		&de.PermitLevel, &de.PermitCost, &de.DIYMode, &de.ProductType)

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

	// Load custom items
	cirows, cierr := db.Query(`
		SELECT id, estimate_id, description, COALESCE(notes,''), cost, sort_order
		FROM estimate_custom_items WHERE estimate_id = $1
		ORDER BY sort_order, id`, estimateID)
	if cierr == nil {
		for cirows.Next() {
			var ci EstimateCustomItem
			if err := cirows.Scan(&ci.ID, &ci.EstimateID, &ci.Description, &ci.Notes, &ci.Cost, &ci.SortOrder); err == nil {
				de.CustomItems = append(de.CustomItems, ci)
			}
		}
		cirows.Close()
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

// getEstimateByToken loads an estimate by its public access token.
func getEstimateByToken(token string) DeckEstimate {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return DeckEstimate{Error: "Database Env - not set up."}
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return DeckEstimate{Error: "Database Connect failed."}
	}
	defer db.Close()

	var de DeckEstimate
	var acceptDate sql.NullTime
	err = db.QueryRow(`
        SELECT e.estimate_id, e.description, e.height, e.material, e.rail_material, e.rail_infill, e.stair_width,
        e.stair_rail_count, e.has_demo, e.has_fascia, e.total_cost, e.has_stair_fascia, e.has_stair_tk,
        e.first_name, e.last_name, e.address, e.city, e.state, e.zip, e.phone_number, e.email,
        e.save_date, e.accept_date, e.expiration_date, e.user_id, e.contractor_id, e.version,
        COALESCE(e.rail_feet_override, 0),
        COALESCE(cp.company_name,''), COALESCE(cp.phone,''), COALESCE(cp.website,''),
        COALESCE(cp.license_number,''), COALESCE(cp.license_state,''), COALESCE(cp.id,1),
        COALESCE(e.deck_cost, 0), COALESCE(e.deck_area, 0), COALESCE(e.rail_cost, 0), COALESCE(e.rail_feet, 0),
        COALESCE(e.stair_cost, 0), COALESCE(e.stair_rail_cost, 0), COALESCE(e.fascia_cost, 0), COALESCE(e.fascia_feet, 0),
        COALESCE(e.stair_fascia_cost, 0), COALESCE(e.stair_toe_kick_cost, 0), COALESCE(e.demo_cost, 0),
        COALESCE(e.subtotal, 0), COALESCE(e.sales_tax, 0),
        COALESCE(e.access_token, ''),
        COALESCE(e.discount_code, ''), COALESCE(e.discount_amount, 0),
        COALESCE(e.permit_level, 0), COALESCE(e.permit_cost, 0),
        COALESCE(e.diy_mode, 0), e.product_type
        FROM estimates e
        LEFT JOIN contractor_profile cp ON cp.id = e.contractor_id
        WHERE e.access_token = $1`, token).Scan(
		&de.EstimateID, &de.Desc, &de.Height, &de.Material, &de.RailMaterial, &de.RailInfill, &de.StairWidth,
		&de.StairRailCount, &de.HasDemo, &de.HasFascia, &de.TotalCost, &de.HasStairFascia, &de.HasStairTK,
		&de.Customer.FirstName, &de.Customer.LastName, &de.Customer.Address, &de.Customer.City, &de.Customer.State,
		&de.Customer.Zip, &de.Customer.PhoneNumber, &de.Customer.Email,
		&de.SaveDate, &acceptDate, &de.ExpirationDate, &de.UserId, &de.ContractorID, &de.Version, &de.RailFeetOverride,
		&de.Contractor.CompanyName, &de.Contractor.Phone, &de.Contractor.Website,
		&de.Contractor.LicenseNum, &de.Contractor.LicenseState, &de.Contractor.ID,
		&de.DeckCost, &de.DeckArea, &de.RailCost, &de.RailFeet,
		&de.StairCost, &de.StairRailCost, &de.FasciaCost, &de.FasciaFeet,
		&de.StairFasciaCost, &de.StairToeKickCost, &de.DemoCost,
		&de.Subtotal, &de.SalesTax,
		&de.AccessToken, &de.DiscountCode, &de.DiscountAmount,
		&de.PermitLevel, &de.PermitCost, &de.DIYMode, &de.ProductType)

	if err != nil {
		return DeckEstimate{Error: "Estimate not found"}
	}

	if acceptDate.Valid {
		de.AcceptDate = acceptDate.Time
	}

	srows, serr := db.Query(`
		SELECT id, estimate_id, label, length, width, sort_order
		FROM estimate_sections WHERE estimate_id = $1
		ORDER BY sort_order, id`, de.EstimateID)
	if serr == nil {
		for srows.Next() {
			var s EstimateSection
			if err := srows.Scan(&s.ID, &s.EstimateID, &s.Label, &s.Length, &s.Width, &s.SortOrder); err == nil {
				de.Sections = append(de.Sections, s)
			}
		}
		srows.Close()
	}

	cirows, cierr := db.Query(`
		SELECT id, estimate_id, description, COALESCE(notes,''), cost, sort_order
		FROM estimate_custom_items WHERE estimate_id = $1
		ORDER BY sort_order, id`, de.EstimateID)
	if cierr == nil {
		for cirows.Next() {
			var ci EstimateCustomItem
			if err := cirows.Scan(&ci.ID, &ci.EstimateID, &ci.Description, &ci.Notes, &ci.Cost, &ci.SortOrder); err == nil {
				de.CustomItems = append(de.CustomItems, ci)
			}
		}
		cirows.Close()
	}

	if len(de.Sections) > 0 {
		de.Length = de.Sections[0].Length
		de.Width = de.Sections[0].Width
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
		// Preserve the form-updated estimate so it survives the login redirect.
		sd.Estimate = *estimate
		sd.PendingSave = true
		if err = sd.Save(r, w); err != nil {
			log.Printf("Unable to save session before login redirect")
		}
		_ = db.Close()
		http.Redirect(w, r, "/login?rurl=/estimate", http.StatusSeeOther)
		return
	}

	estimate.UserId = sessionData.UserAuth.ID
	estimate.SaveDate = time.Now()
	estimate.ExpirationDate = estimate.SaveDate.Add(30 * 24 * time.Hour)
	if estimate.ProductType == "" {
		estimate.ProductType = "deck"
	}

	// Set contractor_id: for new estimates always re-derive from session to prevent
	// session pollution from previously viewed estimates; for updates preserve existing.
	if estimate.EstimateID == 0 || estimate.ContractorID == 0 {
		estimate.ContractorID = 1 // default: Columbia Outdoor
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
    rail_feet_override = $26,
    deck_cost = $27,
    deck_area = $28,
    rail_cost = $29,
    rail_feet = $30,
    stair_cost = $31,
    stair_rail_cost = $32,
    fascia_cost = $33,
    fascia_feet = $34,
    stair_fascia_cost = $35,
    stair_toe_kick_cost = $36,
    demo_cost = $37,
    subtotal = $38,
    sales_tax = $39,
    access_token = $40,
    discount_code = $41,
    discount_amount = $42,
    permit_level = $43,
    permit_cost = $44,
    diy_mode = $45,
    product_type = $46,
    version = version + 1
WHERE estimate_id = $47
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
			estimate.UserId,           //24
			estimate.ContractorID,     //25
			estimate.RailFeetOverride, //26
			estimate.DeckCost, estimate.DeckArea, estimate.RailCost, estimate.RailFeet,           //30
			estimate.StairCost, estimate.StairRailCost, estimate.FasciaCost, estimate.FasciaFeet, //34
			estimate.StairFasciaCost, estimate.StairToeKickCost, estimate.DemoCost,               //37
			estimate.Subtotal, estimate.SalesTax,                                                 //39
			estimate.AccessToken,                                                                 //40
			estimate.DiscountCode, estimate.DiscountAmount,   //42
			estimate.PermitLevel, estimate.PermitCost,         //44
			estimate.DIYMode,                                  //45
			estimate.ProductType,                              //46
			estimate.EstimateID).Scan(&updatedID, &estimate.Version)

		if err != nil {
			log.Printf("Failed to prepare statement to update estimate: %v", err)
			_ = db.Close()
			renderEstimate(w, r, DeckEstimate{Error: "Database error: Update Estimate failed."})
			return
		}

		_ = db.Close()
		log.Printf("Estimate updated: ID=%d, SaveDate=%v, ExpirationDate=%v", estimate.EstimateID, estimate.SaveDate, estimate.ExpirationDate)
		recordNREvent("EstimateUpdated", map[string]interface{}{
			"estimate_id":   estimate.EstimateID,
			"total_cost":    estimate.TotalCost,
			"material":      estimate.Material,
			"user_id":       estimate.UserId,
			"contractor_id": estimate.ContractorID,
		})
	} else {

		// Create NEW Estimate — always generate a fresh token to avoid session bleed-through
		b := make([]byte, 16)
		rand.Read(b)
		estimate.AccessToken = hex.EncodeToString(b)
		log.Printf("Inserting new estimate")
		//Prepared Statement - PostgreSQL handle the ID
		stmt := `INSERT INTO estimates (
			description, height,
			material, rail_material, rail_infill,
			stair_width, stair_rail_count, has_demo, has_fascia, total_cost,
			first_name, last_name, address,
			city, state, zip, phone_number, email,
			save_date, accept_date, expiration_date, has_stair_fascia, has_stair_tk,
			user_id, contractor_id, rail_feet_override,
			deck_cost, deck_area, rail_cost, rail_feet,
			stair_cost, stair_rail_cost, fascia_cost, fascia_feet,
			stair_fascia_cost, stair_toe_kick_cost, demo_cost, subtotal, sales_tax,
			access_token, discount_code, discount_amount, permit_level, permit_cost, diy_mode, product_type, version)
		VALUES (
		$1, $2,
		$3, $4, $5,
		$6, $7, $8, $9, $10,
		$11, $12, $13, $14, $15, $16, $17, $18,
		$19, $20, $21,
		$22, $23, $24, $25, $26,
		$27, $28, $29, $30,
		$31, $32, $33, $34,
		$35, $36, $37, $38, $39,
		$40, $41, $42, $43, $44, $45, $46, 1
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
			estimate.HasStairFascia, estimate.HasStairTK,                                                          //23
			estimate.UserId, estimate.ContractorID, estimate.RailFeetOverride,                                     //26
			estimate.DeckCost, estimate.DeckArea, estimate.RailCost, estimate.RailFeet,                            //30
			estimate.StairCost, estimate.StairRailCost, estimate.FasciaCost, estimate.FasciaFeet,                  //34
			estimate.StairFasciaCost, estimate.StairToeKickCost, estimate.DemoCost, estimate.Subtotal, estimate.SalesTax, //39
			estimate.AccessToken, estimate.DiscountCode, estimate.DiscountAmount, //42
			estimate.PermitLevel, estimate.PermitCost,                           //44
			estimate.DIYMode, estimate.ProductType).                             //46
			Scan(&newID)
		if err != nil {
			log.Printf("Failed to save estimate to DB: %v", err)
			_ = db.Close()
			renderEstimate(w, r, DeckEstimate{Error: "Database error: Save Estimate failed."})
			return
		}
		estimate.EstimateID = int(newID)
		_ = db.Close()
		recordNREvent("EstimateCreated", map[string]interface{}{
			"estimate_id":   estimate.EstimateID,
			"total_cost":    estimate.TotalCost,
			"material":      estimate.Material,
			"user_id":       estimate.UserId,
			"contractor_id": estimate.ContractorID,
		})
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

	// Save custom items (always run to clear deleted items)
	db3, err3 := sql.Open("pgx", dbURL)
	if err3 == nil {
		db3.Exec(`DELETE FROM estimate_custom_items WHERE estimate_id = $1`, estimate.EstimateID)
		for i, ci := range estimate.CustomItems {
			if ci.Description != "" || ci.Cost != 0 {
				db3.Exec(`INSERT INTO estimate_custom_items (estimate_id, description, notes, cost, sort_order)
					VALUES ($1, $2, $3, $4, $5)`,
					estimate.EstimateID, ci.Description, ci.Notes, ci.Cost, i)
			}
		}
		db3.Close()
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
	estimate.CalcPermitCost(costs)

	switch estimate.DIYMode {
	case 1: // Plans + Materials: homeowner installs, we supply materials at 50% of full-service
		estimate.DemoCost = 0
		estimate.DeckCost *= 0.5
		estimate.RailCost *= 0.5
		estimate.StairCost *= 0.5
		estimate.StairRailCost *= 0.5
		estimate.FasciaCost *= 0.5
		estimate.StairFasciaCost *= 0.5
		estimate.StairToeKickCost *= 0.5
	case 2: // Plans Only: design/engineering/permits only — no materials
		estimate.DemoCost = 0
		estimate.DeckCost = 0
		estimate.RailCost = 0
		estimate.StairCost = 0
		estimate.StairRailCost = 0
		estimate.FasciaCost = 0
		estimate.StairFasciaCost = 0
		estimate.StairToeKickCost = 0
	}

	estimate.CustomItemsTotal = 0
	for _, ci := range estimate.CustomItems {
		estimate.CustomItemsTotal += ci.Cost
	}
	estimate.Subtotal = estimate.DeckCost + estimate.RailCost + estimate.StairCost + estimate.StairRailCost + estimate.DemoCost + estimate.FasciaCost + estimate.StairFasciaCost + estimate.StairToeKickCost + estimate.PermitCost + estimate.CustomItemsTotal

	estimate.DiscountAmount = 0
	if estimate.DiscountCode != "" {
		code := strings.ToUpper(strings.TrimSpace(estimate.DiscountCode))
		if rate, ok := costs.DiscountCodes[code]; ok {
			estimate.DiscountAmount = estimate.Subtotal * rate
			estimate.DiscountCode = code
		}
	}

	estimate.SalesTax = CalculateSalesTax(estimate.Subtotal-estimate.DiscountAmount, estimate.Customer.State)
	estimate.TotalCost = estimate.Subtotal - estimate.DiscountAmount + estimate.SalesTax

}

type estimateEmailData struct {
	Estimate           DeckEstimate
	EstimateURL        string
	ContractorName     string
	ContractorPhone    string
	ContractorWebsite  string
	ContractorLicense  string
}

// buildEstimateEmailHTML renders the estimate email template to a string.
func buildEstimateEmailHTML(e DeckEstimate, estimateURL string) string {
	data := estimateEmailData{
		Estimate:          e,
		EstimateURL:       estimateURL,
		ContractorName:    "Columbia Outdoor",
		ContractorPhone:   "(360) 787-8062",
		ContractorWebsite: "columbiaoutdoor.com",
	}
	if e.Contractor.CompanyName != "" {
		data.ContractorName = e.Contractor.CompanyName
	}
	if e.Contractor.Phone != "" {
		data.ContractorPhone = e.Contractor.Phone
	}
	if e.Contractor.Website != "" {
		data.ContractorWebsite = e.Contractor.Website
	}
	if e.Contractor.LicenseNum != "" {
		data.ContractorLicense = fmt.Sprintf("%s (%s)", e.Contractor.LicenseNum, e.Contractor.LicenseState)
	}

	tmpl := template.Must(template.New("email-estimate.gohtml").Funcs(funcMap).ParseFiles("templates/email-estimate.gohtml"))
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "email-estimate.gohtml", data); err != nil {
		log.Printf("buildEstimateEmailHTML: template error: %v", err)
		return "<p>Error rendering estimate email.</p>"
	}
	return buf.String()
}
