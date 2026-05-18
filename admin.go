package main

import (
	"bufio"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// isAdminUser reports whether email appears in static/admin_users.txt.
func isAdminUser(email string) bool {
	f, err := os.Open("static/admin_users.txt")
	if err != nil {
		log.Printf("admin: could not open admin_users.txt: %v", err)
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == email {
			return true
		}
	}
	return false
}

type AdminEstimateRow struct {
	EstimateID     int64
	UserEmail      string
	ContractorName string // set if the estimate owner is a contractor
	FirstName      string
	LastName       string
	Description    string
	TotalCost      float64
	SaveDate       time.Time
	Status         string // Accepted, Expired, or Pending
}

type AdminUserRow struct {
	ID          int64
	Email       string
	FirstName   string
	LastName    string
	Role        string
	IsActive    bool
	LastLoginAt sql.NullTime
	CreatedAt   time.Time
}

type AdminTableInfo struct {
	Name     string
	RowCount int
	Columns  []AdminColumnInfo
}

type AdminColumnInfo struct {
	ColumnName string
	DataType   string
	Nullable   string
}

type AdminSystemInfo struct {
	DBHost   string
	DBName   string
	DBSchema []AdminTableInfo
}

type AdminStats struct {
	TotalEstimates       int
	TotalUsers           int
	EstimatesThisMonth   int
	TotalEstimatedRevenue float64
}

type AdminContractorRow struct {
	ID                 int64
	UserID             int64
	Email              string
	CompanyName        string
	Phone              string
	Website            string
	ServiceCity        string
	ServiceRadius      int
	LicenseNumber      string
	LicenseState       string
	LicenseExpiration  sql.NullTime
	BondNumber         string
	BondExpiration     sql.NullTime
	InsuranceCarrier   string
	InsurancePolicy    string
	ApprovalStatus     string
	ApprovalNotes      string
	CreatedAt          time.Time
}

type AdminPageData struct {
	Stats       AdminStats
	Estimates   []AdminEstimateRow
	Users       []AdminUserRow
	Contractors []AdminContractorRow
	System      AdminSystemInfo
}

func loadAdminData(dbURL string) (AdminPageData, error) {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return AdminPageData{}, err
	}
	defer db.Close()

	var data AdminPageData

	// --- Stats ---
	db.QueryRow(`SELECT COUNT(*) FROM estimates`).Scan(&data.Stats.TotalEstimates)
	db.QueryRow(`SELECT COUNT(*) FROM user_auth`).Scan(&data.Stats.TotalUsers)
	db.QueryRow(`SELECT COUNT(*) FROM estimates WHERE save_date >= date_trunc('month', NOW())`).Scan(&data.Stats.EstimatesThisMonth)
	db.QueryRow(`SELECT COALESCE(SUM(total_cost), 0) FROM estimates`).Scan(&data.Stats.TotalEstimatedRevenue)

	// --- Estimates (most recent 20) ---
	rows, err := db.Query(`
		SELECT e.estimate_id, COALESCE(u.email,''), COALESCE(cp.company_name,''),
		       COALESCE(e.first_name,''), COALESCE(e.last_name,''),
		       COALESCE(e.description,''), COALESCE(e.total_cost,0), e.save_date,
		       CASE
		         WHEN e.accept_date IS NOT NULL AND e.accept_date > '2000-01-01' THEN 'Accepted'
		         WHEN e.expiration_date IS NOT NULL AND e.expiration_date < NOW() THEN 'Expired'
		         ELSE 'Pending'
		       END AS status
		FROM estimates e
		LEFT JOIN user_auth u ON u.id = e.user_id
		LEFT JOIN contractor_profile cp ON cp.user_id = e.user_id
		ORDER BY e.created_at DESC
		LIMIT 20`)
	if err != nil {
		return AdminPageData{}, err
	}
	for rows.Next() {
		var e AdminEstimateRow
		if err := rows.Scan(&e.EstimateID, &e.UserEmail, &e.ContractorName,
			&e.FirstName, &e.LastName,
			&e.Description, &e.TotalCost, &e.SaveDate, &e.Status); err != nil {
			log.Printf("admin: scan estimate row: %v", err)
			continue
		}
		data.Estimates = append(data.Estimates, e)
	}
	rows.Close()

	// --- Users (most recent 20) ---
	urows, err := db.Query(`
		SELECT id, email, COALESCE(first_name,''), COALESCE(last_name,''),
		       role, is_active, last_login_at, created_at
		FROM user_auth
		ORDER BY created_at DESC
		LIMIT 20`)
	if err != nil {
		return AdminPageData{}, err
	}
	for urows.Next() {
		var u AdminUserRow
		if err := urows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName,
			&u.Role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt); err != nil {
			log.Printf("admin: scan user row: %v", err)
			continue
		}
		data.Users = append(data.Users, u)
	}
	urows.Close()

	// --- Contractors ---
	crows, err := db.Query(`
		SELECT cp.id, cp.user_id, u.email, cp.company_name,
		       COALESCE(cp.phone,''), COALESCE(cp.website,''),
		       COALESCE(cp.service_city,''), COALESCE(cp.service_radius_miles,50),
		       COALESCE(cp.license_number,''), COALESCE(cp.license_state,''),
		       cp.license_expiration,
		       COALESCE(cp.bond_number,''), cp.bond_expiration,
		       COALESCE(cp.insurance_carrier,''), COALESCE(cp.insurance_policy,''),
		       cp.approval_status, COALESCE(cp.approval_notes,''), cp.created_at
		FROM contractor_profile cp
		JOIN user_auth u ON u.id = cp.user_id
		ORDER BY
		  CASE cp.approval_status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 WHEN 'expired' THEN 2 ELSE 3 END,
		  cp.created_at DESC`)
	if err != nil {
		return AdminPageData{}, err
	}
	for crows.Next() {
		var c AdminContractorRow
		if err := crows.Scan(
			&c.ID, &c.UserID, &c.Email, &c.CompanyName,
			&c.Phone, &c.Website, &c.ServiceCity, &c.ServiceRadius,
			&c.LicenseNumber, &c.LicenseState, &c.LicenseExpiration,
			&c.BondNumber, &c.BondExpiration,
			&c.InsuranceCarrier, &c.InsurancePolicy,
			&c.ApprovalStatus, &c.ApprovalNotes, &c.CreatedAt,
		); err != nil {
			log.Printf("admin: scan contractor row: %v", err)
			continue
		}
		data.Contractors = append(data.Contractors, c)
	}
	crows.Close()

	// --- System: DB schema ---
	// Collect table names first, then close cursor before running per-table queries.
	trows, err := db.Query(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return AdminPageData{}, err
	}
	var tableNames []string
	for trows.Next() {
		var name string
		if err := trows.Scan(&name); err == nil {
			tableNames = append(tableNames, name)
		}
	}
	trows.Close()

	var tables []AdminTableInfo
	for _, name := range tableNames {
		t := AdminTableInfo{Name: name}

		db.QueryRow(`SELECT COUNT(*) FROM ` + name).Scan(&t.RowCount)

		crows, err := db.Query(`
			SELECT column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position`, name)
		if err == nil {
			for crows.Next() {
				var c AdminColumnInfo
				crows.Scan(&c.ColumnName, &c.DataType, &c.Nullable)
				t.Columns = append(t.Columns, c)
			}
			crows.Close()
		}
		tables = append(tables, t)
	}

	// --- System: parse DB host/name from URL ---
	parsed, err := url.Parse(dbURL)
	if err == nil {
		data.System.DBHost = parsed.Host
		data.System.DBName = strings.TrimPrefix(parsed.Path, "/")
	}
	data.System.DBSchema = tables

	return data, nil
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)

	if !userAuth.IsAuthenticated {
		http.Redirect(w, r, "/login?rurl=/admin", http.StatusFound)
		return
	}

	if !isAdminUser(userAuth.Email) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		http.Error(w, "Database not configured", http.StatusInternalServerError)
		return
	}

	pageData, err := loadAdminData(dbURL)
	if err != nil {
		log.Printf("adminHandler: load data error: %v", err)
		http.Error(w, "Failed to load admin data", http.StatusInternalServerError)
		return
	}

	userAuth.Title = "Admin"
	userAuth.Subtitle = "Administration"

	tmpl := template.Must(template.New("admin.gohtml").Funcs(funcMap).ParseFiles(
		"templates/admin.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page:   &pageData,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "admin.gohtml", rd); err != nil {
		log.Printf("adminHandler execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// adminContractorActionHandler handles approve/reject POST from the admin page.
func adminContractorActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated || !isAdminUser(userAuth.Email) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	contractorID := r.FormValue("contractor_id")
	action := r.FormValue("action") // "approve", "reject", or "expire"
	notes := r.FormValue("notes")
	licenseExp := r.FormValue("license_expiration")
	bondExp := r.FormValue("bond_expiration")

	validActions := map[string]bool{"approve": true, "reject": true, "expire": true}
	if contractorID == "" || !validActions[action] {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	statusMap := map[string]string{"approve": "approved", "reject": "rejected", "expire": "expired"}
	status := statusMap[action]

	// Use NULL for empty date strings
	var licenseExpVal, bondExpVal interface{}
	if licenseExp != "" {
		licenseExpVal = licenseExp
	}
	if bondExp != "" {
		bondExpVal = bondExp
	}

	_, err = db.Exec(`
		UPDATE contractor_profile
		SET approval_status    = $1,
		    approval_notes     = $2,
		    license_expiration = $3,
		    bond_expiration    = $4,
		    approved_at        = NOW(),
		    approved_by        = $5
		WHERE id = $6`,
		status, notes, licenseExpVal, bondExpVal, userAuth.ID, contractorID)
	if err != nil {
		log.Printf("adminContractorActionHandler: update error: %v", err)
		http.Error(w, "Failed to update contractor", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin?tab=contractors", http.StatusSeeOther)
}
