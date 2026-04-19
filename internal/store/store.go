// Package store provides a SQLite-backed store for board notes.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			text         TEXT    NOT NULL,
			created_at   INTEGER NOT NULL,
			expires_at   INTEGER NOT NULL,
			dismissed_at INTEGER
		)
	`)
	return err
}

type Note struct {
	ID          int64
	Text        string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	DismissedAt *time.Time
}

func (n *Note) Active() bool {
	return time.Now().Before(n.ExpiresAt) && n.DismissedAt == nil
}

func (s *Store) CreateNote(text string, duration time.Duration) (*Note, error) {
	now := time.Now()
	expiresAt := now.Add(duration)
	res, err := s.db.Exec(
		"INSERT INTO notes (text, created_at, expires_at) VALUES (?, ?, ?)",
		text, now.Unix(), expiresAt.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("insert note: %w", err)
	}
	id, _ := res.LastInsertId()
	return &Note{
		ID:        id,
		Text:      text,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Store) ActiveNote() (*Note, error) {
	now := time.Now().Unix()
	row := s.db.QueryRow(`
		SELECT id, text, created_at, expires_at
		FROM notes
		WHERE expires_at > ? AND dismissed_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, now)

	var n Note
	var createdAt, expiresAt int64
	if err := row.Scan(&n.ID, &n.Text, &createdAt, &expiresAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan active note: %w", err)
	}
	n.CreatedAt = time.Unix(createdAt, 0)
	n.ExpiresAt = time.Unix(expiresAt, 0)
	return &n, nil
}

func (s *Store) RecentNotes(limit int) ([]*Note, error) {
	rows, err := s.db.Query(`
		SELECT id, text, created_at, expires_at, dismissed_at
		FROM notes
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	var notes []*Note
	for rows.Next() {
		var n Note
		var createdAt, expiresAt int64
		var dismissedAt *int64
		if err := rows.Scan(&n.ID, &n.Text, &createdAt, &expiresAt, &dismissedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		n.CreatedAt = time.Unix(createdAt, 0)
		n.ExpiresAt = time.Unix(expiresAt, 0)
		if dismissedAt != nil {
			t := time.Unix(*dismissedAt, 0)
			n.DismissedAt = &t
		}
		notes = append(notes, &n)
	}
	return notes, rows.Err()
}

func (s *Store) DismissNote(id int64) error {
	_, err := s.db.Exec(
		"UPDATE notes SET dismissed_at = ? WHERE id = ?",
		time.Now().Unix(), id,
	)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}
