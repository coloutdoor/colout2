package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
)

const photosGCSBucket = "columbiaoutdoor-images"

// allowedPhotoExt maps accepted upload file extensions to their content
// type. HEIC/HEIF is intentionally not supported here — convert to JPEG
// before uploading (e.g. export from Photos), or use
// scripts/sync_photo_to_gcs.py for a batch of raw iPhone photos.
var allowedPhotoExt = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// AdminProjectRow is one row on the /admin/projects list.
type AdminProjectRow struct {
	ID          int64
	Name        string
	Description string
	PhotoCount  int
}

type adminProjectsPageData struct {
	Projects []AdminProjectRow
	Message  string
	Error    string
}

// adminProjectsHandler handles GET /admin/projects: lists every project with
// its photo count, plus the create-project and upload-photos forms.
func adminProjectsHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated {
		http.Redirect(w, r, "/login?rurl=/admin/projects", http.StatusFound)
		return
	}
	if !isAdminUser(userAuth.Email) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	db, err := photosDB()
	if err != nil {
		log.Printf("adminProjectsHandler: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT pr.id, pr.name, pr.description, COUNT(p.id)
		FROM projects pr
		LEFT JOIN photos p ON p.project_id = pr.id
		GROUP BY pr.id, pr.name, pr.description
		ORDER BY pr.name`)
	if err != nil {
		log.Printf("adminProjectsHandler: query: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var projects []AdminProjectRow
	for rows.Next() {
		var p AdminProjectRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PhotoCount); err != nil {
			log.Printf("adminProjectsHandler: scan: %v", err)
			continue
		}
		projects = append(projects, p)
	}

	userAuth.Title = "Projects Admin"
	userAuth.CanonicalPath = "/admin/projects"
	userAuth.Subtitle = "Create projects and upload photos"

	tmpl := template.Must(template.New("admin_projects.gohtml").Funcs(funcMap).ParseFiles(
		"templates/admin_projects.gohtml",
		"templates/header.gohtml",
		"templates/footer.gohtml",
	))

	rd := renderData{
		Page: &adminProjectsPageData{
			Projects: projects,
			Message:  r.URL.Query().Get("msg"),
			Error:    r.URL.Query().Get("err"),
		},
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "admin_projects.gohtml", rd); err != nil {
		log.Printf("adminProjectsHandler execute: %v", err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
	}
}

// adminProjectsCreateHandler handles POST /admin/projects/create.
func adminProjectsCreateHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated || !isAdminUser(userAuth.Email) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		redirectWithMsg(w, r, "", "Project name is required")
		return
	}

	db, err := photosDB()
	if err != nil {
		log.Printf("adminProjectsCreateHandler: %v", err)
		redirectWithMsg(w, r, "", "Database error")
		return
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO projects (name, description) VALUES ($1, $2)`, name, description); err != nil {
		log.Printf("adminProjectsCreateHandler: insert: %v", err)
		redirectWithMsg(w, r, "", "Could not create project — that name may already exist")
		return
	}

	redirectWithMsg(w, r, "Created project \""+name+"\"", "")
}

// adminProjectsUploadHandler handles POST /admin/projects/upload — a
// multipart form with a project_id and one or more files under the "photos"
// field. Each file is deduplicated by MD5, uploaded to GCS, and inserted
// into the photos table as reviewed and ready to show immediately (no
// separate /admin/photos review step, since the admin curated it here).
func adminProjectsUploadHandler(w http.ResponseWriter, r *http.Request) {
	userAuth := getUserAuth(r, w)
	if !userAuth.IsAuthenticated || !isAdminUser(userAuth.Email) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("adminProjectsUploadHandler: ParseMultipartForm (Content-Length %d, Content-Type %q): %v",
			r.ContentLength, r.Header.Get("Content-Type"), err)
		redirectWithMsg(w, r, "", "Upload too large or invalid form: "+err.Error())
		return
	}

	projectID, err := strconv.ParseInt(r.FormValue("project_id"), 10, 64)
	if err != nil || projectID <= 0 {
		redirectWithMsg(w, r, "", "Invalid project")
		return
	}

	var files []*multipart.FileHeader
	if r.MultipartForm != nil {
		files = r.MultipartForm.File["photos"]
	}
	if len(files) == 0 {
		redirectWithMsg(w, r, "", "No files selected")
		return
	}

	db, err := photosDB()
	if err != nil {
		log.Printf("adminProjectsUploadHandler: %v", err)
		redirectWithMsg(w, r, "", "Database error")
		return
	}
	defer db.Close()

	ctx := r.Context()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("adminProjectsUploadHandler: storage.NewClient: %v", err)
		redirectWithMsg(w, r, "", "Could not connect to photo storage")
		return
	}
	defer client.Close()
	bucket := client.Bucket(photosGCSBucket)

	var uploaded, skipped, failed int
	for _, fh := range files {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		contentType, ok := allowedPhotoExt[ext]
		if !ok {
			log.Printf("adminProjectsUploadHandler: rejected %q: unsupported file type", fh.Filename)
			failed++
			continue
		}

		f, err := fh.Open()
		if err != nil {
			log.Printf("adminProjectsUploadHandler: open %q: %v", fh.Filename, err)
			failed++
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			log.Printf("adminProjectsUploadHandler: read %q: %v", fh.Filename, err)
			failed++
			continue
		}

		sum := md5.Sum(data)
		hash := hex.EncodeToString(sum[:])

		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM photos WHERE md5 = $1)`, hash).Scan(&exists); err != nil {
			log.Printf("adminProjectsUploadHandler: dedup check for %q: %v", fh.Filename, err)
			failed++
			continue
		}
		if exists {
			skipped++
			continue
		}

		objectName := hash + ext
		wc := bucket.Object(objectName).NewWriter(ctx)
		wc.ContentType = contentType
		if _, err := wc.Write(data); err != nil {
			log.Printf("adminProjectsUploadHandler: gcs write %q: %v", fh.Filename, err)
			wc.Close()
			failed++
			continue
		}
		if err := wc.Close(); err != nil {
			log.Printf("adminProjectsUploadHandler: gcs close %q: %v", fh.Filename, err)
			failed++
			continue
		}

		publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", photosGCSBucket, objectName)
		_, err = db.Exec(`
			INSERT INTO photos (uri, md5, source_path, city, category, description, tags, featured, reviewed, project_id)
			VALUES ($1, $2, $3, '', '', '', '{}', false, true, $4)`,
			publicURL, hash, fh.Filename, projectID)
		if err != nil {
			log.Printf("adminProjectsUploadHandler: insert %q: %v", fh.Filename, err)
			failed++
			continue
		}
		uploaded++
	}

	msg := fmt.Sprintf("Uploaded %d photo(s)", uploaded)
	if skipped > 0 {
		msg += fmt.Sprintf(", %d duplicate(s) skipped", skipped)
	}
	if failed > 0 {
		msg += fmt.Sprintf(", %d failed", failed)
	}
	redirectWithMsg(w, r, msg, "")
}

func redirectWithMsg(w http.ResponseWriter, r *http.Request, msg, errMsg string) {
	q := url.Values{}
	if msg != "" {
		q.Set("msg", msg)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	target := "/admin/projects"
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
