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
	tmpl := template.Must(template.New("customer.html").ParseFiles("templates/customer.html"))

	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

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
		// Save customer to session
		session.Values["customer"] = customer
		if err := session.Save(r, w); err != nil {
			log.Printf("Session save error: %v", err)
		}
	}

	if err := tmpl.ExecuteTemplate(w, "customer.html", nil); err != nil {
		log.Printf("customerHandler execute error: %v", err)
		panic(err)
	}
}
