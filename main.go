package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// nrApp is the New Relic application instance, nil when NR is not configured.
var nrApp *newrelic.Application

// recordNREvent fires a custom New Relic event; no-op when NR is not configured.
func recordNREvent(eventType string, params map[string]interface{}) {
	if nrApp == nil {
		return
	}
	nrApp.RecordCustomEvent(eventType, params)
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
		_, _ = fmt.Fprintf(w, "%s", strContent)
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
		ParseFiles("templates/error404.html", "templates/header.gohtml", "templates/footer.gohtml"))

	data := PageData{PageTitle: "Sorry - Not Found"}

	userAuth := getUserAuth(r, w)
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
		ParseFiles("templates/privacy.html", "templates/header.gohtml", "templates/footer.gohtml"))

	data := PageData{PageTitle: "Privacy Policy"}

	userAuth := getUserAuth(r, w)
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
	// Optional: Explicit load with error checking for production
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — using system env vars")
	}

	if err := loadCosts(); err != nil {
		fmt.Println("Error loading costs:", err)
		os.Exit(1)
	}
	devMode := flag.Bool("dev", false, "Run in development mode (localhost only)")
	flag.Parse()

	// New Relic APM — disabled gracefully if license key is not set
	nrAppName := "colout2"
	if strings.Contains(os.Getenv("SERVER_ADDR"), "localhost") {
		nrAppName = "colout2-test"
	}
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(nrAppName),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigAppLogForwardingEnabled(true),
		newrelic.ConfigAIMonitoringEnabled(true),
	)
	if err != nil {
		log.Printf("New Relic not enabled: %v", err)
		app = nil
	}
	nrApp = app

	mux := http.NewServeMux()

	nrHandle := func(pattern string, handler http.HandlerFunc) {
		if app != nil {
			p, h := newrelic.WrapHandleFunc(app, pattern, handler)
			mux.HandleFunc(p, h)
		} else {
			mux.HandleFunc(pattern, handler)
		}
	}

	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	mux.HandleFunc("/f7897e50677c40c4864e7f10255812bd.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/f7897e50677c40c4864e7f10255812bd.txt") // https://www.bing.com/indexnow/getstarted
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "images/colout2.png") // Adjust path to your file
	})

	nrHandle("/estimate", estimateHandler)
	nrHandle("/estimate/delete/", estimateDeleteHandler)
	nrHandle("/estimate/send/{estimateID}", emailSendHandler)
	nrHandle("/estimate/materials/{estimateID}", materialsHandler)
	nrHandle("/estimate/view/{token}", estimateTokenHandler)
	nrHandle("/estimate/print/{token}", estimatePrintHandler)
	nrHandle("/estimate/fork/{token}", estimateForkHandler)
	nrHandle("/estimate/accept/{token}", estimateAcceptHandler)
	nrHandle("/estimate/{estimateID}", estimateDBHandler)
	nrHandle("/customer", customerHandler)
	nrHandle("/session", sessionHandler)
	nrHandle("/deck-calculator", calcHandler)
	mux.HandleFunc("/calc", func(w http.ResponseWriter, r *http.Request) {
		// Preserve query string on redirect
		target := "/deck-calculator"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	nrHandle("/css/", cssHandler)
	nrHandle("/contact", contactHandler)
	nrHandle("/contact/", contactHandler)
	nrHandle("/login", loginHandler)
	nrHandle("/signup", signupHandler)
	nrHandle("/auth/google", googleLoginHandler)
	nrHandle("/auth/google/callback", googleCallbackHandler)
	nrHandle("/sitemap.xml", sitemapHandler)
	nrHandle("/robots.txt", robotsTxtHandler)
	nrHandle("/error404", notFoundHandler)
	mux.HandleFunc("/.well-known/appspecific/com.chrome.devtools.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	nrHandle("/api/calc/deck", apiCalcDeckHandler)
	nrHandle("/admin", adminHandler)
	nrHandle("/admin/contractor/action", adminContractorActionHandler)
	nrHandle("/contractor", contractorLandingHandler)
	nrHandle("/contractor/register", contractorRegisterHandler)
	nrHandle("/my-estimates", myEstimatesHandler)
	nrHandle("/privacy", privacyHandler)
	nrHandle("/", ownerHandler)

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
	err = http.ListenAndServe(addr, mux)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
