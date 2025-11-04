package main

import (
	"html/template"
	"log"
	"net/http"

	_ "github.com/joho/godotenv/autoload"
)

func calcHandler(w http.ResponseWriter, r *http.Request) {
	// tmpl := template.Must(template.ParseFiles("templates/index.html"))
	// tmpl := template.Must(template.ParseFiles("templates/index.html").Funcs(funcMap))
	tmpl := template.Must(template.New("calculator.html").Funcs(funcMap).ParseFiles("templates/calculator.html",
		"templates/header.html", "templates/footer.html"))

	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Load estimate from session
	estimate := DeckEstimate{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		estimate = est
	}
	if err := tmpl.ExecuteTemplate(w, "calculator.html", estimate); err != nil {
		log.Printf("homeHandler execute error: %v", err)
		panic(err)
	}
}
