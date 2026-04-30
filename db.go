package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all configuration
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBMaxConn  int32
	DBMinConn  int32
	ServerHost string
	ServerPort string
}

func loadConfig() Config {
	return Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "kongmei"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "r18"),
		DBMaxConn:  getEnvInt("DB_MAX_CONN", 20),
		DBMinConn:  getEnvInt("DB_MIN_CONN", 5),
		ServerHost: getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int32) int32 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return int32(i)
		}
	}
	return defaultValue
}

func initDB(cfg Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?pool_max_conns=%d&pool_min_conns=%d",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBMaxConn, cfg.DBMinConn)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 10 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")
	return pool, nil
}

func healthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func scanVideo(row pgx.Row) (Video, error) {
	var v Video
	var releaseDate interface{}

	err := row.Scan(
		&v.ContentID, &v.DvdID, &v.TitleEn, &v.TitleJa, &v.CommentEn, &v.CommentJa,
		&v.RuntimeMins, &releaseDate, &v.SampleURL, &v.MakerID, &v.LabelID, &v.SeriesID,
		&v.JacketFullURL, &v.JacketThumbURL, &v.GalleryFirst, &v.GalleryLast,
		&v.SiteID, &v.ServiceCode,
	)
	if err != nil {
		return v, err
	}

	if releaseDate != nil {
		if td, ok := releaseDate.(time.Time); ok {
			s := td.Format("2006-01-02")
			v.ReleaseDate = &s
		}
	}

	return v, nil
}

func scanVideoRow(row pgx.Row) (Video, error) {
	var v Video
	var releaseDate interface{}

	err := row.Scan(
		&v.ContentID, &v.DvdID, &v.TitleEn, &v.TitleJa, &v.RuntimeMins, &releaseDate,
		&v.JacketThumbURL, &v.SiteID, &v.ServiceCode,
	)
	if err != nil {
		return v, err
	}

	if releaseDate != nil {
		if td, ok := releaseDate.(time.Time); ok {
			s := td.Format("2006-01-02")
			v.ReleaseDate = &s
		}
	}

	return v, nil
}
