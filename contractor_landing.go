package main

import (
	"html/template"
	"log"
	"net/http"
)

func contractorLandingHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Contractors"
	userAuth.Subtitle = "Build More. Earn More. Columbia Outdoor."
	userAuth.MetaDesc = "Pacific Northwest outdoor living contractors — stop working for free. Columbia Outdoor handles leads, estimates, permits, scheduling, and contracts so you can focus on building."

	tmpl := template.Must(template.New("contractor.gohtml").Funcs(funcMap).ParseFiles(
		"templates/contractor.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page:   nil,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "contractor.gohtml", rd); err != nil {
		log.Printf("contractorLandingHandler error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
