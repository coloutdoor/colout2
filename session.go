package main

import (
	"html/template"
	"log"
	"net/http"
)

// SessionData holds session contents for display.
type SessionData struct {
	Estimate DeckEstimate
	Customer Customer
}

func sessionHandler(w http.ResponseWriter, r *http.Request) {
	//tmpl := template.Must(template.New("session.html").ParseFiles("templates/session.html"))
	tmpl := template.Must(template.New("session.html").Funcs(funcMap).ParseFiles("templates/session.html"))

	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		// Reset session by clearing values
		delete(session.Values, "estimate")
		delete(session.Values, "customer")
		if err := session.Save(r, w); err != nil {
			log.Printf("Session save error: %v", err)
			http.Error(w, "Session save error", http.StatusInternalServerError)
			return
		}
		log.Printf("Session reset")
		http.Redirect(w, r, "/session", http.StatusSeeOther)
		return
	}

	// Extract session data
	data := SessionData{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		data.Estimate = est
	}
	if cust, ok := session.Values["customer"].(Customer); ok {
		data.Customer = cust
	}

	if err := tmpl.ExecuteTemplate(w, "session.html", data); err != nil {
		log.Printf("sessionHandler execute error: %v", err)
		panic(err)
	}
}
