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
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	resend "github.com/resend/resend-go/v2"
	"github.com/yuin/goldmark"
)

func init() {
	gob.Register(PatioCoverEstimate{})
}

// PatioCoverEstimate holds all data for a patio cover cost estimate.
type PatioCoverEstimate struct {
	Desc               string
	ProductType        string // always "patio_cover"
	Width              float64
	Depth              float64
	PatioType          string // pergola | leanto | truss | timberframe
	Area               float64
	BaseCost           float64
	RoofSlope          int     // rise per 12" run (the "X" in "X:12"); fixed at 2 for pergola/lean-to
	RoofSlopeCost      float64 // extra cost for slope steeper than the 4:12 baseline (truss/timberframe only)
	PostCount          int     // number of posts, computed from Width (12 ft max spacing, 2 min)
	HasPostWrap        bool    // default off
	PostWrapCost       float64
	HasFinishCeiling   bool // default off; not available on pergola, always on (no extra cost) for timberframe
	FinishCeilingCost  float64
	HasPaintStain      bool // default off
	PaintStainCost     float64
	HasFinishHardware  bool // default off; off = standard galvanized hardware (included), on = upgraded hardware
	FinishHardwareCost float64
	ElectricalLights   int  // canned lights, 0-10
	ElectricalFans     int  // ceiling fans, 0-3
	ElectricalSwitches int  // 0-3
	ElectricalOutlets  int  // 0-4
	HasElectrical      bool // computed: true if any electrical item count > 0
	ElectricalCost     float64
	PermitLevel        int // 0=none, 1=design, 2=design+eng, 3=design+eng+permits
	PermitCost         float64
	CustomItems        []EstimateCustomItem
	CustomItemsTotal   float64
	DiscountCode       string
	DiscountAmount     float64
	Subtotal           float64
	SalesTax           float64
	TotalCost          float64
	DIYMode            int // 0=Full Service, 1=Plans+Materials, 2=Plans Only
	Customer           Customer
	Contractor         ContractorInfo
	ContractorID       int64
	EstimateID         int
	UserId             int64 // FK to UserAuth
	SaveDate           time.Time
	ExpirationDate     time.Time
	AcceptDate         time.Time
	AccessToken        string
	Version            int
	Terms              string
	TermsHTML          template.HTML
	PatioTypeLabel     string // human-readable PatioType, computed in renderPatioEstimate
	Status             string // Accepted, Expired, or Pending — computed in renderPatioEstimate
	IsPublicView       bool   // true when served via the public access-token view (read-only, no login)
	AcceptURL          string // POST target for the Accept button, set on public/accept views
	Error              string
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
	Width              float64 `json:"width"`
	Depth              float64 `json:"depth"`
	PatioType          string  `json:"patioType"`
	Area               float64 `json:"area"`
	BaseCost           float64 `json:"baseCost"`
	RoofSlope          int     `json:"roofSlope"`
	RoofSlopeCost      float64 `json:"roofSlopeCost"`
	PostCount          int     `json:"postCount"`
	HasPostWrap        bool    `json:"hasPostWrap"`
	PostWrapCost       float64 `json:"postWrapCost"`
	HasFinishCeiling   bool    `json:"hasFinishCeiling"`
	FinishCeilingCost  float64 `json:"finishCeilingCost"`
	HasPaintStain      bool    `json:"hasPaintStain"`
	PaintStainCost     float64 `json:"paintStainCost"`
	HasFinishHardware  bool    `json:"hasFinishHardware"`
	FinishHardwareCost float64 `json:"finishHardwareCost"`
	ElectricalLights   int     `json:"electricalLights"`
	ElectricalFans     int     `json:"electricalFans"`
	ElectricalSwitches int     `json:"electricalSwitches"`
	ElectricalOutlets  int     `json:"electricalOutlets"`
	ElectricalCost     float64 `json:"electricalCost"`
}

// CalcAllCosts computes the base cost, sales tax, and total for a patio cover estimate.
func (estimate *PatioCoverEstimate) CalcAllCosts() {
	estimate.CalculatePatioCost(costs)
	if estimate.Error != "" {
		return
	}
	estimate.CalculateRoofSlopeCost(costs)
	estimate.CalculatePostCount()
	estimate.CalculatePostWrapCost(costs)
	estimate.CalculateFinishCeilingCost(costs)
	estimate.CalculatePaintStainCost(costs)
	estimate.CalculateFinishHardwareCost(costs)
	estimate.CalculateElectricalCost(costs)
	estimate.CalcPermitCost(costs)

	estimate.CustomItemsTotal = 0
	for _, ci := range estimate.CustomItems {
		estimate.CustomItemsTotal += ci.Cost
	}
	estimate.Subtotal = estimate.BaseCost + estimate.RoofSlopeCost + estimate.PostWrapCost + estimate.FinishCeilingCost + estimate.PaintStainCost + estimate.FinishHardwareCost + estimate.ElectricalCost + estimate.PermitCost + estimate.CustomItemsTotal

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

// getPatioEstimate loads a patio cover estimate from the DB by estimate_id.
func getPatioEstimate(estimateID int) PatioCoverEstimate {
	return getPatioEstimateBy("e.estimate_id = $1", estimateID)
}

// getPatioEstimateByToken loads a patio cover estimate from the DB by its
// public access token, for the token-based public view/accept pages.
func getPatioEstimateByToken(token string) PatioCoverEstimate {
	return getPatioEstimateBy("e.access_token = $1", token)
}

// getPatioEstimateBy loads a patio cover estimate matching the given WHERE
// clause (parameterized as $1 with arg).
func getPatioEstimateBy(whereClause string, arg interface{}) PatioCoverEstimate {
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
        COALESCE(e.permit_level, 0), COALESCE(e.permit_cost, 0),
        COALESCE(e.discount_code, ''), COALESCE(e.discount_amount, 0),
        COALESCE(cp.company_name,''), COALESCE(cp.phone,''), COALESCE(cp.website,''),
        COALESCE(cp.license_number,''), COALESCE(cp.license_state,''), COALESCE(cp.id,1),
        COALESCE(e.product_details::text, '{}')
        FROM estimates e
        LEFT JOIN contractor_profile cp ON cp.id = e.contractor_id
        WHERE `+whereClause, arg).Scan(
		&pe.EstimateID, &pe.Desc, &pe.TotalCost, &pe.Subtotal, &pe.SalesTax,
		&pe.Customer.FirstName, &pe.Customer.LastName, &pe.Customer.Address, &pe.Customer.City, &pe.Customer.State,
		&pe.Customer.Zip, &pe.Customer.PhoneNumber, &pe.Customer.Email,
		&pe.SaveDate, &acceptDate, &pe.ExpirationDate, &pe.UserId, &pe.ContractorID, &pe.Version,
		&pe.AccessToken, &pe.DIYMode, &pe.ProductType,
		&pe.PermitLevel, &pe.PermitCost,
		&pe.DiscountCode, &pe.DiscountAmount,
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
		pe.RoofSlope = d.RoofSlope
		pe.RoofSlopeCost = d.RoofSlopeCost
		pe.PostCount = d.PostCount
		pe.HasPostWrap = d.HasPostWrap
		pe.PostWrapCost = d.PostWrapCost
		pe.HasFinishCeiling = d.HasFinishCeiling
		pe.FinishCeilingCost = d.FinishCeilingCost
		pe.HasPaintStain = d.HasPaintStain
		pe.PaintStainCost = d.PaintStainCost
		pe.HasFinishHardware = d.HasFinishHardware
		pe.FinishHardwareCost = d.FinishHardwareCost
		pe.ElectricalLights = d.ElectricalLights
		pe.ElectricalFans = d.ElectricalFans
		pe.ElectricalSwitches = d.ElectricalSwitches
		pe.ElectricalOutlets = d.ElectricalOutlets
		pe.ElectricalCost = d.ElectricalCost
		pe.HasElectrical = pe.ElectricalLights+pe.ElectricalFans+pe.ElectricalSwitches+pe.ElectricalOutlets > 0
	}

	cirows, cierr := db.Query(`
		SELECT id, estimate_id, description, COALESCE(notes,''), cost, sort_order
		FROM estimate_custom_items WHERE estimate_id = $1
		ORDER BY sort_order, id`, pe.EstimateID)
	if cierr == nil {
		for cirows.Next() {
			var ci EstimateCustomItem
			if err := cirows.Scan(&ci.ID, &ci.EstimateID, &ci.Description, &ci.Notes, &ci.Cost, &ci.SortOrder); err == nil {
				pe.CustomItems = append(pe.CustomItems, ci)
			}
		}
		cirows.Close()
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
		Width:              estimate.Width,
		Depth:              estimate.Depth,
		PatioType:          estimate.PatioType,
		Area:               estimate.Area,
		BaseCost:           estimate.BaseCost,
		RoofSlope:          estimate.RoofSlope,
		RoofSlopeCost:      estimate.RoofSlopeCost,
		PostCount:          estimate.PostCount,
		HasPostWrap:        estimate.HasPostWrap,
		PostWrapCost:       estimate.PostWrapCost,
		HasFinishCeiling:   estimate.HasFinishCeiling,
		FinishCeilingCost:  estimate.FinishCeilingCost,
		HasPaintStain:      estimate.HasPaintStain,
		PaintStainCost:     estimate.PaintStainCost,
		HasFinishHardware:  estimate.HasFinishHardware,
		FinishHardwareCost: estimate.FinishHardwareCost,
		ElectricalLights:   estimate.ElectricalLights,
		ElectricalFans:     estimate.ElectricalFans,
		ElectricalSwitches: estimate.ElectricalSwitches,
		ElectricalOutlets:  estimate.ElectricalOutlets,
		ElectricalCost:     estimate.ElectricalCost,
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
    permit_level = $21,
    permit_cost = $22,
    discount_code = $23,
    discount_amount = $24,
    version = version + 1
WHERE estimate_id = $25
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
			estimate.PermitLevel, estimate.PermitCost,
			estimate.DiscountCode, estimate.DiscountAmount,
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
			access_token, diy_mode, product_type, product_details,
			permit_level, permit_cost, discount_code, discount_amount, version)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15,
			$16, $17,
			$18, $19, $20, $21,
			$22, $23, $24, $25, 1
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
			estimate.AccessToken, estimate.DIYMode, estimate.ProductType, string(detailsJSON),
			estimate.PermitLevel, estimate.PermitCost, estimate.DiscountCode, estimate.DiscountAmount).
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

	if !estimate.AcceptDate.IsZero() {
		estimate.Status = "Accepted"
	} else if estimate.EstimateID > 0 && !estimate.ExpirationDate.IsZero() && estimate.ExpirationDate.Before(time.Now()) {
		estimate.Status = "Expired"
	} else if estimate.EstimateID > 0 {
		estimate.Status = "Pending"
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
		if desc := r.FormValue("desc"); desc != "" {
			estimate.Desc = desc
		}
		if w := r.FormValue("width"); w != "" {
			if val, err := strconv.ParseFloat(w, 64); err == nil && val > 0 {
				estimate.Width = val
			}
		}
		if d := r.FormValue("length"); d != "" {
			if val, err := strconv.ParseFloat(d, 64); err == nil && val > 0 {
				estimate.Depth = val
			}
		}
		if pt := r.FormValue("patioType"); pt != "" {
			estimate.PatioType = pt
		}
		if rs := r.FormValue("roofSlope"); rs != "" {
			if v, err := strconv.Atoi(rs); err == nil && v >= 4 {
				estimate.RoofSlope = v
			}
		}
		if hpw := r.FormValue("hasPostWrap"); hpw != "" {
			estimate.HasPostWrap = hpw == "true"
		}
		if hfc := r.FormValue("hasFinishCeiling"); hfc != "" {
			estimate.HasFinishCeiling = hfc == "true"
		}
		if hps := r.FormValue("hasPaintStain"); hps != "" {
			estimate.HasPaintStain = hps == "true"
		}
		if hfh := r.FormValue("hasFinishHardware"); hfh != "" {
			estimate.HasFinishHardware = hfh == "true"
		}
		if el := r.FormValue("electricalLights"); el != "" {
			if v, err := strconv.Atoi(el); err == nil && v >= 0 && v <= 10 {
				estimate.ElectricalLights = v
			}
		}
		if ef := r.FormValue("electricalFans"); ef != "" {
			if v, err := strconv.Atoi(ef); err == nil && v >= 0 && v <= 3 {
				estimate.ElectricalFans = v
			}
		}
		if es := r.FormValue("electricalSwitches"); es != "" {
			if v, err := strconv.Atoi(es); err == nil && v >= 0 && v <= 3 {
				estimate.ElectricalSwitches = v
			}
		}
		if eo := r.FormValue("electricalOutlets"); eo != "" {
			if v, err := strconv.Atoi(eo); err == nil && v >= 0 && v <= 4 {
				estimate.ElectricalOutlets = v
			}
		}
		if pl := r.FormValue("permitLevel"); pl != "" {
			if v, err := strconv.Atoi(pl); err == nil && v >= 0 && v <= 3 {
				estimate.PermitLevel = v
			}
		}
		if dm := r.FormValue("diyMode"); dm != "" {
			if v, err := strconv.Atoi(dm); err == nil && v >= 0 && v <= 2 {
				estimate.DIYMode = v
			}
		}
		estimate.DiscountCode = strings.ToUpper(strings.TrimSpace(r.FormValue("discountCode")))
		if ciJSON := r.FormValue("customItems"); ciJSON != "" {
			var parsed []struct {
				Description string  `json:"description"`
				Notes       string  `json:"notes"`
				Cost        float64 `json:"cost"`
			}
			if err := json.Unmarshal([]byte(ciJSON), &parsed); err == nil {
				estimate.CustomItems = nil
				for i, ci := range parsed {
					if ci.Description != "" || ci.Cost != 0 {
						estimate.CustomItems = append(estimate.CustomItems, EstimateCustomItem{
							Description: ci.Description,
							Notes:       ci.Notes,
							Cost:        ci.Cost,
							SortOrder:   i,
						})
					}
				}
			}
		}
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

	// Default permit level: Material Takeoff, same as a typical (non-tall) deck.
	estimate.PermitLevel = 0

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

// patioEstimateTokenHandler handles GET /patio-estimate/view/{token}, serving
// the public customer view of a patio cover estimate. No authentication
// required — the token acts as the credential.
func patioEstimateTokenHandler(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	pe := getPatioEstimateByToken(token)
	if pe.Error != "" {
		renderPatioEstimate(w, r, pe)
		return
	}

	pe.IsPublicView = true
	pe.AcceptURL = "/patio-estimate/accept/" + token
	renderPatioEstimate(w, r, pe)
}

// patioEstimateAcceptHandler handles POST /patio-estimate/accept/{token}.
// Sets accept_date and re-renders the estimate as accepted.
func patioEstimateAcceptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	pe := getPatioEstimateByToken(token)
	if pe.Error != "" {
		renderPatioEstimate(w, r, pe)
		return
	}

	if !pe.AcceptDate.IsZero() {
		pe.IsPublicView = true
		pe.AcceptURL = "/patio-estimate/accept/" + token
		renderPatioEstimate(w, r, pe)
		return
	}

	if !pe.ExpirationDate.IsZero() && time.Now().After(pe.ExpirationDate) {
		pe.Error = "This estimate has expired and can no longer be accepted."
		renderPatioEstimate(w, r, pe)
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Database Connect failed."})
		return
	}
	defer db.Close()

	_, err = db.Exec(`UPDATE estimates SET accept_date = NOW() WHERE access_token = $1`, token)
	if err != nil {
		log.Printf("patioEstimateAcceptHandler: failed to set accept_date: %v", err)
		renderPatioEstimate(w, r, PatioCoverEstimate{Error: "Failed to accept estimate."})
		return
	}

	pe.AcceptDate = time.Now()
	recordNREvent("PatioEstimateAccepted", map[string]interface{}{
		"estimate_id":   pe.EstimateID,
		"total_cost":    pe.TotalCost,
		"contractor_id": pe.ContractorID,
	})
	pe.IsPublicView = true
	pe.AcceptURL = "/patio-estimate/accept/" + token
	renderPatioEstimate(w, r, pe)
}

// patioEmailSendHandler handles POST /patio-estimate/send/{estimateID}, emailing
// the patio cover estimate via Resend to the requester's address (defaulting to
// the saved customer email) and optionally CC'ing the logged-in user. It
// responds with a JSON {status, message} payload rather than HTML.
func patioEmailSendHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	estimateIDStr := r.PathValue("estimateID")
	estimateID, _ := strconv.Atoi(estimateIDStr)

	sd, err := GetSession(r, w)
	if err != nil || !sd.UserAuth.IsAuthenticated {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Please log in first."})
		return
	}

	// Load fresh from DB so options and costs are current
	estimate := getPatioEstimate(estimateID)
	if estimate.Error != "" || estimate.EstimateID == 0 {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Estimate not found."})
		return
	}
	if label, ok := patioTypeLabels[estimate.PatioType]; ok {
		estimate.PatioTypeLabel = label
	} else {
		estimate.PatioTypeLabel = estimate.PatioType
	}
	estimate.CalcAllCosts()

	var req struct {
		Email  string `json:"email"`
		CCSelf bool   `json:"ccSelf"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	toEmail := req.Email
	if toEmail == "" {
		toEmail = estimate.Customer.Email
	}

	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Printf("patioEmailSendHandler: RESEND_API_KEY not set")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Email service is not configured. Please call us at (360) 787-8062.",
		})
		return
	}

	subject := fmt.Sprintf("Patio Cover Estimate #%d-%d — %s", estimate.EstimateID, estimate.Version, estimate.Desc)
	scheme := "https"
	if r.Host == "" || strings.HasPrefix(r.Host, "localhost") {
		scheme = "http"
	}
	estimateURL := fmt.Sprintf("%s://%s/patio-estimate/view/%s", scheme, r.Host, estimate.AccessToken)
	html := buildPatioEstimateEmailHTML(estimate, estimateURL)

	client := resend.NewClient(apiKey)
	params := &resend.SendEmailRequest{
		From:    "Columbia Outdoor <support@columbiaoutdoor.com>",
		To:      []string{toEmail},
		Bcc:     []string{"support@columbiaoutdoor.com"},
		Subject: subject,
		Html:    html,
	}
	if req.CCSelf && sd.UserAuth.Email != "" && sd.UserAuth.Email != toEmail {
		params.Cc = []string{sd.UserAuth.Email}
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		log.Printf("patioEmailSendHandler: resend error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Email could not be sent. Please call us at (360) 787-8062.",
		})
		return
	}

	log.Printf("patioEmailSendHandler: sent estimate %d to %s (id: %s)", estimate.EstimateID, toEmail, sent.Id)
	recordNREvent("PatioEstimateEmailed", map[string]interface{}{
		"estimate_id":   estimate.EstimateID,
		"total_cost":    estimate.TotalCost,
		"contractor_id": estimate.ContractorID,
		"to_email":      toEmail,
	})
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "sent",
		"message": "Estimate emailed to " + toEmail,
	})
}

type patioEstimateEmailData struct {
	Estimate          PatioCoverEstimate
	EstimateURL       string
	ContractorName    string
	ContractorPhone   string
	ContractorWebsite string
	ContractorLicense string
}

// buildPatioEstimateEmailHTML renders the patio cover estimate email template to a string.
func buildPatioEstimateEmailHTML(e PatioCoverEstimate, estimateURL string) string {
	data := patioEstimateEmailData{
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

	tmpl := template.Must(template.New("email-estimate-patio.gohtml").Funcs(funcMap).ParseFiles("templates/email-estimate-patio.gohtml"))
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "email-estimate-patio.gohtml", data); err != nil {
		log.Printf("buildPatioEstimateEmailHTML: template error: %v", err)
		return "<p>Error rendering estimate email.</p>"
	}
	return buf.String()
}
