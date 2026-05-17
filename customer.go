package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"

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

type CustomerPageData struct {
	Customer
	IsHomeowner bool // controls label and auto-fill behavior
}

func customerHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("customer.gohtml").ParseFiles("templates/customer.gohtml",
		"templates/header.gohtml", "templates/footer.gohtml"))

	sessionData, err := GetSession(r, w)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	userAuth := getUserAuth(r, w)
	isHomeowner := userAuth.IsAuthenticated && userAuth.Role == "homeowner"

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

	// GET — start with session customer, then auto-fill from user_auth for homeowners
	customer := sessionData.Customer
	if isHomeowner && customer.FirstName == "" {
		customer = autoFillFromUserAuth(userAuth.ID, customer)
	}

	userAuth.Title = "Customer Information"
	if isHomeowner {
		userAuth.Subtitle = "Your contact information"
	} else {
		userAuth.Subtitle = "Customer contact information"
	}

	rd := renderData{
		Page:   &CustomerPageData{Customer: customer, IsHomeowner: isHomeowner},
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "customer.gohtml", rd); err != nil {
		log.Printf("customerHandler execute error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// autoFillFromUserAuth pre-populates customer fields from the logged-in user's account.
func autoFillFromUserAuth(userID int64, c Customer) Customer {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return c
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return c
	}
	defer db.Close()

	var firstName, lastName, email, phone string
	db.QueryRow(`SELECT COALESCE(first_name,''), COALESCE(last_name,''), email, COALESCE(phone,'')
		FROM user_auth WHERE id = $1`, userID).Scan(&firstName, &lastName, &email, &phone)

	if firstName != "" {
		c.FirstName = firstName
	}
	if lastName != "" {
		c.LastName = lastName
	}
	if email != "" {
		c.Email = email
	}
	if phone != "" {
		c.PhoneNumber = phone
	}
	return c
}
