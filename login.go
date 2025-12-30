package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt" // For password hashing
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

/*
********************************************************************************************
 * Google AuthPlatfrom / Clients
 * https://console.cloud.google.com/auth/clients?project=columbia-outdoor
********************************************************************************************
*/

var googleOauthConfig = &oauth2.Config{
	//RedirectURL:  "http://localhost:8080/auth/google/callback",                            // change for prod
	RedirectURL:  "https://columbiaoutdoor.com/auth/google/callback",                        // change for prod
	ClientID:     "40124933812-ca7bgksogc8k419fqnbcr5mpq1phedi5.apps.googleusercontent.com", // change for prod
	ClientSecret: "",                                                                        // Get this from the env
	Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
	Endpoint:     google.Endpoint,
}

type UserAuth struct {
	ID              int64
	IsAuthenticated bool
	Email           string
	Name            string
	AuthType        string // Google or password
	Role            string // homeowner, admin, or contractor
	LastName        string
	IsActive        bool
	EmailVerified   bool
	Message         string
	Title           string // Header this is the Title page shown in <title> ... </title>
	MetaDesc        string // this is the Meta Description in Header
	Subtitle        string // This is the subtitle in "H1" tags
	Rurl            string // After a successful login - Go here!
}

func getUserAuth(r *http.Request, w http.ResponseWriter) UserAuth {
	// Get session
	sessionData, err := GetSession(r, w)
	if err != nil {
		return UserAuth{}
	}

	if sessionData.UserAuth.IsAuthenticated {
		return sessionData.UserAuth
	}

	return UserAuth{}
}

// /auth/google — starts the login
func googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := randToken() // simple anti-CSRF
	session, _ := store.Get(r, "session")
	session.Values["oauth_state"] = state
	session.Save(r, w)

	googleOauthConfig.ClientSecret = os.Getenv("GOOGLE_OAUTH_SECRET")
	if callBackURL := os.Getenv("GOOGLE_OAUTH_CALLBACK_URL"); callBackURL != "" {
		googleOauthConfig.RedirectURL = callBackURL
	}
	log.Printf("Google OAuth Callback URL: %s", googleOauthConfig.RedirectURL)

	if googleOauthConfig.ClientSecret == "" {
		http.Error(w, "Env Failed:  Missing Oauth Secret.", http.StatusInternalServerError)
	}
	url := googleOauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// /auth/google/callback — Google redirects here
func googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	savedState := session.Values["oauth_state"]

	if r.URL.Query().Get("state") != savedState.(string) {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	googleOauthConfig.ClientSecret = os.Getenv("GOOGLE_OAUTH_SECRET")
	if googleOauthConfig.ClientSecret == "" {
		http.Error(w, "Env Failed:  Missing Oauth Secret.", http.StatusInternalServerError)
	}
	token, err := googleOauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	// Get user info
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		ID    string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&userInfo)

	sessionData, err := GetSession(r, w)
	if err != nil {
		log.Printf("GetSession Failed!!")
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	// Get the original Rurl
	rurl := sessionData.UserAuth.Rurl

	// In your init or main
	dbURL := os.Getenv("DATABASE_URL") // We'll set this to the Neon string
	if dbURL == "" {
		log.Printf("DATABASE_URL environment variable is required")
		return
	}

	db, err = sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("Unable to connect to database: %v", err)
		return
	}

	// Create user in DB if not exists
	// DEJ
	var user UserAuth
	var hash string
	err = db.QueryRow(`
        SELECT id, email, password_hash, role, first_name, last_name, is_active, email_verified
        FROM user_auth 
        WHERE email = $1`, userInfo.Email).Scan(
		&user.ID, &user.Email, &hash, &user.Role,
		&user.Name, &user.LastName, &user.IsActive, &user.EmailVerified,
	)

	if err == sql.ErrNoRows {
		var fakePasswordHash string = "GoogleAuth" // No hashable password for Google Auth users
		uid, err := createUserDB(userInfo.Name, userInfo.Email, fakePasswordHash)
		if err != nil {
			log.Printf("Failed to create user in DB: %v", err)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}
		user.ID = uid
		log.Printf("New Google user created with UID: %d", uid)
	}

	sessionData.UserAuth = UserAuth{
		ID:              user.ID,
		AuthType:        "google",
		Role:            "homeowner",
		IsAuthenticated: true,
		Email:           userInfo.Email,
		Name:            userInfo.Name,
		Message:         "Google Login, " + userInfo.Name,
	}
	sessionData.Save(r, w)

	delete(session.Values, "oauth_state")
	session.Save(r, w)

	// setFlash(w, r, "Welcome back, "+userInfo.Name+"!")
	if rurl == "" {
		rurl = "/"
	}
	log.Printf("After Google Authentication- Going to: %s ", rurl)
	http.Redirect(w, r, rurl, http.StatusSeeOther)
}

func randToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func signupHandler(w http.ResponseWriter, r *http.Request) {

	tmpl := template.Must(template.New("signup.html").ParseFiles("templates/signup.html",
		"templates/header.html", "templates/footer.html"))

	// Get session
	sessionData, err := GetSession(r, w)
	if err != nil {
		log.Printf("GetSession Failed!!")
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	sessionData.UserAuth.Title = "Signup"
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
		uid, err := createUserPassword(name, email, pass1)
		if err != nil {
			sessionData.UserAuth.Message = "Create user DB failure."
			sessionData.Save(r, w)
			log.Printf("%s", sessionData.UserAuth.Message)
			http.Redirect(w, r, "/signup", http.StatusSeeOther)
			return
		}

		log.Printf("User added to DB with UID: %d", uid)

		// Log them in automatically
		sessionData.UserAuth.ID = uid
		sessionData.UserAuth.AuthType = "password"
		sessionData.UserAuth.Role = "homeowner"
		sessionData.UserAuth.Email = email
		sessionData.UserAuth.IsAuthenticated = true
		sessionData.UserAuth.Message = "Welcome to Columbia Outdoor!"
		sessionData.UserAuth.Name = name

		if err := sessionData.Save(r, w); err != nil {
			log.Printf("LoginHandler: Session save Error: %v", err)
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func createUserPassword(name string, email string, pass string) (int64, error) {
	log.Printf("Password User %s Signed up with email %s.", name, email)

	// Hash the plain password before storing (do this in your handler before calling)
	passwordHash, err := hashPassword(pass)
	if err != nil {
		log.Printf("Password Hash failed: %v", err)
		return 0, err
	}

	return createUserDB(name, email, passwordHash)
}

func createUserDB(name string, email string, passwordHash string) (int64, error) {

	var err error

	// In your init or main
	dbURL := os.Getenv("DATABASE_URL") // We'll set this to the Neon string
	if dbURL == "" {
		log.Printf("DATABASE_URL environment variable is required")
		return 0, nil
	}

	db, err = sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("Unable to connect to database: %v", err)
		return 0, err
	}
	var userID int64
	role := "homeowner" // Set to 'homeowner for now
	lastName := ""
	phone := ""
	isActive := true
	isVerified := false

	/* Send the query to the DB - INSERT */
	const stmt = `
    INSERT INTO user_auth (
        email, password_hash, role,
        first_name, last_name, phone,
        is_active, email_verified
    ) VALUES (
        $1, $2, $3, $4, $5, $6, $7, $8
    ) RETURNING id`

	err = db.QueryRow(stmt,
		email,
		passwordHash,
		role,
		name,
		lastName,
		phone,
		isActive,
		isVerified,
	).Scan(&userID)

	if err != nil {
		log.Printf("Failed to create user: %v", err)
		return 0, err
	}

	log.Printf("Successfully created user ID: %d (email: %s)", userID, email)
	return userID, nil
}

// Helper: Hash password securely with bcrypt
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("login.html").ParseFiles("templates/login.html",
		"templates/header.html", "templates/footer.html"))

	// Get session
	sessionData, err := GetSession(r, w)
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	if r.Method == "POST" {
		if err := authN(r, w); err != nil {
			sessionData.UserAuth.Message = err.Error()
		} else {
			// Update sessionData after successful authN
			sessionData, _ = GetSession(r, w)
		}
	}

	if err := sessionData.Save(r, w); err != nil {
		log.Printf("LoginHandler: Session save Error: %v", err)
	}

	option := r.URL.Query().Get("option")
	rurl := r.URL.Query().Get("rurl")

	/* options - logout, signup */
	if option == "signout" {
		sessionData.Delete(r, w)
		rurl = "/"
		http.Redirect(w, r, rurl, http.StatusSeeOther)
		return
	}

	/* After Authentication */
	if sessionData.UserAuth.IsAuthenticated {
		rurl = sessionData.UserAuth.Rurl
		if rurl == "" {
			rurl = "/"
		}
		log.Printf("After Authentication - Going to: %s ", rurl)
		http.Redirect(w, r, rurl, http.StatusSeeOther)
		return
	}

	/* Set the rurl after a successful login */
	sessionData.UserAuth.Title = "Login"
	if rurl == "" {
		rurl = "/"
	}
	sessionData.UserAuth.Rurl = rurl
	sessionData.Save(r, w)
	if err := tmpl.ExecuteTemplate(w, "login.html", sessionData.UserAuth); err != nil {
		log.Printf("Login Handler execute error: %v", err)
		panic(err)
	}
}

func authN(r *http.Request, w http.ResponseWriter) error {
	email := r.FormValue("email")
	password := r.FormValue("password")
	var err error

	if email == "" || password == "" {
		return fmt.Errorf("email and password required")
	}
	// In your init or main
	dbURL := os.Getenv("DATABASE_URL") // We'll set this to the Neon string

	if dbURL == "" {
		log.Printf("DATABASE_URL environment variable is required")
		return fmt.Errorf("db Error")
	}
	db, err = sql.Open("pgx", dbURL)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return fmt.Errorf("db Error")
	}

	// 1. Fetch user from DB
	var user UserAuth
	var hash string
	err = db.QueryRow(`
        SELECT id, email, password_hash, role, first_name, last_name, is_active, email_verified
        FROM user_auth 
        WHERE email = $1`, email).Scan(
		&user.ID, &user.Email, &hash, &user.Role,
		&user.Name, &user.LastName, &user.IsActive, &user.EmailVerified,
	)

	if err == sql.ErrNoRows {
		// Never reveal if email exists — security best practice
		return fmt.Errorf("invalid email or password")
	}
	if err != nil {
		log.Printf("DB query error: %v", err)
		return fmt.Errorf("server error")
	}

	// 2. Check account status
	if !user.IsActive {
		return fmt.Errorf("account is disabled")
	}
	// Optional: require email verification
	// if !user.EmailVerified {
	//     http.Error(w, "Please verify your email first", http.StatusUnauthorized)
	//     return
	// }

	// 3. Compare password hash
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("incorrect email or password")
	}

	// 4. Create server-side session on success
	sessionData, err := GetSession(r, w)
	if err != nil {
		return fmt.Errorf("session error")
	}

	sessionData.UserAuth = user
	sessionData.UserAuth.IsAuthenticated = true

	if err := sessionData.Save(r, w); err != nil {
		return fmt.Errorf("session Save error")
	}

	// 5. Success — redirect or respond
	log.Printf("User %s (ID: %d) logged in successfully", user.Email, user.ID)
	return nil
}
