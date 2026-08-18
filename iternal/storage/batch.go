package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Batch struct {
	ID        string
	FileIDs   []string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type BatchStore struct {
	db *sql.DB
}

func NewBatchStore(db *sql.DB) (*BatchStore, error) {
	schema := `
	CREATE TABLE IF NOT EXISTS batches (
		id TEXT PRIMARY KEY,
		file_ids TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("creating batches schema: %w", err)
	}

	return &BatchStore{db: db}, nil
}

func (s *BatchStore) Set(id string, b Batch) error {
	fileIDsJSON, err := json.Marshal(b.FileIDs)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO batches (id, file_ids, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		id, string(fileIDsJSON), b.CreatedAt, b.ExpiresAt,
	)
	return err
}

func (s *BatchStore) Get(id string) (Batch, bool) {
	var b Batch
	var fileIDsJSON string

	row := s.db.QueryRow(`SELECT file_ids, created_at, expires_at FROM batches WHERE id = ?`, id)
	if err := row.Scan(&fileIDsJSON, &b.CreatedAt, &b.ExpiresAt); err != nil {
		return Batch{}, false
	}

	if err := json.Unmarshal([]byte(fileIDsJSON), &b.FileIDs); err != nil {
		return Batch{}, false
	}

	b.ID = id
	return b, true
}
