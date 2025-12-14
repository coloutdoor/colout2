package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

// CityPageData – everything your template needs
type CityPageData struct {
	City       string // "Seattle"
	State      string // "WA"
	Service    string // "Deck Builders", "Patio Cover Contractors", etc.
	Title      string
	H1         string
	MetaDesc   string
	Phone      string // optional local number
	SchemaJSON string // for local business schema
}

// ————————————————————————————————————————
// ALL CITY-SPECIFIC URL SLUGS – 100% AUTOMATIC
// ————————————————————————————————————————
var serviceSlugs = []string{
	"deck-builders",
	"patio-cover-contractors",
	"trex-deck-installers",
	"timbertech-deck-installers",
	"composite-decking",
	"outdoor-kitchen-builders",
	"pergola-builders",
	"outdoor-living",
}

var cities = []struct {
	Name  string // pretty name
	Slug  string // URL-safe
	State string // WA, OR, ID
}{
	{"Woodland", "woodland", "wa"},
	{"Ridgefield", "ridgefield", "wa"},
	{"Kalama", "kalama", "wa"},
	{"La Center", "lacenter", "wa"}, // note: "la-center" would also be fine
}

// Generate every combination once at startup
var allCityPageSlugs []string

func init() {
	for _, service := range serviceSlugs {
		for _, city := range cities {
			slug := fmt.Sprintf("%s-%s-%s", service, city.Slug, city.State)
			allCityPageSlugs = append(allCityPageSlugs, slug)
		}
	}
	// Result: 8 services × 4 cities = 32 perfect URLs
}

func sitemapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)

	base := "https://columbiaoutdoor.com"
	pages := []string{"/contact", "/login"}

	// Put the main page and Calculator as 1.0
	fmt.Fprintf(w, "<url><loc>%s</loc><priority>1.0</priority></url>\n", base)
	fmt.Fprintf(w, "<url><loc>%s/calc</loc><priority>1.0</priority></url>\n", base)

	for _, p := range pages {
		fmt.Fprintf(w, "<url><loc>%s%s</loc><priority>0.8</priority></url>\n", base, p)
	}
	for _, slug := range allCityPageSlugs {
		fmt.Fprintf(w, "<url><loc>%s/%s</loc><priority>0.9</priority><changefreq>weekly</changefreq></url>\n", base, slug)
	}
	fmt.Fprint(w, "</urlset>")
}

// Cities
// City specfic landing pages.
//
//  /deck-builders-vancouver-wa
//  /deck-builders-woodland-wa
//  /deck-builders-kalama-wa

func cityHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path // e.g. "/deck-builders-seattle-wa"

	// 1. Extract service + city + state from slug
	service, city, state := parseCitySlug(path)
	if city == "" {
		http.NotFound(w, r)
		return
	}

	// 2. Capitalize nicely
	cityPretty := strings.Title(strings.ReplaceAll(city, "-", " "))
	stateUpper := strings.ToUpper(state)

	// 3. Build SEO-perfect strings
	data := CityPageData{
		City:     cityPretty,
		State:    stateUpper,
		Service:  prettifyService(service),
		Title:    prettifyService(service) + " in " + cityPretty + ", " + stateUpper,
		H1:       prettifyService(service) + " in " + cityPretty + ", " + stateUpper,
		MetaDesc: "Top-rated " + strings.ToLower(prettifyService(service)) + " in " + cityPretty + ", " + stateUpper + ". Pre-approved plans, permits included. Get your free quote today.",
		Phone:    "(360) 219-9434", // or pull from a map of local numbers
	}

	// Optional: Add JSON-LD schema
	data.SchemaJSON = generateSchema(data)

	userAuth := getUserAuth(r)
	userAuth.Title = data.Title
	userAuth.MetaDesc = data.MetaDesc

	log.Printf("Renderign City Data with %+v", data)

	rd := renderData{
		Page:   &data,
		Header: &userAuth,
	}
	tmpl := template.Must(template.New("city.html").Funcs(funcMap).
		ParseFiles("templates/city.html", "templates/header.html", "templates/footer.html"))

	if err := tmpl.ExecuteTemplate(w, "city.html", rd); err != nil {
		log.Printf("ownerHandler execute error: %v", err)
		panic(err)
	}
}

func parseCitySlug(path string) (service, city, state string) {
	// Remove leading/trailing slash and split
	parts := strings.Split(strings.Trim(path, "/"), "-")

	if len(parts) < 4 {
		return "", "", "" // too short → 404
	}

	// Last two parts are always city + state
	state = parts[len(parts)-1]
	city = parts[len(parts)-2]

	// Everything before that is the service
	service = strings.Join(parts[:len(parts)-2], "-")
	return
}

func prettifyService(slug string) string {
	m := map[string]string{
		"deck-builders":              "Deck Builders",
		"patio-cover-contractors":    "Patio Cover Contractors",
		"trex-deck-installers":       "Trex Deck Installers",
		"timbertech-deck-installers": "Trex Deck Installers",
		"composite-decking":          "Composite Decking Experts",
		"outdoor-kitchen-builders":   "Outdoor Kitchen Builders",
		"pergola-builders":           "Pergola Builders",
		"outdoor-living":             "Outdoor Living",
	}
	if pretty, ok := m[slug]; ok {
		return pretty
	}
	// Fallback: replace hyphens with spaces and title case
	return strings.Title(strings.ReplaceAll(slug, "-", " "))
}

func generateSchema(data CityPageData) string {
	// Returns full JSON-LD LocalBusiness schema – Google loves this
	return `{
  "@context": "https://schema.org",
  "@type": "HomeAndConstructionBusiness",
  "name": "Columbia Outdoor – ` + data.Service + ` in ` + data.City + `",
  "telephone": "` + data.Phone + `",
  "address": {
    "@type": "PostalAddress",
    "addressLocality": "` + data.City + `",
    "addressRegion": "` + data.State + `"
  },
  "areaServed": "` + data.City + `, ` + data.State + `"
}`
}
