package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"
)

// Test - Not used !!!
func homeHandler(w http.ResponseWriter, r *http.Request) {
	// tmpl := template.Must(template.ParseFiles("templates/index.html"))
	// tmpl := template.Must(template.ParseFiles("templates/index.html").Funcs(funcMap))
	tmpl := template.Must(template.New("index.html").Funcs(funcMap).ParseFiles("templates/index.html"))

	// Get session
	session, err := store.Get(r, "colout2-session")
	if err != nil {
		log.Printf("Session get error: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Load estimate from session
	estimate := DeckEstimate{}
	if est, ok := session.Values["estimate"].(DeckEstimate); ok {
		estimate = est
	}
	if err := tmpl.ExecuteTemplate(w, "index.html", estimate); err != nil {
		log.Printf("homeHandler execute error: %v", err)
		panic(err)
	}
}

func cssHandler(w http.ResponseWriter, r *http.Request) {
	// log.Printf("CSS Handler for : %s", r.URL.Path)
	// Set the content type to CSS
	w.Header().Set("Content-Type", "text/css")

	// Strip the leading "/" from the path
	filePath := strings.TrimPrefix(r.URL.Path, "/")
	// Serve the file from the "css" directory, using the full path
	http.ServeFile(w, r, filePath)
}

func robotsTxtHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if r.URL.Path == "/robots.txt" {
		// Read robots.txt from file
		content, err := os.ReadFile("static/robots.txt")
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		strContent := string(content)
		fmt.Fprintf(w, "%s", strContent)
	} else {
		http.NotFound(w, r)
	}
}

// notFoundHandler serves your custom 404 page
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound) // 404 status

	log.Printf("Error - 404 - Page not found - %s", r.URL)
	tmpl := template.Must(template.New("error404.html").
		Funcs(funcMap).
		ParseFiles("templates/error404.html", "templates/header.html", "templates/footer.html"))

	data := PageData{PageTitle: "Sorry - Not Found"}

	userAuth := getUserAuth(r)
	userAuth.Title = "404 - Not Found"
	userAuth.Subtitle = "Sorry, this page is not available."
	rd := renderData{
		Page:   &data,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "error404.html", rd); err != nil {
		http.Error(w, "Server Error", 500)
		log.Printf("404 error page failed: %v", err)
	}
}

// Privacy Handler - / privacy
func privacyHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("error404.html").
		Funcs(funcMap).
		ParseFiles("templates/privacy.html", "templates/header.html", "templates/footer.html"))

	data := PageData{PageTitle: "Privacy Policy"}

	userAuth := getUserAuth(r)
	userAuth.Title = "Privacy"
	userAuth.Subtitle = "Please review our privacy policy"
	rd := renderData{
		Page:   &data,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "privacy.html", rd); err != nil {
		http.Error(w, "Privacy Policy - Server Error", 500)
		log.Printf("Privacy Policy page failed: %v", err)
	}
}

func main() {
	if err := loadCosts(); err != nil {
		fmt.Println("Error loading costs:", err)
		os.Exit(1)
	}
	devMode := flag.Bool("dev", false, "Run in development mode (localhost only)")
	flag.Parse()

	mux := http.NewServeMux()

	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	mux.HandleFunc("/estimate", estimateHandler)
	mux.HandleFunc("/customer", customerHandler)
	mux.HandleFunc("/session", sessionHandler)
	mux.HandleFunc("/calc", calcHandler)
	mux.HandleFunc("/css/", cssHandler)
	mux.HandleFunc("/contact", contactHandler)
	mux.HandleFunc("/contact/", contactHandler)
	mux.HandleFunc("/test", homeHandler)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/signup", signupHandler)
	mux.HandleFunc("/auth/google", googleLoginHandler)
	mux.HandleFunc("/auth/google/callback", googleCallbackHandler)
	mux.HandleFunc("/sitemap.xml", sitemapHandler)
	mux.HandleFunc("/robots.txt", robotsTxtHandler)
	mux.HandleFunc("/error404", notFoundHandler) // Testing purposes
	mux.HandleFunc("/privacy", privacyHandler)
	mux.HandleFunc("/", ownerHandler) // Defualt - also City specific pages.  This should return a 404.

	//fmt.Println("Server starting on :8080...")
	// err := http.ListenAndServe(":8080", nil)
	addr := ":8080"
	if envAddr := os.Getenv("SERVER_ADDR"); envAddr != "" {
		addr = envAddr
		fmt.Printf("Server starting on %s (from env)...\n", addr)
	} else if *devMode {
		addr = "127.0.0.1:8080"
		fmt.Println("Server starting on localhost:8080 (dev mode)...")
	} else {
		fmt.Println("Default Server starting on :8080...")
	}
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
