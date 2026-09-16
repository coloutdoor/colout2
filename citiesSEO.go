package main

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

// sitemapImage is one <image:image> entry attached to a <url> in sitemap.xml,
// so Google Images can discover and index the photo without crawling the page's HTML.
type sitemapImage struct {
	loc     string
	caption string
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// projectsPageImages returns an <image:image> entry for every reviewed photo
// shown on /projects, sourced live from static/photos.yaml.
func projectsPageImages() []sitemapImage {
	photos, err := loadPhotosFull()
	if err != nil {
		log.Printf("projectsPageImages: %v", err)
		return nil
	}
	images := make([]sitemapImage, 0, len(photos))
	for _, p := range photos {
		if !p.Reviewed || p.URI == "" {
			continue
		}
		images = append(images, sitemapImage{loc: p.URI, caption: p.Description})
	}
	return images
}

func sitemapHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">`)

	base := "https://columbiaoutdoor.com"
	// lastmod is the date this page's content last meaningfully changed —
	// bump it by hand whenever you edit that page's template/copy.
	pages := []struct {
		path     string
		priority string
		lastmod  string
		images   []sitemapImage
	}{
		{"/", "1.0", "2026-09-15", nil},
		{"/deck-calculator", "1.0", "2026-08-19", nil},
		{"/contact", "0.8", "2026-08-22", nil},
		{"/patio-cover-contractors-woodland-wa", "0.9", "2026-09-16", patioCoverWoodlandImages},
		{"/deck-builders-woodland-wa", "0.9", "2026-09-16", deckBuildersWoodlandImages},
		{"/outdoor-living-woodland-wa", "0.9", "2026-09-16", outdoorLivingWoodlandImages},
		{"/projects", "0.8", "2026-09-15", projectsPageImages()},
	}

	for _, p := range pages {
		_, _ = fmt.Fprintf(w, "<url><loc>%s%s</loc><lastmod>%s</lastmod><priority>%s</priority>", base, p.path, p.lastmod, p.priority)
		for _, img := range p.images {
			_, _ = fmt.Fprintf(w, "<image:image><image:loc>%s</image:loc><image:caption>%s</image:caption></image:image>",
				xmlEscape(img.loc), xmlEscape(img.caption))
		}
		_, _ = fmt.Fprint(w, "</url>\n")
	}
	_, _ = fmt.Fprint(w, "</urlset>")
}

// patioCoverWoodlandImages mirrors the photos shown in
// templates/patio-cover-contractors-woodland-wa.gohtml.
var patioCoverWoodlandImages = []sitemapImage{
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0507.jpg", "Standard cover frame"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0508.jpg", "Standard cover with vaulted trusses"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0421.jpg", "Cover engineered trusses"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0510.jpg", "Timberframe finish no hardware"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0509.jpg", "Timberframe beam connection"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0549.jpg", "Woodland basic cover frame — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0475.jpg", "Lean to cover skyjacks"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0060.jpeg", "Lean-to patio cover framing — Vancouver, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0545.jpg", "White cover 2 fans and concrete"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0546.jpg", "White cover finish lean to"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0547.jpg", "White patio cover pergola style"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0450.jpg", "Finish cover standard"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0050.jpg", "Woodland Timberframe king stud and cedar finish — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0049.jpg", "Woodland King stud and struts timberframe — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0053.jpg", "Woodland deck and timberframe — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0015.jpg", "Cathlamet deck and timberframe cover — Cathlamet, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0020.jpg", "Cathlamet roof Timberframe — Cathlamet, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0473.jpg", "Large cover overframe"},
}

// deckBuildersWoodlandImages mirrors the photos shown in
// templates/deck-builders-woodland-wa.gohtml.
var deckBuildersWoodlandImages = []sitemapImage{
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0405.jpg", "Large deck with river view — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0404.jpg", "Large Fiberon deck, Tuscan villa — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0309.jpg", "Trex enhance naturals toasted sand with drink rail — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0204.jpg", "Timbertech deck with cedar rails and steps — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0263.jpg", "Deck with steps, cedar rails and cedar trim — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0313.jpg", "Commercial deck with 42 inch aluminum rails — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0503.jpg", "Large deck with Fiberon and steel cinch rail — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0339.jpg", "Deck with custom ramp and aluminum rails — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0525.jpg", "Drink rail with lights, cedar with aluminum balusters — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0391.jpg", "Deck with cladding, aluminum rail, and cable — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0099.png", "Deck with stainless steel cable rail — Kalama, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0126.jpeg", "Two level Trex deck with aluminum rails — La Center, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0112.jpg", "Timbertech deck with aluminum rails and arbor — Battle Ground, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0109.jpg", "Custom deck with wrap around stairs — Vancouver, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0174.jpg", "Custom cedar deck around hot tub — Camas, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0247.jpg", "Custom cedar privacy screen on deck — La Center, WA"},
}

// outdoorLivingWoodlandImages mirrors the photos shown in
// templates/outdoor-living-woodland-wa.gohtml.
var outdoorLivingWoodlandImages = []sitemapImage{
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0378.jpg", "Large deck and cover with fireplace and lights — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0262.jpg", "Outdoor room with bar and cedar finish — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0298.jpg", "Cedar tongue and groove finish with ceiling fan — Vancouver, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0373.jpg", "Large timberframe cover with deck and skylights — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0556.jpg", "Cover painted and finished with wrapped posts and gutters — Vancouver, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0303.jpg", "Patio cover with lights and skylights — Camas, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0260.jpg", "Cover with bar for outdoor living — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0560.jpg", "Cover with lights and fan — Vancouver, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0474.jpg", "Large deck and stairs with hogwire rail infill — Brush Prairie, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0465.jpg", "Deck with gazebo and hot tub — Washougal, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0491.jpg", "Upper deck with cover and stairs — Brush Prairie, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0082.jpeg", "Enclosed outdoor cover — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0336.jpg", "Cover and deck restoration — Woodland, WA"},
	{"https://storage.googleapis.com/columbiaoutdoor-images/photos-0492.jpeg", "Outdoor kitchen sink — Woodland, WA"},
}

// patioCoverWoodlandHandler serves a dedicated, hand-written landing page
// (not an auto-generated thin city page) for the Woodland, WA patio cover ad campaign.
func patioCoverWoodlandHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Patio Cover Contractors in Woodland, WA"
	userAuth.Subtitle = "Transparent pricing, quality craftsmanship, and expert project management for patio covers in Woodland and SW Washington."
	userAuth.MetaDesc = "Columbia Outdoor builds custom patio covers in Woodland, WA and across Clark and Cowlitz County. Transparent pricing, licensed builders, and a dedicated project manager on every job."
	userAuth.CanonicalPath = "/patio-cover-contractors-woodland-wa"

	tmpl := template.Must(template.New("patio-cover-contractors-woodland-wa.gohtml").Funcs(funcMap).ParseFiles(
		"templates/patio-cover-contractors-woodland-wa.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page:   nil,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "patio-cover-contractors-woodland-wa.gohtml", rd); err != nil {
		log.Printf("patioCoverWoodlandHandler error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// deckBuildersWoodlandHandler serves a dedicated, hand-written landing page
// for the Woodland, WA deck builder ad campaign.
func deckBuildersWoodlandHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Deck Builders in Woodland, WA"
	userAuth.Subtitle = "Transparent pricing, quality craftsmanship, and expert project management for custom decks in Woodland and SW Washington."
	userAuth.MetaDesc = "Columbia Outdoor builds custom decks in Woodland, WA and across Clark and Cowlitz County. Transparent pricing, licensed builders, and a dedicated project manager on every job."
	userAuth.CanonicalPath = "/deck-builders-woodland-wa"

	tmpl := template.Must(template.New("deck-builders-woodland-wa.gohtml").Funcs(funcMap).ParseFiles(
		"templates/deck-builders-woodland-wa.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page:   nil,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "deck-builders-woodland-wa.gohtml", rd); err != nil {
		log.Printf("deckBuildersWoodlandHandler error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// outdoorLivingWoodlandHandler serves a dedicated, hand-written landing page
// for the Woodland, WA outdoor living ad campaign.
func outdoorLivingWoodlandHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	userAuth.Title = "Outdoor Living Contractors in Woodland, WA"
	userAuth.Subtitle = "Transparent pricing, quality craftsmanship, and expert project management for decks, covers, and outdoor living spaces in Woodland and SW Washington."
	userAuth.MetaDesc = "Columbia Outdoor builds custom decks, patio covers, and outdoor living spaces in Woodland, WA and across Clark and Cowlitz County. Transparent pricing, licensed builders, and a dedicated project manager on every job."
	userAuth.CanonicalPath = "/outdoor-living-woodland-wa"

	tmpl := template.Must(template.New("outdoor-living-woodland-wa.gohtml").Funcs(funcMap).ParseFiles(
		"templates/outdoor-living-woodland-wa.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page:   nil,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "outdoor-living-woodland-wa.gohtml", rd); err != nil {
		log.Printf("outdoorLivingWoodlandHandler error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
