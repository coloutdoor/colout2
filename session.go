package main

import (
	"html/template"
	"log"
	"net/http"
)

// lSessionData holds session contents for display.
type SessionData struct {
	Estimate        DeckEstimate
	Customer        Customer
	IsAuthenticated bool
	AuthEmail       string
	Message         string
}

// This is used to test / debug the session data
func sessionHandler(w http.ResponseWriter, r *http.Request) {
	//tmpl := template.Must(template.New("session.html").ParseFiles("templates/session.html"))
	tmpl := template.Must(template.New("session.html").Funcs(funcMap).ParseFiles("templates/session.html"))

	data, err := GetSession(r)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// The only post here is a delete :)
	if r.Method == http.MethodPost {
		err := data.Delete(r, w)
		if err != nil {
			http.Error(w, "Session Delete error", http.StatusInternalServerError)
		}
	}

	if err := tmpl.ExecuteTemplate(w, "session.html", data); err != nil {
		log.Printf("sessionHandler execute error: %v", err)
		panic(err)
	}
}

func GetSession(r *http.Request) (*SessionData, error) {
	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		return nil, err
	}
	// Extract session data
	data := SessionData{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		data.Estimate = est
	}
	if cust, ok := session.Values["customer"].(Customer); ok {
		data.Customer = cust
	}
	if em, ok := session.Values["authemail"].(string); ok {
		data.AuthEmail = em
	}
	if isauth, ok := session.Values["isauthenticated"].(bool); ok {
		data.IsAuthenticated = isauth
	}
	if m, ok := session.Values["message"].(string); ok {
		data.Message = m
	}

	return &data, nil
}

// func SaveSession(w http.ResponseWriter, s *SessionData) error
func (s *SessionData) Save(r *http.Request, w http.ResponseWriter) error {
	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		return err
	}

	session.Values["estimate"] = s.Estimate
	session.Values["customer"] = s.Customer
	session.Values["authemail"] = s.AuthEmail
	session.Values["isauthenticated"] = s.IsAuthenticated
	session.Values["message"] = s.Message

	if err := session.Save(r, w); err != nil {
		log.Printf("Session save error: %v", err)
		return err
	}

	return nil

}

func (s *SessionData) Delete(r *http.Request, w http.ResponseWriter) error {
	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		return err
	}

	// Reset session by clearing values
	delete(session.Values, "estimate")
	delete(session.Values, "customer")
	delete(session.Values, "message")
	delete(session.Values, "authemail")
	session.Values["isauthenticated"] = false

	if err := session.Save(r, w); err != nil {
		log.Printf("Session save error: %v", err)
		return err
	}

	log.Printf("Session reset")
	return nil

}
