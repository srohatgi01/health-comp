package config

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func InitSqlite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	pragmas := `
	PRAGMA journal_mode = WAL;
	PRAGMA foreign_keys = ON;
	PRAGMA synchronous = NORMAL;
	`

	if _, err := db.Exec(pragmas); err != nil {
		return nil, err
	}

	return db, nil
}

func RunMigrations(db *sql.DB) error {
	queries := []string{
		// To keep the heatlh logs
		`CREATE TABLE IF NOT EXISTS body_metrics (
    		id INTEGER PRIMARY KEY AUTOINCREMENT,
    		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    		metric_name TEXT NOT NULL,
    		value REAL NOT NULL,      
    		unit TEXT NOT NULL 
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	return nil
}
