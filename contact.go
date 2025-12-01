package main

import (
	"html/template"
	"log"
	"net/http"
)

type PageData struct {
	PageTitle string
}

func contactHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Simple form handling (expand with email, DB, etc.)
		r.ParseForm()
		name := r.FormValue("name")
		email := r.FormValue("email")
		// TODO: Send email, save to DB, etc.
		log.Printf("Contact form: %s <%s>", name, email)
		http.Redirect(w, r, "/contact?sent=1", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.New("contact.html").
		Funcs(funcMap).
		ParseFiles("templates/contact.html", "templates/header.html", "templates/footer.html"))

	data := PageData{PageTitle: "Contact Us"}
	if r.URL.Query().Get("sent") == "1" {
		data.PageTitle = "Thank You – Message Sent!"
	}

	userAuth := getUserAuth(r)
	rd := renderData{
		Page:   &data,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "contact.html", rd); err != nil {
		http.Error(w, "Server Error", 500)
		log.Printf("contact error: %v", err)
	}
}
