package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type FileMeta struct {
	OriginalName string    `json:"original_name"`
	StoreName    string    `json:"store_name"`
	UploadedAt   time.Time `json:"uploaded_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) (*Store, error) {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		original_name TEXT NOT NULL,
		store_name TEXT NOT NULL,
		uploaded_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Set(id string, meta FileMeta) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO files (id, original_name, store_name, uploaded_at, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, meta.OriginalName, meta.StoreName, meta.UploadedAt, meta.ExpiresAt,
	)
	return err
}

func (s *Store) Get(id string) (FileMeta, bool) {
	var meta FileMeta
	row := s.db.QueryRow(
		`SELECT original_name, store_name, uploaded_at, expires_at FROM files WHERE id = ?`, id,
	)

	err := row.Scan(&meta.OriginalName, &meta.StoreName, &meta.UploadedAt, &meta.ExpiresAt)
	if err != nil {
		return FileMeta{}, false
	}

	return meta, true
}

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM files WHERE id = ?`, id)
	return err
}

func (s *Store) All() (map[string]FileMeta, error) {
	rows, err := s.db.Query(`SELECT id, original_name, store_name, uploaded_at, expires_at FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]FileMeta)
	for rows.Next() {
		var id string
		var meta FileMeta
		if err := rows.Scan(&id, &meta.OriginalName, &meta.StoreName, &meta.UploadedAt, &meta.ExpiresAt); err != nil {
			return nil, err
		}
		result[id] = meta
	}

	return result, rows.Err()
}
