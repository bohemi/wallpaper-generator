package models

import (
	"database/sql"
	"time"
)

type Wallpaper struct {
	ID        int64          `json:"id"`
	ImageURL  string         `json:"image_url"`
	Source    string         `json:"source"`
	Caption   sql.NullString `json:"caption"` // Handles SQL NULL safely
	IsServed  bool           `json:"is_served"`
	ServedAt  sql.NullTime   `json:"served_at"` // Handles SQL NULL safely
	CreatedAt time.Time      `json:"created_at"`
}
