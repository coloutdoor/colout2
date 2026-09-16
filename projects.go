package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectGroup is a set of reviewed photos that share a Project name, along
// with the single cover photo shown on the /projects grid.
type ProjectGroup struct {
	Name        string
	Cover       PhotoFull
	Photos      []PhotoFull
	Description string
}

const projectDescriptionsYAMLPath = "static/project_descriptions.yaml"

// loadProjectDescriptions reads the hand-written, one-time-authored blurb for
// each project group, keyed by the same "project" value used in photos.yaml.
// These are written once (not regenerated per request) so /projects has real
// body text alongside its photo grid instead of images alone.
func loadProjectDescriptions() (map[string]string, error) {
	data, err := os.ReadFile(projectDescriptionsYAMLPath)
	if err != nil {
		return nil, err
	}
	var descriptions map[string]string
	if err := yaml.Unmarshal(data, &descriptions); err != nil {
		return nil, err
	}
	return descriptions, nil
}

// CategoryOption is one entry in the /projects category filter dropdown.
type CategoryOption struct {
	Value string
	Label string
}

// categoryLabels gives a friendly display name for each category value,
// in the preferred display order. Matches the options in the admin editor
// (templates/admin_photos.gohtml).
var categoryOrder = []CategoryOption{
	{"deck", "Deck"},
	{"cover", "Cover"},
	{"stairs", "Stairs"},
	{"rails", "Rails"},
	{"patio", "Patio"},
	{"fence", "Fence"},
	{"farm", "Farm / Shed / Coop"},
	{"other", "Other"},
}

// projectPhoto is the subset of PhotoFull exposed to the public /projects
// page's carousel data — internal admin fields (source_path, md5) are left out.
type projectPhoto struct {
	URI         string   `json:"uri"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Featured    bool     `json:"featured"`
	Tags        []string `json:"tags,omitempty"`
}

type projectsPageData struct {
	Projects         []ProjectGroup
	Categories       []CategoryOption
	SelectedCategory string
	SelectedKeyword  string
}

var validCategoryValues = func() map[string]bool {
	m := make(map[string]bool, len(categoryOrder))
	for _, c := range categoryOrder {
		m[c.Value] = true
	}
	return m
}()

func projectPhotosJSON(photos []PhotoFull) (string, error) {
	out := make([]projectPhoto, len(photos))
	for i, p := range photos {
		out[i] = projectPhoto{URI: p.URI, Description: p.Description, Category: p.Category, Featured: p.Featured, Tags: p.Tags}
	}
	b, err := json.Marshal(out)
	return string(b), err
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	photos, err := loadPhotosFull()
	if err != nil {
		log.Printf("projectsHandler: %v", err)
		http.Error(w, "Failed to load photos", http.StatusInternalServerError)
		return
	}

	groups := map[string]*ProjectGroup{}
	var order []string
	categoriesSeen := map[string]bool{}
	for _, p := range photos {
		if !p.Reviewed || p.Project == "" {
			continue
		}
		g, ok := groups[p.Project]
		if !ok {
			g = &ProjectGroup{Name: p.Project}
			groups[p.Project] = g
			order = append(order, p.Project)
		}
		g.Photos = append(g.Photos, p)
		if p.Category != "" {
			categoriesSeen[p.Category] = true
		}
	}

	descriptions, err := loadProjectDescriptions()
	if err != nil {
		log.Printf("projectsHandler: loadProjectDescriptions: %v", err)
	}

	result := make([]ProjectGroup, 0, len(order))
	for _, name := range order {
		g := groups[name]
		g.Cover = g.Photos[0]
		g.Description = descriptions[name]
		for _, p := range g.Photos {
			if p.Featured {
				g.Cover = p
				break
			}
		}
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	categories := make([]CategoryOption, 0, len(categoryOrder))
	for _, c := range categoryOrder {
		if categoriesSeen[c.Value] {
			categories = append(categories, c)
			delete(categoriesSeen, c.Value)
		}
	}
	var leftover []string
	for c := range categoriesSeen {
		leftover = append(leftover, c)
	}
	sort.Strings(leftover)
	for _, c := range leftover {
		categories = append(categories, CategoryOption{Value: c, Label: c})
	}

	userAuth := getUserAuth(r, w)
	userAuth.Title = "Our Projects"
	userAuth.Subtitle = "Browse completed decks, covers, and outdoor structures by project."
	userAuth.MetaDesc = "Browse completed deck and outdoor living projects by Columbia Outdoor across SW Washington."
	userAuth.CanonicalPath = "/projects"

	tmpl := template.Must(template.New("projects.gohtml").Funcs(funcMap).ParseFiles(
		"templates/projects.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	category := r.URL.Query().Get("category")
	if !validCategoryValues[category] {
		category = ""
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))

	rd := renderData{
		Page: &projectsPageData{
			Projects:         result,
			Categories:       categories,
			SelectedCategory: category,
			SelectedKeyword:  keyword,
		},
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "projects.gohtml", rd); err != nil {
		log.Printf("projectsHandler execute: %v", err)
		http.Error(w, "Server Error", 500)
	}
}
