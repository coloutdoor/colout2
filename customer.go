package main

import (
	"encoding/gob"
	"html/template"
	"log"
	"net/http"

	_ "github.com/gorilla/sessions"
)

// Customer holds contact information submitted by the user.
type Customer struct {
	FirstName   string
	LastName    string
	Address     string
	PhoneNumber string
	Email       string
	City        string
	State       string
	Zip         string
}

func init() {
	gob.Register(Customer{})
}

func customerHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("customer.html").ParseFiles("templates/customer.html",
		"templates/header.html", "templates/footer.html"))

	// Get session
	sessionData, err := GetSession(r)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Load customer from session for GET
	customer := sessionData.Customer

	if r.Method == http.MethodPost {
		customer := Customer{
			FirstName:   r.FormValue("firstName"),
			LastName:    r.FormValue("lastName"),
			Address:     r.FormValue("address"),
			PhoneNumber: r.FormValue("phoneNumber"),
			Email:       r.FormValue("email"),
			City:        r.FormValue("city"),
			State:       r.FormValue("state"),
			Zip:         r.FormValue("zip"),
		}
		log.Printf("Customer POST: %+v", customer)
		sessionData.Customer = customer
		if err := sessionData.Save(r, w); err != nil {
			log.Printf("Session save error: %v", err)
		}

		http.Redirect(w, r, "/estimate", http.StatusSeeOther)
		return
	}

	// Render customer page onlyon GET
	userAuth := getUserAuth(r)
	rd := renderData{
		Page:   &customer,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "customer.html", rd); err != nil {
		log.Printf("customerHandler execute error: %v", err)
		panic(err)
	}
}
