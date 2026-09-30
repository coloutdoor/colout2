// migrate_projects is a one-time tool that populates the `projects` table
// (see sql/projects.sql) from static/project_descriptions.yaml, then
// backfills photos.project_id by matching the existing photos.project text
// column against projects.name. Run once per database:
//
//	go run ./scripts/migrate_projects                      # uses $DATABASE_URL
//	DATABASE_URL="$DATABASE_URL_PROD" go run ./scripts/migrate_projects  # prod
//
// Add -dry-run to preview without writing. Safe to re-run: existing project
// names are left untouched rather than duplicated or overwritten, and the
// project_id backfill only touches rows where it's currently NULL.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"gopkg.in/yaml.v3"
)

func main() {
	yamlPath := flag.String("yaml", "static/project_descriptions.yaml", "path to project_descriptions.yaml")
	dryRun := flag.Bool("dry-run", false, "preview without writing to the database")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	data, err := os.ReadFile(*yamlPath)
	if err != nil {
		log.Fatalf("reading %s: %v", *yamlPath, err)
	}
	var descriptions map[string]string
	if err := yaml.Unmarshal(data, &descriptions); err != nil {
		log.Fatalf("parsing %s: %v", *yamlPath, err)
	}
	fmt.Printf("Parsed %d project descriptions from %s\n", len(descriptions), *yamlPath)

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("pinging database: %v", err)
	}

	// Collect every project name that needs to exist: everything in the
	// YAML, plus any photos.project value that isn't in the YAML (so no
	// photo silently loses its project when we backfill project_id).
	names := map[string]bool{}
	for name := range descriptions {
		names[name] = true
	}
	rows, err := db.Query(`SELECT DISTINCT project FROM photos WHERE project IS NOT NULL AND project != ''`)
	if err != nil {
		log.Fatalf("querying distinct photo projects: %v", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatalf("scanning project name: %v", err)
		}
		names[name] = true
	}
	rows.Close()
	fmt.Printf("Found %d distinct project names total (YAML + photos.project)\n", len(names))

	var inserted, skipped int
	for name := range names {
		desc := descriptions[name] // "" if this name only came from photos.project

		if *dryRun {
			var exists bool
			if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE name = $1)`, name).Scan(&exists); err != nil {
				log.Fatalf("checking %s: %v", name, err)
			}
			if exists {
				skipped++
			} else {
				inserted++
			}
			continue
		}

		res, err := db.Exec(`INSERT INTO projects (name, description) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, name, desc)
		if err != nil {
			log.Fatalf("inserting project %q: %v", name, err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			inserted++
		} else {
			skipped++
		}
	}
	verb := "Inserted"
	if *dryRun {
		verb = "Would insert"
	}
	fmt.Printf("%s %d projects, skipped %d already-present.\n", verb, inserted, skipped)

	if *dryRun {
		var toBackfill int
		if err := db.QueryRow(`
			SELECT COUNT(*) FROM photos p
			JOIN projects pr ON pr.name = p.project
			WHERE p.project_id IS NULL`).Scan(&toBackfill); err != nil {
			log.Fatalf("counting backfill candidates: %v", err)
		}
		fmt.Printf("Would backfill project_id on %d photos.\n", toBackfill)
		return
	}

	res, err := db.Exec(`
		UPDATE photos p
		SET project_id = pr.id
		FROM projects pr
		WHERE pr.name = p.project AND p.project_id IS NULL`)
	if err != nil {
		log.Fatalf("backfilling photos.project_id: %v", err)
	}
	n, _ := res.RowsAffected()
	fmt.Printf("Backfilled project_id on %d photos.\n", n)

	var orphans int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM photos
		WHERE project IS NOT NULL AND project != '' AND project_id IS NULL`).Scan(&orphans); err != nil {
		log.Fatalf("checking for orphans: %v", err)
	}
	if orphans > 0 {
		fmt.Printf("WARNING: %d photos still have a project name with no matching project_id.\n", orphans)
	} else {
		fmt.Println("No orphaned photo.project values remain.")
	}
}
