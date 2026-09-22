package main

import (
	"bytes"
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

	resend "github.com/resend/resend-go/v2"
	"github.com/yuin/goldmark"
)

// funcMap holds the helper functions available to every template parsed with it,
// including cost/description formatters used by estimate.gohtml and its partials.
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
	"formatPermitDescription":      formatPermitDescription,
	"currentYear":                  func() int { return time.Now().Year() },
	"neg": func(f float64) float64 { return -f },
	"mul": func(a, b int) int { return a * b },
	"div": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	},
	// jsStr encodes a string as a JavaScript string literal, safe inside <script> tags.
	"jsStr": func(s string) template.JS {
		b, _ := json.Marshal(s)
		return template.JS(b)
	},
	"projectPhotosJSON": projectPhotosJSON,
	"categoryOptions":   func() []CategoryOption { return categoryOrder },
}

// EstimateSection is one labeled length/width section of a multi-section deck
// (e.g. a main deck plus a bump-out or wraparound).
type EstimateSection struct {
	ID         int64
	EstimateID int
	Label      string
	Length     float64
	Width      float64
	SortOrder  int
}

// EstimateCustomItem is a manually added line item (description, notes, cost)
// attached to an estimate alongside its calculated costs.
type EstimateCustomItem struct {
	ID          int64
	EstimateID  int
	Description string
	Notes       string
	Cost        float64
	SortOrder   int
}

// ContractorInfo holds the contractor details rendered on an estimate: company
// name, contact info, and license, sourced from contractor_profile.
type ContractorInfo struct {
	ID           int64
	CompanyName  string
	Phone        string
	Website      string
	LicenseNum   string
	LicenseState string
}

// tmpl is the parsed estimate.gohtml template, built once at startup.
var tmpl *template.Template

// db is unused; every DB-backed function in this package opens its own
// short-lived *sql.DB via sql.Open rather than sharing a package-level handle.
var db *sql.DB

// init registers the session-persisted types for gob encoding and parses tmpl.
func init() {
	gob.Register(EstimateSection{})
	gob.Register(EstimateCustomItem{})
	gob.Register(Customer{})
	gob.Register(UserAuth{})
	gob.Register(time.Time{})
	tmpl = template.Must(template.New("estimate.gohtml").Funcs(funcMap).ParseFiles("templates/estimate.gohtml",
		"templates/header.gohtml", "templates/footer.gohtml"))
}

// renderEstimate computes the estimate's display status (Accepted/Expired/Pending),
// loads the terms and conditions, and executes estimate.gohtml. Template execution
// errors are logged and reported to the client as a 500.
func renderEstimate(w http.ResponseWriter, r *http.Request, estimate DeckEstimate) {
	// Compute status
	if !estimate.AcceptDate.IsZero() {
		estimate.Status = "Accepted"
	} else if estimate.EstimateID > 0 && !estimate.ExpirationDate.IsZero() && estimate.ExpirationDate.Before(time.Now()) {
		estimate.Status = "Expired"
	} else if estimate.EstimateID > 0 {
		estimate.Status = "Pending"
	}

	// Terms is not part of session — read and convert markdown each render
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
	userAuth.IsAdmin = isAdminUser(userAuth.Email)
	userAuth.Title = "Deck Estimate"
	userAuth.CanonicalPath = "/estimate"
	rd := renderData{
		Page:   &estimate,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "estimate.gohtml", rd); err != nil {
		log.Printf("estimateHandler execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// estimateDBHandler handles GET /estimate/{estimateID}, loading a saved estimate
// from the database. It requires an authenticated session, redirecting to /login
// if the caller isn't logged in, and reports "Unauthorized" unless the caller owns
// the estimate or is an admin. On success it syncs the session with the loaded
// estimate so /customer pre-fills correctly.
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

	// Use stored costs if available; recalculate only for old estimates saved before line items were persisted.
	if de.Subtotal == 0 {
		de.CalcAllCosts()
		if de.Error != "" {
			renderEstimate(w, r, de)
			return
		}
	}

	// Sync session with the loaded estimate so /customer pre-fills correctly
	sd.Estimate  = de
	sd.Customer  = de.Customer
	_ = sd.Save(r, w)

	// Render the estimate
	renderEstimate(w, r, de)
}

// estimateHandler handles GET and POST /estimate.
//
// GET renders the current session (or freshly reloaded database) estimate, and
// auto-completes a save that was interrupted by a login redirect.
//
// POST branches on form values: save=true persists the estimate via saveEstimate,
// accept=true marks a previously saved estimate accepted, and otherwise the form
// is treated as calculator input — either the full calculator or the basic
// /deck-calculator finish-level form — and the cost breakdown is recalculated.
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

	// GET
	if r.Method != http.MethodPost {
		// Auto-complete a save that was interrupted by a login redirect.
		if sd.UserAuth.IsAuthenticated && sd.PendingSave {
			sd.PendingSave = false
			_ = sd.Save(r, w)
			estimate.CalcAllCosts()
			if estimate.TotalCost > 0 && estimate.Customer.FirstName != "" {
				saveEstimate(w, r, &estimate, sd)
				http.Redirect(w, r, fmt.Sprintf("/estimate/%d", estimate.EstimateID), http.StatusSeeOther)
				return
			}
			// Customer info still missing — fall through and render so user can complete it
		}

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

	// POST save=true
	if r.FormValue("save") == "true" {
		if desc := r.FormValue("desc"); desc != "" {
			estimate.Desc = desc
			sd.Estimate.Desc = desc
		}
		if h := r.FormValue("height"); h != "" {
			if val, err := strconv.ParseFloat(h, 64); err == nil {
				estimate.Height = val
			}
		}
		if m := r.FormValue("material"); m != "" {
			estimate.Material = m
		}
		if m := r.FormValue("railMaterial"); m != "" {
			estimate.RailMaterial = m
		}
		if m := r.FormValue("railInfill"); m != "" {
			estimate.RailInfill = m
		}
		if sw := r.FormValue("stairWidth"); sw != "" {
			if val, err := strconv.ParseFloat(sw, 64); err == nil {
				estimate.StairWidth = val
			}
		}
		if src := r.FormValue("stairRailCount"); src != "" {
			if val, err := strconv.ParseFloat(src, 64); err == nil {
				estimate.StairRailCount = val
			}
		}
		if railOverride := r.FormValue("railOverride"); railOverride != "" {
			if val, err := strconv.ParseFloat(railOverride, 64); err == nil {
				estimate.RailFeetOverride = val
			}
		}
		estimate.HasDemo        = r.FormValue("hasDemo") == "true"
		estimate.HasFascia      = r.FormValue("hasFascia") == "true"
		estimate.HasStairFascia = r.FormValue("hasStairFascia") == "true"
		estimate.HasStairTK     = r.FormValue("hasStairTK") == "true"
		estimate.DiscountCode   = strings.ToUpper(strings.TrimSpace(r.FormValue("discountCode")))
		if dm := r.FormValue("diyMode"); dm != "" {
			if v, err := strconv.Atoi(dm); err == nil && v >= 0 && v <= 2 {
				estimate.DIYMode = v
			}
		}
		if pl := r.FormValue("permitLevel"); pl != "" {
			if v, err := strconv.Atoi(pl); err == nil {
				estimate.PermitLevel = v
			}
		}
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
				estimate.Length = estimate.Sections[0].Length
				estimate.Width  = estimate.Sections[0].Width
			}
		}
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
			saveEstimate(w, r, &estimate, sd)
			http.Redirect(w, r, fmt.Sprintf("/estimate/%d", estimate.EstimateID), http.StatusSeeOther)
		} else {
			renderEstimate(w, r, DeckEstimate{Error: "Please complete Customer and Estimate before Saving."})
		}
		return
	}

	// POST accept=true (after save)
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

	// POST calculator input — parse and validate the deck dimensions
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
	estimate.ProductType = "deck"
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

	// "finish" is set by the basic /deck-calculator form; map its finish-level
	// selection to a concrete material/rail/stair combination.
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

	if dm := r.FormValue("diyMode"); dm != "" {
		if v, err := strconv.Atoi(dm); err == nil && v >= 0 && v <= 2 {
			estimate.DIYMode = v
		}
	}

	// Build initial section from L/W for new estimates from calculator
	estimate.Sections = []EstimateSection{{
		Label:  "Main Deck",
		Length: estimate.Length,
		Width:  estimate.Width,
	}}

	// Default permit level: Design for most decks, Design+Engineering for tall decks
	if estimate.Height >= 12 {
		estimate.PermitLevel = 2
	} else {
		estimate.PermitLevel = 1
	}

	// Calculate the costs
	estimate.CalcAllCosts()
	if estimate.Error != "" {
		renderEstimate(w, r, estimate)
		return
	}

	// Unsave - if it was previously saved - It is changed :(
	estimate.SaveDate = time.Time{}
	estimate.EstimateID = 0
	estimate.AccessToken = "" // clear so saveEstimate generates a fresh token
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


// emailSendHandler handles POST /estimate/send/{estimateID}, emailing the estimate
// via Resend to the requester's address (defaulting to the saved customer email)
// and optionally CC'ing the logged-in user. It responds with a JSON
// {status, message} payload rather than HTML.
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

	// Read email address and CC preference from request body
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
		log.Printf("emailSendHandler: RESEND_API_KEY not set")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Email service is not configured. Please call us at (360) 787-8062.",
		})
		return
	}

	subject := fmt.Sprintf("Deck Estimate #%d-%d — %s", estimate.EstimateID, estimate.Version, estimate.Desc)
	scheme := "https"
	if r.Host == "" || strings.HasPrefix(r.Host, "localhost") {
		scheme = "http"
	}
	estimateURL := fmt.Sprintf("%s://%s/estimate/view/%s", scheme, r.Host, estimate.AccessToken)
	html := buildEstimateEmailHTML(estimate, estimateURL)

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
		log.Printf("emailSendHandler: resend error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Email could not be sent. Please call us at (360) 787-8062.",
		})
		return
	}

	log.Printf("emailSendHandler: sent estimate %d to %s (id: %s)", estimate.EstimateID, toEmail, sent.Id)
	recordNREvent("EstimateEmailed", map[string]interface{}{
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


// estimateTokenHandler handles GET /estimate/view/{token}, serving the public
// customer view of an estimate. No authentication required — the token acts as
// the credential.
func estimateTokenHandler(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	de := getEstimateByToken(token)
	if de.Error != "" {
		renderEstimate(w, r, de)
		return
	}

	if de.Subtotal == 0 {
		de.CalcAllCosts()
		if de.Error != "" {
			renderEstimate(w, r, de)
			return
		}
	}

	de.IsPublicView = true
	de.AcceptURL = "/estimate/accept/" + token
	renderEstimate(w, r, de)
}

// estimatePrintHandler handles GET /estimate/print/{token}, serving a
// print-optimized view of an estimate via access token.
func estimatePrintHandler(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	de := getEstimateByToken(token)
	if de.Error != "" {
		http.Error(w, "Estimate not found", http.StatusNotFound)
		return
	}

	if de.Subtotal == 0 {
		de.CalcAllCosts()
	}

	if mdBytes, err := os.ReadFile("static/t_and_c.md"); err == nil {
		var buf bytes.Buffer
		if err := goldmark.Convert(mdBytes, &buf); err == nil {
			de.TermsHTML = template.HTML(buf.String())
		}
	}

	tmpl := template.Must(template.New("estimate-print.gohtml").Funcs(funcMap).ParseFiles("templates/estimate-print.gohtml"))
	if err := tmpl.ExecuteTemplate(w, "estimate-print.gohtml", &de); err != nil {
		log.Printf("estimatePrintHandler execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// estimateForkHandler copies a public estimate into the session as a new unsaved estimate.
// Custom items and save/accept metadata are stripped; all pricing inputs are preserved.
func estimateForkHandler(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	src := getEstimateByToken(token)
	if src.Error != "" {
		http.Error(w, "Estimate not found", http.StatusNotFound)
		return
	}

	sd, err := GetSession(r, w)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	forked := DeckEstimate{
		Desc:          src.Desc,
		ProductType:   src.ProductType,
		Material:      src.Material,
		Height:        src.Height,
		RailMaterial:  src.RailMaterial,
		RailInfill:    src.RailInfill,
		RailFeetOverride: src.RailFeetOverride,
		StairWidth:    src.StairWidth,
		StairRailCount: src.StairRailCount,
		HasDemo:       src.HasDemo,
		HasFascia:     src.HasFascia,
		HasStairFascia: src.HasStairFascia,
		HasStairTK:    src.HasStairTK,
		DIYMode:       src.DIYMode,
		PermitLevel:   src.PermitLevel,
		Sections:      src.Sections,
		Customer:      src.Customer,
	}
	// Clear section IDs so they get new ones on save
	for i := range forked.Sections {
		forked.Sections[i].ID = 0
		forked.Sections[i].EstimateID = 0
	}

	forked.CalcAllCosts()

	sd.Estimate = forked
	sd.Customer = forked.Customer
	if err := sd.Save(r, w); err != nil {
		http.Error(w, "Session save error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/estimate", http.StatusSeeOther)
}

// estimateAcceptHandler handles POST /estimate/accept/{token}.
// Sets accept_date and re-renders the estimate as accepted.
func estimateAcceptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	token := r.PathValue("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	de := getEstimateByToken(token)
	if de.Error != "" {
		renderEstimate(w, r, de)
		return
	}

	if !de.AcceptDate.IsZero() {
		de.IsPublicView = true
		de.AcceptURL = "/estimate/accept/" + token
		renderEstimate(w, r, de)
		return
	}

	if !de.ExpirationDate.IsZero() && time.Now().After(de.ExpirationDate) {
		de.Error = "This estimate has expired and can no longer be accepted."
		renderEstimate(w, r, de)
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		renderEstimate(w, r, DeckEstimate{Error: "Database Connect failed."})
		return
	}
	defer db.Close()

	_, err = db.Exec(`UPDATE estimates SET accept_date = NOW() WHERE access_token = $1`, token)
	if err != nil {
		log.Printf("estimateAcceptHandler: failed to set accept_date: %v", err)
		renderEstimate(w, r, DeckEstimate{Error: "Failed to accept estimate."})
		return
	}

	de.AcceptDate = time.Now()
	recordNREvent("EstimateAccepted", map[string]interface{}{
		"estimate_id":   de.EstimateID,
		"total_cost":    de.TotalCost,
		"contractor_id": de.ContractorID,
	})
	de.IsPublicView = true
	de.AcceptURL = "/estimate/accept/" + token
	renderEstimate(w, r, de)
}
