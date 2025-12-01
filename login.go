package main

import (
	"encoding/gob"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

type UserAuth struct {
	IsAuthenticated bool
	AuthEmail       string
	Message         string
}

func init() {
	gob.Register(&UserAuth{}) // ← register the struct (pointer for safety)
	// Add any other session types here, e.g., gob.Register(&SessionData{})
}

func getUserAuth(r *http.Request) UserAuth {
	// Get session
	sessionData, err := GetSession(r)
	if err != nil && sessionData.UserAuth.IsAuthenticated {
		return sessionData.UserAuth
	}

	return UserAuth{}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("login.html").ParseFiles("templates/login.html",
		"templates/header.html", "templates/footer.html"))

	// Get session
	sessionData, err := GetSession(r)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	userAuth := sessionData.UserAuth

	if r.Method == "POST" {
		if err := authN(r); err != nil {
			userAuth.Message = "Login failed.  Try again"
		} else {
			userAuth.AuthEmail = r.FormValue("email")
			userAuth.IsAuthenticated = true
			userAuth.Message = fmt.Sprintf("Welcome %s", userAuth.AuthEmail)
		}
	}

	if err := sessionData.Save(r, w); err != nil {
		log.Printf("Session save Error: %v", err)
	}

	if userAuth.IsAuthenticated {
		rurl := r.URL.Query().Get("rurl")
		if rurl == "" {
			rurl = "/"
		}

		http.Redirect(w, r, rurl, http.StatusSeeOther)
	}

	if err := tmpl.ExecuteTemplate(w, "login.html", sessionData.UserAuth); err != nil {
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
