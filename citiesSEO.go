package main

import (
	"fmt"
	"net/http"
)

func sitemapHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="https://www.sitemaps.org/schemas/sitemap/0.9">`)

	base := "https://columbiaoutdoor.com"
	pages := []struct {
		path     string
		priority string
	}{
		{"/", "1.0"},
		{"/deck-calculator", "1.0"},
		{"/contact", "0.8"},
	}

	for _, p := range pages {
		_, _ = fmt.Fprintf(w, "<url><loc>%s%s</loc><priority>%s</priority></url>\n", base, p.path, p.priority)
	}
	_, _ = fmt.Fprint(w, "</urlset>")
}
