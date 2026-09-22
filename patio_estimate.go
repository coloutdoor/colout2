package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/yuin/goldmark"
)

func init() {
	gob.Register(PatioCoverEstimate{})
}

// PatioCoverEstimate holds all data for a patio cover cost estimate.
type PatioCoverEstimate struct {
	Desc           string
	ProductType    string // always "patio_cover"
	Width          float64
	Depth          float64
	PatioType      string // pergola | leanto | truss | timberframe
	Area           float64
	BaseCost       float64
	Subtotal       float64
	SalesTax       float64
	TotalCost      float64
	DIYMode        int // 0=Full Service, 1=Plans+Materials, 2=Plans Only
	Customer       Customer
	Contractor     ContractorInfo
	ContractorID   int64
	EstimateID     int
	UserId         int64 // FK to UserAuth
	SaveDate       time.Time
	ExpirationDate time.Time
	AcceptDate     time.Time
	AccessToken    string
	Version        int
	Terms          string
	TermsHTML      template.HTML
	PatioTypeLabel string // human-readable PatioType, computed in renderPatioEstimate
	Error          string
}

// patioTypeLabels maps the stored PatioType slug to its display label.
var patioTypeLabels = map[string]string{
	"pergola":     "Pergola",
	"leanto":      "Lean-to",
	"truss":       "Truss",
	"timberframe": "Timberframe",
}

// patioDetails is the JSON shape stored in estimates.product_details for a
// patio_cover row — the fields that have no dedicated (deck-shaped) column.
type patioDetails struct {
	Width     float64 `json:"width"`
	Depth     float64 `json:"depth"`
	PatioType string  `json:"patioType"`
	Area      float64 `json:"area"`
	BaseCost  float64 `json:"baseCost"`
}

// CalcAllCosts computes the base cost, sales tax, and total for a patio cover estimate.
func (estimate *PatioCoverEstimate) CalcAllCosts() {
	estimate.CalculatePatioCost(costs)
	if estimate.Error != "" {
		return
	}

	estimate.Subtotal = estimate.BaseCost
	estimate.SalesTax = CalculateSalesTax(estimate.Subtotal, estimate.Customer.State)
	estimate.TotalCost = estimate.Subtotal + estimate.SalesTax
}

// getPatioEstimate loads a patio cover estimate from the DB by estimate_id.
func getPatioEstimate(estimateID int) PatioCoverEstimate {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return PatioCoverEstimate{Error: "Database Env - not set up."}
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return PatioCoverEstimate{Error: "Database Connect failed."}
	}
	defer db.Close()

	var pe PatioCoverEstimate
	var acceptDate sql.NullTime
	var detailsJSON []byte
	err = db.QueryRow(`
        SELECT e.estimate_id, e.description, COALESCE(e.total_cost,0), COALESCE(e.subtotal,0), COALESCE(e.sales_tax,0),
        e.first_name, e.last_name, e.address, e.city, e.state, e.zip, e.phone_number, e.email,
        e.save_date, e.accept_date, e.expiration_date, e.user_id, e.contractor_id, e.version,
        COALESCE(e.access_token, ''), COALESCE(e.diy_mode, 0), e.product_type,
        COALESCE(cp.company_name,''), COALESCE(cp.phone,''), COALESCE(cp.website,''),
        COALESCE(cp.license_number,''), COALESCE(cp.license_state,''), COALESCE(cp.id,1),
        COALESCE(e.product_details::text, '{}')
        FROM estimates e
        LEFT JOIN contractor_profile cp ON cp.id = e.contractor_id
        WHERE e.estimate_id = $1`, estimateID).Scan(
		&pe.EstimateID, &pe.Desc, &pe.TotalCost, &pe.Subtotal, &pe.SalesTax,
		&pe.Customer.FirstName, &pe.Customer.LastName, &pe.Customer.Address, &pe.Customer.City, &pe.Customer.State,
		&pe.Customer.Zip, &pe.Customer.PhoneNumber, &pe.Customer.Email,
		&pe.SaveDate, &acceptDate, &pe.ExpirationDate, &pe.UserId, &pe.ContractorID, &pe.Version,
		&pe.AccessToken, &pe.DIYMode, &pe.ProductType,
		&pe.Contractor.CompanyName, &pe.Contractor.Phone, &pe.Contractor.Website,
		&pe.Contractor.LicenseNum, &pe.Contractor.LicenseState, &pe.Contractor.ID,
		&detailsJSON)

	if err != nil {
		log.Printf("getPatioEstimate query error: %v", err)
		return PatioCoverEstimate{Error: "Estimate not found"}
	}

	if acceptDate.Valid {
		pe.AcceptDate = acceptDate.Time
	}

	var d patioDetails
	if err := json.Unmarshal(detailsJSON, &d); err == nil {
		pe.Width = d.Width
		pe.Depth = d.Depth
		pe.PatioType = d.PatioType
		pe.Area = d.Area
		pe.BaseCost = d.BaseCost
	}

	pe.Error = ""
	return pe
}

// savePatioEstimate persists a patio cover estimate to the DB and session.
// Requires an authenticated session; unauthenticated requests are redirected to
// /login and the in-progress estimate is preserved in session for after login.
func savePatioEstimate(w http.ResponseWriter, r *http.Request, estimate *PatioCoverEstimate, sd *SessionData) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Printf("DATABASE_URL environment variable is required")
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Database Env - not set up."})
		return
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("Unable to connect to database: %v", err)
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Database Connect failed."})
		return
	}
	defer db.Close()

	sessionData, err := GetSession(r, w)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	if !sessionData.UserAuth.IsAuthenticated {
		sd.PatioEstimate = *estimate
		sd.PendingSave = true
		if err = sd.Save(r, w); err != nil {
			log.Printf("Unable to save session before login redirect")
		}
		http.Redirect(w, r, "/login?rurl=/patio-estimate", http.StatusSeeOther)
		return
	}

	estimate.UserId = sessionData.UserAuth.ID
	estimate.SaveDate = time.Now()
	estimate.ExpirationDate = estimate.SaveDate.Add(30 * 24 * time.Hour)
	estimate.ProductType = "patio_cover"

	if estimate.EstimateID == 0 || estimate.ContractorID == 0 {
		estimate.ContractorID = 1 // default: Columbia Outdoor
		if sessionData.UserAuth.Role == "contractor" {
			db.QueryRow(`SELECT id FROM contractor_profile WHERE user_id = $1`, estimate.UserId).Scan(&estimate.ContractorID)
		}
	}

	detailsJSON, err := json.Marshal(patioDetails{
		Width:     estimate.Width,
		Depth:     estimate.Depth,
		PatioType: estimate.PatioType,
		Area:      estimate.Area,
		BaseCost:  estimate.BaseCost,
	})
	if err != nil {
		log.Printf("savePatioEstimate: failed to marshal product_details: %v", err)
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Database error: Save Estimate failed."})
		return
	}

	if estimate.EstimateID > 0 {
		log.Printf("Updating existing patio estimate ID=%d", estimate.EstimateID)
		stmt := `UPDATE estimates
SET
    description = $1,
    total_cost = $2,
    subtotal = $3,
    sales_tax = $4,
    first_name = $5,
    last_name = $6,
    address = $7,
    city = $8,
    state = $9,
    zip = $10,
    phone_number = $11,
    email = $12,
    save_date = $13,
    expiration_date = $14,
    user_id = $15,
    contractor_id = $16,
    access_token = $17,
    diy_mode = $18,
    product_type = $19,
    product_details = $20,
    version = version + 1
WHERE estimate_id = $21
RETURNING estimate_id, version`
		var updatedID int64
		err = db.QueryRow(stmt, estimate.Desc, estimate.TotalCost, estimate.Subtotal, estimate.SalesTax,
			estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address,
			estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip,
			estimate.Customer.PhoneNumber, estimate.Customer.Email,
			estimate.SaveDate.Format("2006-01-02 15:04:05"),
			estimate.ExpirationDate.Format("2006-01-02 15:04:05"),
			estimate.UserId, estimate.ContractorID, estimate.AccessToken,
			estimate.DIYMode, estimate.ProductType, string(detailsJSON),
			estimate.EstimateID).Scan(&updatedID, &estimate.Version)
		if err != nil {
			log.Printf("Failed to update patio estimate: %v", err)
			renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Database error: Update Estimate failed."})
			return
		}
		recordNREvent("PatioEstimateUpdated", map[string]interface{}{
			"estimate_id":   estimate.EstimateID,
			"total_cost":    estimate.TotalCost,
			"patio_type":    estimate.PatioType,
			"user_id":       estimate.UserId,
			"contractor_id": estimate.ContractorID,
		})
	} else {
		b := make([]byte, 16)
		rand.Read(b)
		estimate.AccessToken = hex.EncodeToString(b)
		log.Printf("Inserting new patio estimate")
		stmt := `INSERT INTO estimates (
			description, total_cost, subtotal, sales_tax,
			first_name, last_name, address, city, state, zip, phone_number, email,
			save_date, accept_date, expiration_date,
			user_id, contractor_id,
			access_token, diy_mode, product_type, product_details, version)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15,
			$16, $17,
			$18, $19, $20, $21, 1
		) RETURNING estimate_id`
		var newID int64
		err = db.QueryRow(stmt,
			estimate.Desc, estimate.TotalCost, estimate.Subtotal, estimate.SalesTax,
			estimate.Customer.FirstName, estimate.Customer.LastName, estimate.Customer.Address,
			estimate.Customer.City, estimate.Customer.State, estimate.Customer.Zip,
			estimate.Customer.PhoneNumber, estimate.Customer.Email,
			estimate.SaveDate.Format("2006-01-02 15:04:05"),
			nil,
			estimate.ExpirationDate.Format("2006-01-02 15:04:05"),
			estimate.UserId, estimate.ContractorID,
			estimate.AccessToken, estimate.DIYMode, estimate.ProductType, string(detailsJSON)).
			Scan(&newID)
		if err != nil {
			log.Printf("Failed to save patio estimate to DB: %v", err)
			renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Database error: Save Estimate failed."})
			return
		}
		estimate.EstimateID = int(newID)
		recordNREvent("PatioEstimateCreated", map[string]interface{}{
			"estimate_id":   estimate.EstimateID,
			"total_cost":    estimate.TotalCost,
			"patio_type":    estimate.PatioType,
			"user_id":       estimate.UserId,
			"contractor_id": estimate.ContractorID,
		})
	}

	sd.PatioEstimate = *estimate
	if err := sd.Save(r, w); err != nil {
		log.Printf("Failed to save Session Data in Patio Estimate - savePatioEstimate()")
	}

	log.Printf("Patio estimate saved: ID=%d, SaveDate=%v, ExpirationDate=%v", estimate.EstimateID, estimate.SaveDate, estimate.ExpirationDate)
}

// renderPatioEstimate loads the terms and conditions and executes
// estimate-patio.gohtml. Template execution errors are logged and reported to
// the client as a 500.
func renderPatioEstimate(w http.ResponseWriter, r *http.Request, estimate PatioCoverEstimate) {
	if label, ok := patioTypeLabels[estimate.PatioType]; ok {
		estimate.PatioTypeLabel = label
	} else {
		estimate.PatioTypeLabel = estimate.PatioType
	}

	if mdBytes, err := os.ReadFile("static/t_and_c.md"); err == nil {
		var buf bytes.Buffer
		if err := goldmark.Convert(mdBytes, &buf); err == nil {
			estimate.TermsHTML = template.HTML(buf.String())
		}
	}
	if estimate.TermsHTML == "" {
		estimate.Terms = "Terms and Conditions not available."
	}

	userAuth := getUserAuth(r, w)
	if estimate.EstimateID > 0 {
		userAuth.Title = fmt.Sprintf("Patio Cover Estimate #%d", estimate.EstimateID)
	} else {
		userAuth.Title = "Patio Cover Estimate"
	}
	userAuth.CanonicalPath = "/patio-estimate"
	rd := renderData{
		Page:   &estimate,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("estimate-patio.gohtml").Funcs(funcMap).ParseFiles("templates/estimate-patio.gohtml",
		"templates/header.gohtml", "templates/footer.gohtml"))
	if err := tmpl.ExecuteTemplate(w, "estimate-patio.gohtml", rd); err != nil {
		log.Printf("renderPatioEstimate execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// patioEstimateHandler handles GET and POST /patio-estimate.
//
// GET renders the current session (or freshly reloaded database) patio
// estimate, and auto-completes a save that was interrupted by a login
// redirect.
//
// POST branches on form values: save=true persists the estimate via
// savePatioEstimate, and otherwise the form is treated as calculator input —
// posted from /patio-cover-calculator — and the cost breakdown is recalculated.
func patioEstimateHandler(w http.ResponseWriter, r *http.Request) {
	sd, err := GetSession(r, w)
	if err != nil {
		log.Printf("Session failed: %v", err)
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Session error"})
		return
	}

	estimate := sd.PatioEstimate
	estimate.Customer = sd.Customer

	// GET
	if r.Method != http.MethodPost {
		if sd.UserAuth.IsAuthenticated && sd.PendingSave {
			sd.PendingSave = false
			_ = sd.Save(r, w)
			estimate.CalcAllCosts()
			if estimate.TotalCost > 0 && estimate.Customer.FirstName != "" {
				savePatioEstimate(w, r, &estimate, sd)
				http.Redirect(w, r, fmt.Sprintf("/patio-estimate/%d", estimate.EstimateID), http.StatusSeeOther)
				return
			}
		}

		if estimate.EstimateID > 0 {
			fresh := getPatioEstimate(estimate.EstimateID)
			fresh.Customer = estimate.Customer
			renderPatioEstimate(w, r, fresh)
			return
		}
		renderPatioEstimate(w, r, estimate)
		return
	}

	// POST save=true
	if r.FormValue("save") == "true" {
		estimate.CalcAllCosts()
		if estimate.TotalCost > 0 && estimate.Customer.FirstName != "" {
			savePatioEstimate(w, r, &estimate, sd)
			http.Redirect(w, r, fmt.Sprintf("/patio-estimate/%d", estimate.EstimateID), http.StatusSeeOther)
		} else {
			renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Please complete Customer and Estimate before Saving."})
		}
		return
	}

	// POST calculator input from /patio-cover-calculator
	width, err := strconv.ParseFloat(r.FormValue("width"), 64)
	if err != nil || width <= 0 {
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Width must be a positive number"})
		return
	}
	depth, err := strconv.ParseFloat(r.FormValue("length"), 64)
	if err != nil || depth <= 0 {
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Depth must be a positive number"})
		return
	}
	patioType := r.FormValue("patioType")

	estimate.Desc = r.FormValue("desc")
	estimate.ProductType = "patio_cover"
	estimate.Width = width
	estimate.Depth = depth
	estimate.PatioType = patioType

	if dm := r.FormValue("diyMode"); dm != "" {
		if v, err := strconv.Atoi(dm); err == nil && v >= 0 && v <= 2 {
			estimate.DIYMode = v
		}
	}

	estimate.CalcAllCosts()
	if estimate.Error != "" {
		renderPatioEstimate(w, r, estimate)
		return
	}

	// Unsave — a recalculated estimate is no longer the saved version
	estimate.SaveDate = time.Time{}
	estimate.EstimateID = 0
	estimate.AccessToken = ""
	estimate.ExpirationDate = time.Time{}

	sd.PatioEstimate = estimate
	sd.ActiveProduct = "patio_cover"
	if err := sd.Save(r, w); err != nil {
		log.Printf("patioEstimateHandler - Save Session failed.")
	}

	renderPatioEstimate(w, r, estimate)
}

// patioEstimateDBHandler handles GET /patio-estimate/{estimateID}, loading a
// saved patio cover estimate from the database. It requires an authenticated
// session and reports "Unauthorized" unless the caller owns the estimate or
// is an admin.
func patioEstimateDBHandler(w http.ResponseWriter, r *http.Request) {
	sd, err := GetSession(r, w)
	if err != nil {
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Session error"})
		return
	}

	idStr := r.PathValue("estimateID")
	idInt, convErr := strconv.Atoi(idStr)
	if convErr != nil {
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "patioEstimateDBHandler - URI not found!"})
		return
	}

	if !sd.UserAuth.IsAuthenticated {
		sd.UserAuth.Message = "Please Login to view estimate " + idStr
		_ = sd.Save(r, w)
		http.Redirect(w, r, "/login?rurl=/patio-estimate/"+idStr, http.StatusSeeOther)
		return
	}

	pe := getPatioEstimate(idInt)
	if pe.Error != "" {
		renderPatioEstimate(w, r, pe)
		return
	}

	if pe.UserId != sd.UserAuth.ID && !isAdminUser(sd.UserAuth.Email) {
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Unauthorized."})
		return
	}

	sd.PatioEstimate = pe
	sd.Customer = pe.Customer
	sd.ActiveProduct = "patio_cover"
	_ = sd.Save(r, w)

	renderPatioEstimate(w, r, pe)
}
