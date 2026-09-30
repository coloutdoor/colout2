package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// pgTypeMap wraps scan destinations for Postgres array columns (tags) so
// database/sql's generic Scan can decode them — pgx's stdlib driver only
// supports this via pgtype.Map.SQLScanner, not a bare *[]string.
var pgTypeMap = pgtype.NewMap()

type PhotoFull struct {
	URI         string
	MD5         string
	SourcePath  string
	City        string
	Category    string
	Description string
	Tags        []string
	Featured    bool
	Reviewed    bool
	Project     string
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

// photosDB opens a connection to the photos table's database. Callers must
// close it. Matches the rest of the codebase's convention of opening a
// short-lived connection per call rather than sharing a package-level pool.
func photosDB() (*sql.DB, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	return sql.Open("pgx", dbURL)
}

// loadPhotosFull loads every photo from the photos table, oldest first —
// the same order static/photos.yaml used to preserve by file order.
func loadPhotosFull() ([]PhotoFull, error) {
	db, err := photosDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT p.uri, COALESCE(p.md5,''), COALESCE(p.source_path,''), p.city, p.category, p.description,
		       p.tags, p.featured, p.reviewed, COALESCE(pr.name,'')
		FROM photos p
		LEFT JOIN projects pr ON pr.id = p.project_id
		ORDER BY p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []PhotoFull
	for rows.Next() {
		var p PhotoFull
		if err := rows.Scan(&p.URI, &p.MD5, &p.SourcePath, &p.City, &p.Category, &p.Description,
			pgTypeMap.SQLScanner(&p.Tags), &p.Featured, &p.Reviewed, &p.Project); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
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

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	db, err := photosDB()
	if err != nil {
		log.Printf("adminPhotosSaveHandler: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to connect to database"})
		return
	}
	defer db.Close()

	projectID, err := resolveProjectID(db, req.Project)
	if err != nil {
		log.Printf("adminPhotosSaveHandler: resolveProjectID: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to resolve project"})
		return
	}

	res, err := db.Exec(`
		UPDATE photos
		SET city = $1, category = $2, description = $3, tags = $4, featured = $5, project_id = $6,
		    reviewed = true, updated_at = NOW()
		WHERE uri = $7`,
		req.City, req.Category, req.Description, tags, req.Featured, projectID, req.URI)
	if err != nil {
		log.Printf("adminPhotosSaveHandler: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "photo not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// adminPhotosDeleteHandler removes a photo entry from the database.
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

	db, err := photosDB()
	if err != nil {
		log.Printf("adminPhotosDeleteHandler: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to connect to database"})
		return
	}
	defer db.Close()

	if _, err := db.Exec(`DELETE FROM photos WHERE uri = $1`, req.URI); err != nil {
		log.Printf("adminPhotosDeleteHandler: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// nullIfEmpty converts an empty string to a SQL NULL, for nullable text
// columns (project) where the app's zero value is "" but the column allows
// NULL for "not set".
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// resolveProjectID looks up (or creates) a project by name, for the photo
// review page's free-text "Project" field. Returns nil for an empty name.
func resolveProjectID(db *sql.DB, name string) (interface{}, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	var id int64
	err := db.QueryRow(`
		INSERT INTO projects (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, name).Scan(&id)
	if err != nil {
		return nil, err
	}
	return id, nil
}
