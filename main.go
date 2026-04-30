package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

func main() {
	cfg := loadConfig()

	var err error
	pool, err = initDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/health", healthCheck)

	// Video endpoints
	r.GET("/api/v1/videos", listVideos)
	r.GET("/api/v1/videos/search", searchVideos)
	r.GET("/api/v1/videos/:content_id", getVideo)
	r.POST("/api/v1/videos/batch", batchGetVideos)
	r.POST("/api/v1/videos/lookup", batchLookupVideos)

	// Actress endpoints
	r.GET("/api/v1/actresses", listActresses)
	r.GET("/api/v1/actresses/:id", getActress)
	r.GET("/api/v1/actresses/:id/videos", getActressVideos)
	r.POST("/api/v1/actresses/batch_videos", batchActressVideos)

	// Auxiliary data endpoints
	r.GET("/api/v1/makers", listMakers)
	r.GET("/api/v1/labels", listLabels)
	r.GET("/api/v1/series", listSeries)
	r.GET("/api/v1/categories", listCategories)
	r.GET("/api/v1/categories/stats", getCategoryStats)

	// Stats
	r.GET("/api/v1/stats", getStats)

	addr := cfg.ServerHost + ":" + cfg.ServerPort

	// Graceful shutdown
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
