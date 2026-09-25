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

type PhotoFull struct {
	URI         string   `yaml:"uri"`
	MD5         string   `yaml:"md5,omitempty"`
	SourcePath  string   `yaml:"source_path,omitempty"`
	City        string   `yaml:"city"`
	Category    string   `yaml:"category"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags,omitempty"`
	Featured    bool     `yaml:"featured"`
	Reviewed    bool     `yaml:"reviewed"`
	Project     string   `yaml:"project,omitempty"`
}

type adminPhotosPageData struct {
	Photos          []PhotoFull
	ProjectGroups   []ProjectGroup
	Projects        []string
	Total           int
	Reviewed        int
	Remaining       int
	View            string
	SelectedProject string
}

const photosYAMLPath = "static/photos.yaml"

// unassignedProjectLabel groups photos with no project set, on the All Photos
// overview grid, so they're still reachable instead of silently dropped.
const unassignedProjectLabel = "(No Project)"

// groupPhotosByProject builds one ProjectGroup per distinct project (ignoring
// reviewed status), for the All Photos overview grid at /admin/photos — this
// keeps that page to one thumbnail per project instead of rendering every
// full-resolution photo at once.
func groupPhotosByProject(photos []PhotoFull) []ProjectGroup {
	groups := map[string]*ProjectGroup{}
	var order []string
	for _, p := range photos {
		name := p.Project
		if name == "" {
			name = unassignedProjectLabel
		}
		g, ok := groups[name]
		if !ok {
			g = &ProjectGroup{Name: name}
			groups[name] = g
			order = append(order, name)
		}
		g.Photos = append(g.Photos, p)
	}

	result := make([]ProjectGroup, 0, len(order))
	for _, name := range order {
		g := groups[name]
		g.Cover = g.Photos[0]
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
	return result
}

func loadPhotosFull() ([]PhotoFull, error) {
	data, err := os.ReadFile(photosYAMLPath)
	if err != nil {
		return nil, err
	}
	var photos []PhotoFull
	if err := yaml.Unmarshal(data, &photos); err != nil {
		return nil, err
	}
	return photos, nil
}

func savePhotosFull(photos []PhotoFull) error {
	data, err := yaml.Marshal(photos)
	if err != nil {
		return err
	}
	return os.WriteFile(photosYAMLPath, data, 0644)
}

func adminPhotosHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated {
		http.Redirect(w, r, "/login?rurl=/admin/photos", http.StatusFound)
		return
	}
	if !isAdminUser(userAuth.Email) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	photos, err := loadPhotosFull()
	if err != nil {
		log.Printf("adminPhotosHandler: %v", err)
		http.Error(w, "Failed to load photos", http.StatusInternalServerError)
		return
	}

	var unreviewed []PhotoFull
	reviewedCount := 0
	projectSeen := map[string]bool{}
	var projects []string
	for _, p := range photos {
		if p.Reviewed {
			reviewedCount++
		} else {
			unreviewed = append(unreviewed, p)
		}
		if p.Project != "" && !projectSeen[p.Project] {
			projectSeen[p.Project] = true
			projects = append(projects, p.Project)
		}
	}

	view := r.URL.Query().Get("view")
	selectedProject := ""
	var displayed []PhotoFull
	var groups []ProjectGroup
	switch view {
	case "all":
		if project := r.URL.Query().Get("project"); project != "" {
			selectedProject = project
			for _, p := range photos {
				name := p.Project
				if name == "" {
					name = unassignedProjectLabel
				}
				if name == project {
					displayed = append(displayed, p)
				}
			}
			sort.SliceStable(displayed, func(i, j int) bool {
				return displayed[i].Featured && !displayed[j].Featured
			})
		} else {
			groups = groupPhotosByProject(photos)
		}
	case "featured":
		for _, p := range photos {
			if p.Featured {
				displayed = append(displayed, p)
			}
		}
	default:
		view = "unreviewed"
		displayed = unreviewed
	}

	userAuth.Title = "Photo Review"
	userAuth.CanonicalPath = "/admin/photos"
	userAuth.Subtitle = "Review and approve photos for the gallery"

	tmpl := template.Must(template.New("admin_photos.gohtml").Funcs(funcMap).ParseFiles(
		"templates/admin_photos.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page: &adminPhotosPageData{
			Photos:          displayed,
			ProjectGroups:   groups,
			Projects:        projects,
			Total:           len(photos),
			Reviewed:        reviewedCount,
			Remaining:       len(unreviewed),
			View:            view,
			SelectedProject: selectedProject,
		},
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "admin_photos.gohtml", rd); err != nil {
		log.Printf("adminPhotosHandler execute: %v", err)
		http.Error(w, "Server Error", 500)
	}
}

// adminPhotosSaveHandler approves a photo and updates its metadata.
func adminPhotosSaveHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated || !isAdminUser(userAuth.Email) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		return
	}

	var req struct {
		URI         string   `json:"uri"`
		City        string   `json:"city"`
		Category    string   `json:"category"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Featured    bool     `json:"featured"`
		Project     string   `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
		return
	}

	photos, err := loadPhotosFull()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to load photos"})
		return
	}

	found := false
	for i, p := range photos {
		if p.URI == req.URI {
			photos[i].City = req.City
			photos[i].Category = req.Category
			photos[i].Description = req.Description
			photos[i].Tags = req.Tags
			photos[i].Featured = req.Featured
			photos[i].Project = req.Project
			photos[i].Reviewed = true
			found = true
			break
		}
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "photo not found"})
		return
	}

	if err := savePhotosFull(photos); err != nil {
		log.Printf("adminPhotosSaveHandler: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// adminPhotosDeleteHandler removes a photo entry from the YAML.
func adminPhotosDeleteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated || !isAdminUser(userAuth.Email) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		return
	}

	var req struct {
		URI string `json:"uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
		return
	}

	photos, err := loadPhotosFull()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to load photos"})
		return
	}

	filtered := photos[:0]
	for _, p := range photos {
		if p.URI != req.URI {
			filtered = append(filtered, p)
		}
	}

	if err := savePhotosFull(filtered); err != nil {
		log.Printf("adminPhotosDeleteHandler: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
