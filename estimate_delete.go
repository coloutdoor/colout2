package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
)

func estimateDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/estimate/delete/")
	if idStr == r.URL.Path || idStr == "" {
		http.Error(w, "Invalid estimate ID", http.StatusBadRequest)
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("estimateDeleteHandler: db open error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Verify the estimate exists, is expired, and belongs to this user (or requester is admin).
	var ownerID int64
	var status string
	err = db.QueryRow(`
		SELECT user_id,
		       CASE
		         WHEN accept_date IS NOT NULL THEN 'Accepted'
		         WHEN expiration_date IS NOT NULL AND expiration_date < NOW() THEN 'Expired'
		         ELSE 'Pending'
		       END
		FROM estimates WHERE estimate_id = $1`, idStr).Scan(&ownerID, &status)
	if err == sql.ErrNoRows {
		http.Error(w, "Estimate not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("estimateDeleteHandler: query error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if ownerID != userAuth.ID && !isAdminUser(userAuth.Email) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if status != "Expired" {
		http.Error(w, "Only expired estimates can be deleted", http.StatusBadRequest)
		return
	}

	_, err = db.Exec(`DELETE FROM estimates WHERE estimate_id = $1`, idStr)
	if err != nil {
		log.Printf("estimateDeleteHandler: delete error: %v", err)
		http.Error(w, "Failed to delete estimate", http.StatusInternalServerError)
		return
	}

	log.Printf("Estimate %s deleted by user %s", idStr, userAuth.Email)

	// Redirect back to wherever the request came from.
	redirect := r.FormValue("redirect")
	if redirect == "" {
		redirect = "/my-estimates"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}
