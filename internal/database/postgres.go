package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"wallpaper-generator/internal/models"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// DB wraps the standard *sql.DB connection pool
type DB struct {
	*sql.DB
}

// the Connect function initializes and checks the PostgreSQL connection pool
func Connect(connStr string) (*DB, error) {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("invalid connection string: %w", err)
	}
	// Configure connection pooling
	db.SetMaxOpenConns(10)
	// keeps idle connections in memory as idle. so if the more connections are required then these 5 will be instantely pre made
	// since making the new connections is not fast these idle connections will be available instead of making new on the go.
	// the connections are not limited to 5 here as they will be built again in background if someone used them so they always will be
	// at given size of pool which here is 5.
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	return &DB{db}, nil
}

// InsertWallpaper stores a new wallpaper record
// "ON CONFLICT (image_url) DO NOTHING" prevents duplicates without throwing error
func (db *DB) InsertWallpaper(ctx context.Context, imageURL, source, caption string) error {
	query := `
		INSERT INTO wallpapers (image_url, source, caption)
		VALUES ($1, $2, $3)
		ON CONFLICT (image_url) DO NOTHING;
	`
	_, err := db.ExecContext(ctx, query, imageURL, source, caption)
	if err != nil {
		return fmt.Errorf(":red_circle: failed to insert wallpaper: %w", err)
	}
	return nil
}

// GetNextUnserved retrieves the oldest unserved wallpaper, or falls back to recycling the oldest served wallpaper if the queue is empty
func (db *DB) GetNextUnserved(ctx context.Context) (*models.Wallpaper, error) {
	// 1. Try to fetch an unserved wallpaper
	queryUnserved := `
		UPDATE wallpapers
		SET is_served = TRUE, served_at = NOW()
		WHERE id = (
			SELECT id FROM wallpapers
			WHERE is_served = FALSE
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, image_url, source, caption, is_served, served_at, created_at;
	`

	var wp models.Wallpaper
	// QueryRowContext will return only one row it finds or error of ErrNoRows if none were there
	err := db.QueryRowContext(ctx, queryUnserved).Scan(
		&wp.ID,
		&wp.ImageURL,
		&wp.Source,
		&wp.Caption,
		&wp.IsServed,
		&wp.ServedAt,
		&wp.CreatedAt,
	)

	if err == nil {
		return &wp, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to fetch unserved wallpaper: %w", err)
	}

	// 2. Fallback: If all are served, recycle by picking the one least recently served
	queryRecycle := `
		UPDATE wallpapers
		SET served_at = NOW()
		WHERE id = (
			SELECT id FROM wallpapers
			ORDER BY served_at ASC NULLS FIRST
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, image_url, source, caption, is_served, served_at, created_at;
	`

	err = db.QueryRowContext(ctx, queryRecycle).Scan(
		&wp.ID,
		&wp.ImageURL,
		&wp.Source,
		&wp.Caption,
		&wp.IsServed,
		&wp.ServedAt,
		&wp.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Table is completely empty
	}
	if err != nil {
		return nil, fmt.Errorf("failed to recycle wallpaper: %w", err)
	}

	return &wp, nil
}
