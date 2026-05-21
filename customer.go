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
		sessionData.Estimate.Customer = customer
		if err := sessionData.Save(r, w); err != nil {
			log.Printf("Session save error: %v", err)
		}
		// Save contact info back to user_auth for homeowners
		if isHomeowner {
			updateUserAuthContact(userAuth.ID, customer)
		}
		// Persist customer fields to the estimate in the DB if one is saved
		if sessionData.Estimate.EstimateID > 0 {
			updateEstimateCustomer(sessionData.Estimate.EstimateID, customer)
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

	var firstName, lastName, email, phone, address, city, state, zip string
	db.QueryRow(`SELECT COALESCE(first_name,''), COALESCE(last_name,''), email,
		COALESCE(phone,''), COALESCE(address,''), COALESCE(city,''),
		COALESCE(state,''), COALESCE(zip,'')
		FROM user_auth WHERE id = $1`, userID).
		Scan(&firstName, &lastName, &email, &phone, &address, &city, &state, &zip)

	if firstName != "" { c.FirstName = firstName }
	if lastName  != "" { c.LastName  = lastName  }
	if email     != "" { c.Email     = email      }
	if phone     != "" { c.PhoneNumber = phone    }
	if address   != "" { c.Address   = address    }
	if city      != "" { c.City      = city       }
	if state     != "" { c.State     = state      }
	if zip       != "" { c.Zip       = zip        }
	return c
}

// updateEstimateCustomer persists customer contact fields to a saved estimate in the DB.
func updateEstimateCustomer(estimateID int, c Customer) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return
	}
	defer db.Close()

	_, err = db.Exec(`UPDATE estimates
		SET first_name = $1, last_name = $2, phone_number = $3,
		    address = $4, city = $5, state = $6, zip = $7, email = $8
		WHERE estimate_id = $9`,
		c.FirstName, c.LastName, c.PhoneNumber,
		c.Address, c.City, c.State, c.Zip, c.Email, estimateID)
	if err != nil {
		log.Printf("updateEstimateCustomer: failed to update estimate %d: %v", estimateID, err)
	}
}

// updateUserAuthContact saves customer contact info back to user_auth for homeowners.
func updateUserAuthContact(userID int64, c Customer) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return
	}
	defer db.Close()

	db.Exec(`UPDATE user_auth
		SET first_name = $1, last_name = $2, phone = $3,
		    address = $4, city = $5, state = $6, zip = $7
		WHERE id = $8`,
		c.FirstName, c.LastName, c.PhoneNumber,
		c.Address, c.City, c.State, c.Zip, userID)
}
