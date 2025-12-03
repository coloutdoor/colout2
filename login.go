package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

type UserAuth struct {
	IsAuthenticated bool
	AuthEmail       string
	AuthName        string
	Message         string
}

func getUserAuth(r *http.Request) UserAuth {

	// Get session
	sessionData, err := GetSession(r)
	if err != nil {
		return UserAuth{}
	}

	if sessionData.UserAuth.IsAuthenticated {
		return sessionData.UserAuth
	}

	return UserAuth{}
}

func signupHandler(w http.ResponseWriter, r *http.Request) {

	tmpl := template.Must(template.New("signup.html").ParseFiles("templates/signup.html",
		"templates/header.html", "templates/footer.html"))

	// Get session
	sessionData, err := GetSession(r)
	if err != nil {
		log.Printf("GetSession Failed!!")
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	rd := renderData{
		Page:   &sessionData.UserAuth,
		Header: &sessionData.UserAuth,
	}
	if r.Method == http.MethodGet {
		if err := tmpl.ExecuteTemplate(w, "signup.html", rd); err != nil {
			log.Printf("Login Handler execute error: %v", err)
			panic(err)
		}
		return
	}

	// POST – handle signup
	if r.Method == http.MethodPost {

		// Parmas from form
		name := strings.TrimSpace(r.FormValue("name"))
		email := strings.TrimSpace(r.FormValue("email"))
		pass1 := r.FormValue("password")
		pass2 := r.FormValue("password2")

		// Basic validation
		if name == "" || email == "" || pass1 == "" || pass1 != pass2 || len(pass1) < 8 {
			sessionData.UserAuth.Message = "Please fill all fields correctly and ensure passwords match (8+ chars)"
			sessionData.Save(r, w)
			log.Printf("%s", sessionData.UserAuth.Message)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		// Create user (your existing function)
		err = createUser(name, email, pass1)
		if err != nil {
			sessionData.UserAuth.Message = "Create user DB failure."
			sessionData.Save(r, w)
			log.Printf("%s", sessionData.UserAuth.Message)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		// Log them in automatically
		sessionData.UserAuth.AuthEmail = email
		sessionData.UserAuth.IsAuthenticated = true
		sessionData.UserAuth.Message = "Welcome to Columbia Outdoor!"
		sessionData.UserAuth.AuthName = name

		if err := sessionData.Save(r, w); err != nil {
			log.Printf("LoginHandler: Session save Error: %v", err)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func createUser(name string, email string, pass string) error {
	log.Printf("User %s Signed up with email %s.", name, email)
	log.Printf("Password set to %s", pass)
	return nil
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

	if r.Method == "POST" {
		if err := authN(r); err != nil {
			sessionData.UserAuth.Message = "Login failed.  Try again"
		} else {
			sessionData.UserAuth.AuthEmail = r.FormValue("email")
			sessionData.UserAuth.IsAuthenticated = true
			sessionData.UserAuth.Message = fmt.Sprintf("Welcome %s", sessionData.UserAuth.AuthEmail)
		}
	}

	if err := sessionData.Save(r, w); err != nil {
		log.Printf("LoginHandler: Session save Error: %v", err)
	}

	option := r.URL.Query().Get("option")
	rurl := r.URL.Query().Get("rurl")

	/* options - logout, signup */
	if option == "signout" {
		log.Printf("Sign Out for ")
		sessionData.Delete(r, w)
		rurl = "/"
		http.Redirect(w, r, rurl, http.StatusSeeOther)
		return
	}

	if sessionData.UserAuth.IsAuthenticated {
		if rurl == "" {
			rurl = "/"
		}
		http.Redirect(w, r, rurl, http.StatusSeeOther)
		return
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
