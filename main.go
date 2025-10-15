package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

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

func main() {
	if err := loadCosts(); err != nil {
		fmt.Println("Error loading costs:", err)
		os.Exit(1)
	}
	devMode := flag.Bool("dev", false, "Run in development mode (localhost only)")
	flag.Parse()

	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/estimate", estimateHandler)
	http.HandleFunc("/customer", customerHandler)
	http.HandleFunc("/session", sessionHandler)
	http.HandleFunc("/calc", calcHandler)

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
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
