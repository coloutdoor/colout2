package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

type MyEstimateRow struct {
	EstimateID int64
	FirstName  string
	LastName   string
	City       string
	TotalCost  float64
	SaveDate   time.Time
	AcceptDate sql.NullTime
}

func myEstimatesHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)

	if !userAuth.IsAuthenticated {
		http.Redirect(w, r, "/login?rurl=/my-estimates", http.StatusFound)
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		http.Error(w, "Database not configured", http.StatusInternalServerError)
		return
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("myEstimatesHandler: db open error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT estimate_id, COALESCE(first_name,''), COALESCE(last_name,''),
		       COALESCE(city,''), COALESCE(total_cost,0), save_date, accept_date
		FROM estimates
		WHERE user_id = $1
		ORDER BY created_at DESC`, userAuth.ID)
	if err != nil {
		log.Printf("myEstimatesHandler: query error: %v", err)
		http.Error(w, "Failed to load estimates", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var estimates []MyEstimateRow
	for rows.Next() {
		var e MyEstimateRow
		if err := rows.Scan(&e.EstimateID, &e.FirstName, &e.LastName,
			&e.City, &e.TotalCost, &e.SaveDate, &e.AcceptDate); err != nil {
			log.Printf("myEstimatesHandler: scan error: %v", err)
			continue
		}
		estimates = append(estimates, e)
	}

	userAuth.Title = "My Estimates"
	userAuth.Subtitle = "Your saved estimates"

	tmpl := template.Must(template.New("my-estimates.gohtml").Funcs(funcMap).ParseFiles(
		"templates/my-estimates.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page:   estimates,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "my-estimates.gohtml", rd); err != nil {
		log.Printf("myEstimatesHandler execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
