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
	EstimateID  int64
	UserEmail   string
	FirstName   string
	LastName    string
	Description string
	TotalCost   float64
	SaveDate    time.Time
	AcceptDate  sql.NullTime
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

type AdminPageData struct {
	Stats     AdminStats
	Estimates []AdminEstimateRow
	Users     []AdminUserRow
	System    AdminSystemInfo
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
		SELECT e.estimate_id, COALESCE(u.email,''), COALESCE(e.first_name,''), COALESCE(e.last_name,''),
		       COALESCE(e.description,''), COALESCE(e.total_cost,0), e.save_date, e.accept_date
		FROM estimates e
		LEFT JOIN user_auth u ON u.id = e.user_id
		ORDER BY e.created_at DESC
		LIMIT 20`)
	if err != nil {
		return AdminPageData{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var e AdminEstimateRow
		if err := rows.Scan(&e.EstimateID, &e.UserEmail, &e.FirstName, &e.LastName,
			&e.Description, &e.TotalCost, &e.SaveDate, &e.AcceptDate); err != nil {
			log.Printf("admin: scan estimate row: %v", err)
			continue
		}
		data.Estimates = append(data.Estimates, e)
	}

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
	defer urows.Close()
	for urows.Next() {
		var u AdminUserRow
		if err := urows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName,
			&u.Role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt); err != nil {
			log.Printf("admin: scan user row: %v", err)
			continue
		}
		data.Users = append(data.Users, u)
	}

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
