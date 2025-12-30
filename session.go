package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/sessions"
)

// lSessionData holds session contents for display.
type SessionData struct {
	Estimate DeckEstimate
	Customer Customer
	UserAuth UserAuth
}

// Session store - in-memory for now, single secret key
var sessionName = "colout2-session3"
var secretKey []byte
var store *sessions.FilesystemStore
var sessionStoreDir = "./sessions" // or "./sessions" for local dev

func init() {
	log.Printf("Initializing Session Store")

	// Secret key (at least 32 bytes) - load from env var in production
	secretKey = []byte(os.Getenv("SESSION_SECRET")) // e.g., generate with crypto/rand
	if len(secretKey) == 0 {
		log.Fatal("SESSION_SECRET env var is required")
	}

	// store = sessions.NewCookieStore([]byte("super-secret-key-12345"))
	// In your initialization (e.g., main.go)
	if err := os.MkdirAll(sessionStoreDir, 0755); err != nil {
		log.Fatalf("Failed to create session directory: %v", err)
	}

	// Test write to confirm directory is usable
	testFile := filepath.Join(sessionStoreDir, "init-test.txt") // Use filepath.Join for cross-platform safety
	f, err := os.Create(testFile)
	if err != nil {
		log.Fatalf("Session directory %s is not writable: %v", sessionStoreDir, err)
	}
	fmt.Fprintln(f, "Session dir test - writable on startup")
	f.Close()
	log.Printf("Session directory test file created at: %s", testFile)

	store = sessions.NewFilesystemStore(sessionStoreDir, secretKey)

	if store == nil {
		log.Panic("Init!  Session store is nil!")
	}

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
	}
}

// This is used to test / debug the session data
func sessionHandler(w http.ResponseWriter, r *http.Request) {
	if store == nil {
		log.Panic("SessionHandler!  Session store is nil!")
	}
	//tmpl := template.Must(template.New("session.html").ParseFiles("templates/session.html"))
	tmpl := template.Must(template.New("session.html").Funcs(funcMap).ParseFiles("templates/session.html"))

	data, err := GetSession(r, w)
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

func GetSession(r *http.Request, w http.ResponseWriter) (*SessionData, error) {
	if store == nil {
		log.Printf("Session store is nil!")
		return nil, fmt.Errorf("session store is nil")
	}

	// Get session
	session, err := store.Get(r, sessionName)
	if err != nil {
		log.Printf("Session get error: %v", err)
		// Clear any invalid/old cookie and force a fresh session
		session.Options.MaxAge = -1 // Deletes the cookie immediately
		session.Save(r, w)          // Sends deletion header
		log.Printf("Reset old/invalid session for new FilesystemStore")

		return &SessionData{}, err
	}

	// Extract session data
	data := SessionData{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		data.Estimate = est
	} else {
		data.Estimate = DeckEstimate{}
	}
	if cust, ok := session.Values["customer"].(Customer); ok {
		data.Customer = cust
	} else {
		data.Customer = Customer{}
	}
	if ua, ok := session.Values["userauth"].(UserAuth); ok {
		data.UserAuth = ua
	} else {
		data.UserAuth = UserAuth{}
	}

	return &data, nil
}

// func SaveSession(w http.ResponseWriter, s *SessionData) error
func (s *SessionData) Save(r *http.Request, w http.ResponseWriter) error {
	// Get session
	session, err := store.Get(r, sessionName)
	if err != nil {
		log.Printf("Session get error: %v", err)
		return err
	}

	session.Values["estimate"] = s.Estimate
	session.Values["customer"] = s.Customer
	session.Values["userauth"] = s.UserAuth

	log.Printf("Saving User Session for %s", s.UserAuth.Email)

	if err := session.Save(r, w); err != nil {
		log.Printf("Session save error: %v", err)
		return err
	}

	return nil

}

func (s *SessionData) Delete(r *http.Request, w http.ResponseWriter) error {
	// Get session
	session, err := store.Get(r, sessionName)
	if err != nil {
		log.Printf("Session get error: %v", err)
		return err
	}

	// Reset session by clearing values
	delete(session.Values, "estimate")
	delete(session.Values, "customer")
	delete(session.Values, "userauth")

	if err := session.Save(r, w); err != nil {
		log.Printf("Session save error: %v", err)
		return err
	}

	log.Printf("Session reset")
	return nil

}
