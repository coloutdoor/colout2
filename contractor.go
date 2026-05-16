package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

type ContractorProfile struct {
	ID                 int64
	CompanyName        string
	Phone              string
	Website            string
	TermsText          string
	Specialties        []string
	ServiceStates      []string
	ServiceCity        string
	ServiceRadiusMiles int
	LicenseNumber      string
	LicenseState       string
	BondNumber         string
	InsuranceCarrier   string
	InsurancePolicy    string
	ApprovalStatus     string
	ApprovalNotes      string
	Message            string
}

func contractorRegisterHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated {
		http.Redirect(w, r, "/login?rurl=/contractor/register", http.StatusFound)
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		http.Error(w, "Database not configured", http.StatusInternalServerError)
		return
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("contractorRegisterHandler: db open error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Check if a profile already exists for this user.
	var existing ContractorProfile
	err = db.QueryRow(`
		SELECT id, company_name, COALESCE(phone,''), COALESCE(website,''),
		       COALESCE(service_city,''), COALESCE(service_radius_miles, 50),
		       COALESCE(license_number,''), COALESCE(license_state,''),
		       COALESCE(bond_number,''), COALESCE(insurance_carrier,''), COALESCE(insurance_policy,''),
		       approval_status, COALESCE(approval_notes,'')
		FROM contractor_profile WHERE user_id = $1`, userAuth.ID).Scan(
		&existing.ID, &existing.CompanyName, &existing.Phone, &existing.Website,
		&existing.ServiceCity, &existing.ServiceRadiusMiles,
		&existing.LicenseNumber, &existing.LicenseState,
		&existing.BondNumber, &existing.InsuranceCarrier, &existing.InsurancePolicy,
		&existing.ApprovalStatus, &existing.ApprovalNotes,
	)

	userAuth.Title = "Contractor Registration"
	userAuth.Subtitle = "Join the Columbia Outdoor contractor network"

	tmpl := template.Must(template.New("contractor-register.gohtml").Funcs(funcMap).ParseFiles(
		"templates/contractor-register.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	// Profile already exists — show status page, not the form.
	if err == nil {
		rd := renderData{Page: &existing, Header: &userAuth}
		if err := tmpl.ExecuteTemplate(w, "contractor-register.gohtml", rd); err != nil {
			log.Printf("contractorRegisterHandler status render error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// POST — process the registration form.
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		companyName := strings.TrimSpace(r.FormValue("company_name"))
		if companyName == "" {
			profile := buildProfileFromForm(r)
			profile.Message = "Company name is required."
			rd := renderData{Page: &profile, Header: &userAuth}
			tmpl.ExecuteTemplate(w, "contractor-register.gohtml", rd)
			return
		}

		specialtiesSlice := r.Form["specialties"]
		statesSlice := r.Form["service_states"]
		radius := 50
		if r.FormValue("service_radius_miles") != "" {
			fmt.Sscanf(r.FormValue("service_radius_miles"), "%d", &radius)
		}

		_, insertErr := db.Exec(`
			INSERT INTO contractor_profile (
				user_id, company_name, phone, website, terms_text,
				specialties, service_states, service_city, service_radius_miles,
				license_number, license_state, bond_number,
				insurance_carrier, insurance_policy, approval_status
			) VALUES (
				$1, $2, $3, $4, $5,
				$6::text[], $7::text[], $8, $9,
				$10, $11, $12, $13, $14, 'pending'
			)`,
			userAuth.ID,
			companyName,
			strings.TrimSpace(r.FormValue("phone")),
			strings.TrimSpace(r.FormValue("website")),
			strings.TrimSpace(r.FormValue("terms_text")),
			formatPGArray(specialtiesSlice),
			formatPGArray(statesSlice),
			strings.TrimSpace(r.FormValue("service_city")),
			radius,
			strings.TrimSpace(r.FormValue("license_number")),
			strings.TrimSpace(r.FormValue("license_state")),
			strings.TrimSpace(r.FormValue("bond_number")),
			strings.TrimSpace(r.FormValue("insurance_carrier")),
			strings.TrimSpace(r.FormValue("insurance_policy")),
		)
		if insertErr != nil {
			log.Printf("contractorRegisterHandler: insert error: %v", insertErr)
			profile := buildProfileFromForm(r)
			profile.Message = "Registration failed — please try again."
			rd := renderData{Page: &profile, Header: &userAuth}
			tmpl.ExecuteTemplate(w, "contractor-register.gohtml", rd)
			return
		}

		// Update role from homeowner -> contractor
		db.Exec(`UPDATE user_auth SET role = 'contractor' WHERE id = $1`, userAuth.ID)

		existing = ContractorProfile{ApprovalStatus: "pending"}
		rd := renderData{Page: &existing, Header: &userAuth}
		tmpl.ExecuteTemplate(w, "contractor-register.gohtml", rd)
		return
	}

	// GET — show blank form.
	rd := renderData{Page: &ContractorProfile{}, Header: &userAuth}
	if err := tmpl.ExecuteTemplate(w, "contractor-register.gohtml", rd); err != nil {
		log.Printf("contractorRegisterHandler render error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// formatPGArray formats a Go string slice as a PostgreSQL array literal e.g. {WA,OR}
func formatPGArray(items []string) string {
	cleaned := make([]string, 0, len(items))
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s != "" {
			cleaned = append(cleaned, s)
		}
	}
	return fmt.Sprintf("{%s}", strings.Join(cleaned, ","))
}

func buildProfileFromForm(r *http.Request) ContractorProfile {
	radius := 50
	fmt.Sscanf(r.FormValue("service_radius_miles"), "%d", &radius)
	return ContractorProfile{
		CompanyName:        r.FormValue("company_name"),
		Phone:              r.FormValue("phone"),
		Website:            r.FormValue("website"),
		TermsText:          r.FormValue("terms_text"),
		Specialties:        r.Form["specialties"],
		ServiceStates:      r.Form["service_states"],
		ServiceCity:        r.FormValue("service_city"),
		ServiceRadiusMiles: radius,
		LicenseNumber:      r.FormValue("license_number"),
		LicenseState:       r.FormValue("license_state"),
		BondNumber:         r.FormValue("bond_number"),
		InsuranceCarrier:   r.FormValue("insurance_carrier"),
		InsurancePolicy:    r.FormValue("insurance_policy"),
	}
}
