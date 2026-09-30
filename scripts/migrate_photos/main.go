// migrate_photos is a one-time tool that loads static/photos.yaml into the
// new `photos` table (see sql/photos.sql). Run it once per database:
//
//	go run ./scripts/migrate_photos                        # uses $DATABASE_URL
//	DATABASE_URL="$DATABASE_URL_PROD" go run ./scripts/migrate_photos   # prod
//
// Add -dry-run to preview without writing. Safe to re-run: existing rows
// (matched by uri) are left untouched rather than duplicated.
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

// photoFull mirrors admin_photos.go's PhotoFull — kept as its own copy here
// since this is a standalone `package main` and can't import from the root
// colout2 package.
type photoFull struct {
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

func main() {
	yamlPath := flag.String("yaml", "static/photos.yaml", "path to photos.yaml")
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
	var photos []photoFull
	if err := yaml.Unmarshal(data, &photos); err != nil {
		log.Fatalf("parsing %s: %v", *yamlPath, err)
	}
	fmt.Printf("Parsed %d photos from %s\n", len(photos), *yamlPath)

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("pinging database: %v", err)
	}

	var inserted, skipped int
	for _, p := range photos {
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}

		if *dryRun {
			var exists bool
			if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM photos WHERE uri = $1)`, p.URI).Scan(&exists); err != nil {
				log.Fatalf("checking %s: %v", p.URI, err)
			}
			if exists {
				skipped++
			} else {
				inserted++
			}
			continue
		}

		res, err := db.Exec(`
			INSERT INTO photos (uri, md5, source_path, city, category, description, tags, featured, reviewed, project)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (uri) DO NOTHING`,
			p.URI, nullIfEmpty(p.MD5), nullIfEmpty(p.SourcePath), p.City, p.Category, p.Description,
			tags, p.Featured, p.Reviewed, nullIfEmpty(p.Project))
		if err != nil {
			log.Fatalf("inserting %s: %v", p.URI, err)
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
	fmt.Printf("%s %d, skipped %d already-present rows.\n", verb, inserted, skipped)
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
