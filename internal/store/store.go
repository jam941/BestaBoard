// Package store provides a Postgres-backed store for board data.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	db *sql.DB
}

func Open(connStr string) (*Store, error) {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
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
			id           BIGSERIAL PRIMARY KEY,
			text         TEXT   NOT NULL,
			created_at   BIGINT NOT NULL,
			expires_at   BIGINT NOT NULL,
			dismissed_at BIGINT
		);
		CREATE TABLE IF NOT EXISTS users (
			id            BIGSERIAL PRIMARY KEY,
			username      TEXT UNIQUE NOT NULL,
			password_hash TEXT        NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT   PRIMARY KEY,
			user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at BIGINT NOT NULL
		);
	`)
	return err
}

// ---- Notes ----

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
	var id int64
	err := s.db.QueryRow(
		"INSERT INTO notes (text, created_at, expires_at) VALUES ($1, $2, $3) RETURNING id",
		text, now.Unix(), expiresAt.Unix(),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert note: %w", err)
	}
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
		WHERE expires_at > $1 AND dismissed_at IS NULL
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
		LIMIT $1
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
		"UPDATE notes SET dismissed_at = $1 WHERE id = $2",
		time.Now().Unix(), id,
	)
	return err
}

// ---- Users ----

type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

func (s *Store) CreateUser(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = s.db.Exec(
		"INSERT INTO users (username, password_hash) VALUES ($1, $2)",
		username, string(hash),
	)
	return err
}

// AuthenticateUser verifies credentials and returns the user, or nil if invalid.
func (s *Store) AuthenticateUser(username, password string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		"SELECT id, username, password_hash FROM users WHERE username = $1", username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, nil
	}
	return &u, nil
}

// SeedAdminIfEmpty creates the given user only when the users table is empty.
// Safe to call on every startup — no-ops once any user exists.
func (s *Store) SeedAdminIfEmpty(username, password string) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.CreateUser(username, password)
}

// ---- Sessions ----

const sessionTTL = 30 * 24 * time.Hour

func (s *Store) CreateSession(userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(sessionTTL).Unix()
	_, err := s.db.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)",
		token, userID, expiresAt,
	)
	return token, err
}

func (s *Store) ValidateSession(token string) bool {
	var expiresAt int64
	err := s.db.QueryRow("SELECT expires_at FROM sessions WHERE token = $1", token).Scan(&expiresAt)
	return err == nil && time.Now().Unix() < expiresAt
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = $1", token)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}
