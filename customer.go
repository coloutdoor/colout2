package main

import (
	"html/template"
	"log"
	"net/http"
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

func customerHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("customer.html").ParseFiles("templates/customer.html"))

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
	}

	if err := tmpl.ExecuteTemplate(w, "customer.html", nil); err != nil {
		log.Printf("customerHandler execute error: %v", err)
		panic(err)
	}
}
