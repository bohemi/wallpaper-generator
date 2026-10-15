package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"wallpaper-generator/internal/database"
	_"wallpaper-generator/internal/models"
)

type Server struct {
	db       *database.DB
	rdb      *database.RedisClient
	cacheTTL time.Duration
}

func NewServer(db *database.DB, rdb *database.RedisClient, cacheTTL time.Duration) *Server {
	return &Server{
		db:       db,
		rdb:      rdb,
		cacheTTL: cacheTTL,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealthCheck)
	mux.HandleFunc("GET /api/v1/wallpaper/active", s.handleGetActiveWallpaper)
	mux.HandleFunc("POST /api/v1/wallpaper", s.handleInsertWallpaper)
	mux.HandleFunc("POST /api/v1/wallpaper/batch", s.handleBatchInsert)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}

// GET /health
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// GET /api/v1/wallpaper/active
func (s *Server) handleGetActiveWallpaper(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cacheKey := "wallpaper:active"

	// 1. Check Redis Cache
	cached, err := s.rdb.GetActiveWallpaper(ctx, cacheKey)
	if err != nil {
		log.Printf("Redis read error: %v", err)
	}
	if cached != nil {
		w.Header().Set("X-Cache", "HIT")
		writeJSON(w, http.StatusOK, cached)
		return
	}

	// 2. Redis dont have the wallpaper. Fetch from DB
	wp, err := s.db.GetNextUnserved(ctx)
	if err != nil {
		http.Error(w, "internal database error", http.StatusInternalServerError)
		return
	}

	if wp == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no wallpapers available in database"})
		return
	}

	// 3. Cache into Redis
	if err := s.rdb.SetActiveWallpaper(ctx, cacheKey, wp, s.cacheTTL); err != nil {
		log.Printf("Failed to cache to Redis: %v", err)
	}

	w.Header().Set("X-Cache", "MISS")
	writeJSON(w, http.StatusOK, wp)
}

type singleInsertInput struct {
	ImageURL string `json:"image_url"`
	Source   string `json:"source"`
	Caption  string `json:"caption"`
}

// POST /api/v1/wallpaper
func (s *Server) handleInsertWallpaper(w http.ResponseWriter, r *http.Request) {
	var input singleInsertInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.ImageURL == "" {
		http.Error(w, "image_url is required", http.StatusBadRequest)
		return
	}
	if input.Source == "" {
		input.Source = "manual"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := s.db.InsertWallpaper(ctx, input.ImageURL, input.Source, input.Caption)
	if err != nil {
		http.Error(w, "failed to insert wallpaper", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"message": "wallpaper added successfully"})
}

type batchInsertInput struct {
	URLs    []string `json:"urls"`
	Source  string   `json:"source"`
	Caption string   `json:"caption"`
}

// POST /api/v1/wallpaper/batch
func (s *Server) handleBatchInsert(w http.ResponseWriter, r *http.Request) {
	var input batchInsertInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(input.URLs) == 0 {
		http.Error(w, "urls list cannot be empty", http.StatusBadRequest)
		return
	}
	if input.Source == "" {
		input.Source = "pinterest"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	insertedCount := 0
	for _, rawURL := range input.URLs {
		err := s.db.InsertWallpaper(ctx, rawURL, input.Source, input.Caption)
		if err != nil {
			log.Printf("failed to insert %s: %v", rawURL, err)
			continue
		}
		insertedCount++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"ingested": insertedCount,
		"total":    len(input.URLs),
	})
}