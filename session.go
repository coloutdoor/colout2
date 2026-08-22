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

// SessionData holds session contents for display.
type SessionData struct {
	Estimate    DeckEstimate
	Customer    Customer
	UserAuth    UserAuth
	PendingSave bool // set when a save was interrupted by a login redirect
}

// Session store - in-memory for now, single secret key
var sessionName = "colout2-session3"
var secretKey []byte
var store *sessions.FilesystemStore
var sessionStoreDir = "./sessions" // or "./sessions" for local dev

// **********************************************************************************
// init
//
//	Initialize the session store.  This only runs once at startup
//
// **********************************************************************************
func init() {
	log.Printf("Initializing Session Store at %s", sessionStoreDir)

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
	_, _ = fmt.Fprintln(f, "Session dir test - writable on startup")
	_ = f.Close()
	log.Printf("Session directory test file created at: %s", testFile)

	store = sessions.NewFilesystemStore(sessionStoreDir, secretKey)

	if store == nil {
		log.Panic("Init!  Session store initialization failed.")
	}

	// FilesystemStore writes data to disk, not the cookie — no size limit needed
	store.MaxLength(0)

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
	}
}

// **********************************************************************************
// sessionHandler
//
//	This generates the session debug page.
//
// **********************************************************************************
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

	// The only post here is to delete :)
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

// GetSession  Get the current session object
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
		_ = session.Save(r, w)      // Sends deletion header
		log.Printf("Reset old/invalid session for new FilesystemStore")

		return &SessionData{}, err
	}

	// Extract session data
	data := SessionData{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		data.Estimate = est
	} else {
		log.Printf("GetSession - No DeckEstimate found")
		data.Estimate = DeckEstimate{}
	}
	if customer, ok := session.Values["customer"].(Customer); ok {
		data.Customer = customer
	} else {
		data.Customer = Customer{}
	}
	if ua, ok := session.Values["userauth"].(UserAuth); ok {
		data.UserAuth = ua
	} else {
		data.UserAuth = UserAuth{}
	}
	if ps, ok := session.Values["pending_save"].(bool); ok {
		data.PendingSave = ps
	}

	return &data, nil
}

// Save the session - w http.ResponseWriter, s *SessionData) error
func (s *SessionData) Save(r *http.Request, w http.ResponseWriter) error {
	// Get session
	session, err := store.Get(r, sessionName)
	if err != nil {
		log.Printf("Session get error: %v", err)
		return err
	}

	// Strip render-only fields that are reloaded each request — keeps session small
	estimateForSession := s.Estimate
	estimateForSession.Terms = ""
	estimateForSession.TermsHTML = ""
	session.Values["estimate"] = estimateForSession
	session.Values["customer"] = s.Customer
	session.Values["userauth"] = s.UserAuth
	session.Values["pending_save"] = s.PendingSave

	userName := s.UserAuth.Email
	if userName == "" {
		userName = "Unknown - Not authenticated."
	}

	log.Printf("Saving User Session for %s", userName)

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
