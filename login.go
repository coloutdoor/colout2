package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("login.html").ParseFiles("templates/login.html",
		"templates/header.html", "templates/footer.html"))

	// Get session
	sessionData, err := GetSession(r)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	if r.Method == "POST" {
		if err := authN(r); err != nil {
			sessionData.Message = "Login failed.  Try again"
		} else {
			sessionData.AuthEmail = r.FormValue("email")
			sessionData.IsAuthenticated = true
			sessionData.Message = fmt.Sprintf("Welcome %s", sessionData.AuthEmail)
		}
	}

	if err := sessionData.Save(r, w); err != nil {
		log.Printf("Session save Error: %v", err)
	}

	if sessionData.IsAuthenticated {
		rurl := r.URL.Query().Get("rurl")
		if rurl == "" {
			rurl = "/"
		}

		http.Redirect(w, r, rurl, http.StatusSeeOther)
	}

	if err := tmpl.ExecuteTemplate(w, "login.html", sessionData); err != nil {
		log.Printf("Login Handler execute error: %v", err)
		panic(err)
	}
}

func authN(r *http.Request) error {
	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" {
		return fmt.Errorf("missing user name")
	}

	// Success - For now
	if email == password {
		return nil
	}

	return fmt.Errorf("incorrect password")
}
