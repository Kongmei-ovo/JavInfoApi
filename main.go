package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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

// Video represents the main video record
type Video struct {
	ContentID      string    `json:"content_id"`
	DvdID          *string   `json:"dvd_id,omitempty"`
	TitleEn        *string   `json:"title_en,omitempty"`
	TitleJa        *string   `json:"title_ja,omitempty"`
	CommentEn      *string   `json:"comment_en,omitempty"`
	CommentJa      *string   `json:"comment_ja,omitempty"`
	RuntimeMins    *int      `json:"runtime_mins,omitempty"`
	ReleaseDate    *string   `json:"release_date,omitempty"`
	SampleURL      *string   `json:"sample_url,omitempty"`
	MakerID        *int      `json:"maker_id,omitempty"`
	LabelID        *int      `json:"label_id,omitempty"`
	SeriesID       *int      `json:"series_id,omitempty"`
	JacketFullURL  *string   `json:"jacket_full_url,omitempty"`
	JacketThumbURL *string   `json:"jacket_thumb_url,omitempty"`
	GalleryFirst   *string   `json:"gallery_thumb_first,omitempty"`
	GalleryLast    *string   `json:"gallery_thumb_last,omitempty"`
	SiteID         int       `json:"site_id"`
	ServiceCode    string    `json:"service_code"`
	Maker          *Maker    `json:"maker,omitempty"`
	Label          *Label    `json:"label,omitempty"`
	Series         *Series   `json:"series,omitempty"`
	Actresses      []Actress `json:"actresses,omitempty"`
	Categories     []Category `json:"categories,omitempty"`
}

// Actress represents an actress
type Actress struct {
	ID         int    `json:"id"`
	NameRomaji *string `json:"name_romaji,omitempty"`
	NameKanji  *string `json:"name_kanji,omitempty"`
	NameKana   *string `json:"name_kana,omitempty"`
	ImageURL   *string `json:"image_url,omitempty"`
}

// Maker represents a maker
type Maker struct {
	ID     int    `json:"id"`
	NameEn *string `json:"name_en,omitempty"`
	NameJa *string `json:"name_ja,omitempty"`
}

// Label represents a label
type Label struct {
	ID     int    `json:"id"`
	NameEn *string `json:"name_en,omitempty"`
	NameJa *string `json:"name_ja,omitempty"`
}

// Series represents a series
type Series struct {
	ID     int    `json:"id"`
	NameEn *string `json:"name_en,omitempty"`
	NameJa *string `json:"name_ja,omitempty"`
}

// Category represents a category
type Category struct {
	ID     int    `json:"id"`
	NameEn string `json:"name_en"`
	NameJa *string `json:"name_ja,omitempty"`
}

// CategoryWithCount represents a category with its video count
type CategoryWithCount struct {
	ID         int    `json:"id"`
	NameEn     string `json:"name_en"`
	NameJa     *string `json:"name_ja,omitempty"`
	VideoCount int64  `json:"video_count"`
}

// PaginatedResponse is a generic paginated response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalCount int         `json:"total_count"`
	TotalPages int         `json:"total_pages"`
}

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

	r.GET("/health", healthCheck)

	r.GET("/api/v1/videos", listVideos)
	r.GET("/api/v1/videos/search", searchVideos)
	r.GET("/api/v1/videos/:content_id", getVideo)

	r.GET("/api/v1/actresses", listActresses)
	r.GET("/api/v1/actresses/:id", getActress)
	r.GET("/api/v1/actresses/:id/videos", getActressVideos)

	r.GET("/api/v1/makers", listMakers)
	r.GET("/api/v1/labels", listLabels)
	r.GET("/api/v1/series", listSeries)
	r.GET("/api/v1/categories", listCategories)
	r.GET("/api/v1/categories/stats", getCategoryStats)

	r.GET("/api/v1/stats", getStats)

	addr := cfg.ServerHost + ":" + cfg.ServerPort
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
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

func listVideos(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var totalCount int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_video").Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, err := pool.Query(ctx, `
		SELECT content_id, dvd_id, title_en, title_ja, runtime_mins, release_date,
			   jacket_thumb_url, site_id, service_code
		FROM derived_video
		ORDER BY release_date DESC NULLS LAST, content_id DESC
		LIMIT $1 OFFSET $2
	`, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	videos := []Video{}
	for rows.Next() {
		v, err := scanVideoRow(rows)
		if err != nil {
			continue
		}
		videos = append(videos, v)
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       videos,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: (totalCount + pageSize - 1) / pageSize,
	})
}

func getVideo(c *gin.Context) {
	contentID := c.Param("content_id")
	serviceCode := c.Query("service_code")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	query := `
		SELECT v.content_id, v.dvd_id, v.title_en, v.title_ja, v.comment_en, v.comment_ja,
			   v.runtime_mins, v.release_date, COALESCE(v.sample_url, t.url) as sample_url,
			   v.maker_id, v.label_id, v.series_id,
			   v.jacket_full_url, v.jacket_thumb_url, v.gallery_thumb_first, v.gallery_thumb_last,
			   v.site_id, v.service_code
		FROM derived_video v
		LEFT JOIN source_dmm_trailer t ON v.content_id = t.content_id
		WHERE v.content_id = $1`
	args := []interface{}{contentID}

	if serviceCode != "" {
		query += " AND service_code = $2"
		args = append(args, serviceCode)
	}

	var video Video
	var releaseDate interface{}

	err := pool.QueryRow(ctx, query, args...).Scan(
		&video.ContentID, &video.DvdID, &video.TitleEn, &video.TitleJa,
		&video.CommentEn, &video.CommentJa, &video.RuntimeMins, &releaseDate,
		&video.SampleURL, &video.MakerID, &video.LabelID, &video.SeriesID,
		&video.JacketFullURL, &video.JacketThumbURL, &video.GalleryFirst, &video.GalleryLast,
		&video.SiteID, &video.ServiceCode,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if releaseDate != nil {
		if td, ok := releaseDate.(time.Time); ok {
			s := td.Format("2006-01-02")
			video.ReleaseDate = &s
		}
	}

	loadRelatedData(ctx, &video)

	c.JSON(http.StatusOK, video)
}

func searchVideos(c *gin.Context) {
	query := c.Query("q")
	contentID := c.Query("content_id")
	dvdID := c.Query("dvd_id")
	makerID := getQueryInt(c, "maker_id", 0)
	makerName := c.Query("maker_name")
	seriesID := getQueryInt(c, "series_id", 0)
	seriesName := c.Query("series_name")
	actressID := getQueryInt(c, "actress_id", 0)
	actressName := c.Query("actress_name")
	categoryID := getQueryInt(c, "category_id", 0)
	categoryName := c.Query("category_name")
	year := getQueryInt(c, "year", 0)
	serviceCode := c.Query("service_code")
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	sortBy := c.Query("sort_by")
	random := c.Query("random")

	if pageSize > 100 {
		pageSize = 100
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if query != "" {
		whereClause += fmt.Sprintf(" AND (title_en ILIKE $%d OR title_ja ILIKE $%d OR comment_en ILIKE $%d)", argIndex, argIndex+1, argIndex+2)
		searchPattern := "%" + query + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
		argIndex += 3
	}

	if contentID != "" {
		whereClause += fmt.Sprintf(" AND content_id = $%d", argIndex)
		args = append(args, contentID)
		argIndex++
	}

	if dvdID != "" {
		cleanDvdID := strings.ToLower(strings.ReplaceAll(dvdID, "-", ""))
		whereClause += fmt.Sprintf(" AND LOWER(REPLACE(dvd_id, '-', '')) = $%d", argIndex)
		args = append(args, cleanDvdID)
		argIndex++
	}

	if makerID > 0 {
		whereClause += fmt.Sprintf(" AND maker_id = $%d", argIndex)
		args = append(args, makerID)
		argIndex++
	} else if makerName != "" {
		// Resolve maker_name to maker_id(s)
		rows, err := pool.Query(ctx, "SELECT id FROM derived_maker WHERE name_en ILIKE $1 OR name_ja ILIKE $1", "%"+makerName+"%")
		if err == nil {
			defer rows.Close()
			makerIDs := []int{}
			for rows.Next() {
				var id int
				if rows.Scan(&id) == nil {
					makerIDs = append(makerIDs, id)
				}
			}
			if len(makerIDs) > 0 {
				placeholders := make([]string, len(makerIDs))
				for i, id := range makerIDs {
					args = append(args, id)
					placeholders[i] = fmt.Sprintf("$%d", argIndex)
					argIndex++
				}
				whereClause += fmt.Sprintf(" AND maker_id IN (%s)", strings.Join(placeholders, ","))
			}
		}
	}

	if seriesID > 0 {
		whereClause += fmt.Sprintf(" AND series_id = $%d", argIndex)
		args = append(args, seriesID)
		argIndex++
	} else if seriesName != "" {
		// Resolve series_name to series_id(s)
		rows, err := pool.Query(ctx, "SELECT id FROM derived_series WHERE name_en ILIKE $1 OR name_ja ILIKE $1", "%"+seriesName+"%")
		if err == nil {
			defer rows.Close()
			seriesIDs := []int{}
			for rows.Next() {
				var id int
				if rows.Scan(&id) == nil {
					seriesIDs = append(seriesIDs, id)
				}
			}
			if len(seriesIDs) > 0 {
				placeholders := make([]string, len(seriesIDs))
				for i, id := range seriesIDs {
					args = append(args, id)
					placeholders[i] = fmt.Sprintf("$%d", argIndex)
					argIndex++
				}
				whereClause += fmt.Sprintf(" AND series_id IN (%s)", strings.Join(placeholders, ","))
			}
		}
	}

	if actressID > 0 {
		whereClause += fmt.Sprintf(" AND content_id IN (SELECT content_id FROM derived_video_actress WHERE actress_id = $%d)", argIndex)
		args = append(args, actressID)
		argIndex++
	} else if actressName != "" {
		// Resolve actress_name to actress_id(s)
		rows, err := pool.Query(ctx, "SELECT id FROM derived_actress WHERE name_romaji ILIKE $1 OR name_kanji ILIKE $1 OR name_kana ILIKE $1", "%"+actressName+"%")
		if err == nil {
			defer rows.Close()
			actressIDs := []int{}
			for rows.Next() {
				var id int
				if rows.Scan(&id) == nil {
					actressIDs = append(actressIDs, id)
				}
			}
			if len(actressIDs) > 0 {
				placeholders := make([]string, len(actressIDs))
				for i, id := range actressIDs {
					args = append(args, id)
					placeholders[i] = fmt.Sprintf("$%d", argIndex)
					argIndex++
				}
				whereClause += fmt.Sprintf(" AND content_id IN (SELECT content_id FROM derived_video_actress WHERE actress_id IN (%s))", strings.Join(placeholders, ","))
			}
		}
	}

	if categoryID > 0 {
		whereClause += fmt.Sprintf(" AND content_id IN (SELECT content_id FROM derived_video_category WHERE category_id = $%d)", argIndex)
		args = append(args, categoryID)
		argIndex++
	} else if categoryName != "" {
		// Resolve category_name to category_id(s)
		rows, err := pool.Query(ctx, "SELECT id FROM derived_category WHERE name_en ILIKE $1 OR name_ja ILIKE $1", "%"+categoryName+"%")
		if err == nil {
			defer rows.Close()
			catIDs := []int{}
			for rows.Next() {
				var id int
				if rows.Scan(&id) == nil {
					catIDs = append(catIDs, id)
				}
			}
			if len(catIDs) > 0 {
				placeholders := make([]string, len(catIDs))
				for i, id := range catIDs {
					args = append(args, id)
					placeholders[i] = fmt.Sprintf("$%d", argIndex)
					argIndex++
				}
				whereClause += fmt.Sprintf(" AND content_id IN (SELECT content_id FROM derived_video_category WHERE category_id IN (%s))", strings.Join(placeholders, ","))
			}
		}
	}

	if year > 0 {
		whereClause += fmt.Sprintf(" AND EXTRACT(YEAR FROM release_date) = $%d", argIndex)
		args = append(args, year)
		argIndex++
	}

	if serviceCode != "" {
		whereClause += fmt.Sprintf(" AND service_code = $%d", argIndex)
		args = append(args, serviceCode)
		argIndex++
	}

	var totalCount int
	countQuery := "SELECT COUNT(*) FROM derived_video " + whereClause
	err := pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 构建排序
	var orderClause string
	if random == "1" {
		orderClause = "ORDER BY RANDOM()"
	} else if sortBy != "" {
		validSortFields := map[string]bool{
			"release_date": true,
			"content_id":   true,
			"dvd_id":      true,
			"title_en":    true,
			"title_ja":    true,
			"runtime_mins": true,
		}
		nullsLastFields := map[string]bool{
			"release_date": true,
			"runtime_mins": true,
		}
		// 解析格式: field1:asc,field2:desc,field3:asc
		var clauses []string
		sortParts := strings.Split(sortBy, ",")
		for _, part := range sortParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// 分离字段和方向
			field := part
			dir := "ASC"
			if idx := strings.LastIndex(part, ":"); idx > 0 {
				field = part[:idx]
				dirPart := strings.ToLower(part[idx+1:])
				if dirPart == "desc" {
					dir = "DESC"
				} else {
					dir = "ASC"
				}
			}
			if validSortFields[field] {
				if nullsLastFields[field] {
					clauses = append(clauses, fmt.Sprintf("%s %s NULLS LAST", field, dir))
				} else {
					clauses = append(clauses, fmt.Sprintf("%s %s", field, dir))
				}
			}
		}
		if len(clauses) > 0 {
			orderClause = "ORDER BY " + strings.Join(clauses, ", ")
		} else {
			orderClause = "ORDER BY release_date DESC NULLS LAST, content_id DESC"
		}
	} else {
		orderClause = "ORDER BY release_date DESC NULLS LAST, content_id DESC"
	}

	selectQuery := fmt.Sprintf(`
		SELECT content_id, dvd_id, title_en, title_ja, runtime_mins, release_date,
			   jacket_thumb_url, site_id, service_code
		FROM derived_video
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderClause, argIndex, argIndex+1)
	args = append(args, pageSize, offset)

	rows, err := pool.Query(ctx, selectQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	videos := []Video{}
	for rows.Next() {
		v, err := scanVideoRow(rows)
		if err != nil {
			continue
		}
		videos = append(videos, v)
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       videos,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: (totalCount + pageSize - 1) / pageSize,
	})
}

func loadRelatedData(ctx context.Context, video *Video) {
	var wg sync.WaitGroup

	if video.MakerID != nil && *video.MakerID > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var m Maker
			err := pool.QueryRow(ctx, "SELECT id, name_en, name_ja FROM derived_maker WHERE id = $1", video.MakerID).Scan(&m.ID, &m.NameEn, &m.NameJa)
			if err == nil {
				video.Maker = &m
			}
		}()
	}

	if video.LabelID != nil && *video.LabelID > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var l Label
			err := pool.QueryRow(ctx, "SELECT id, name_en, name_ja FROM derived_label WHERE id = $1", video.LabelID).Scan(&l.ID, &l.NameEn, &l.NameJa)
			if err == nil {
				video.Label = &l
			}
		}()
	}

	if video.SeriesID != nil && *video.SeriesID > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var s Series
			err := pool.QueryRow(ctx, "SELECT id, name_en, name_ja FROM derived_series WHERE id = $1", video.SeriesID).Scan(&s.ID, &s.NameEn, &s.NameJa)
			if err == nil {
				video.Series = &s
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := pool.Query(ctx, `
			SELECT a.id, a.name_romaji, a.name_kanji, a.name_kana, a.image_url
			FROM derived_actress a
			JOIN derived_video_actress va ON a.id = va.actress_id
			WHERE va.content_id = $1
			ORDER BY va.ordinality
		`, video.ContentID)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var a Actress
			if err := rows.Scan(&a.ID, &a.NameRomaji, &a.NameKanji, &a.NameKana, &a.ImageURL); err == nil {
				video.Actresses = append(video.Actresses, a)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := pool.Query(ctx, `
			SELECT c.id, c.name_en, c.name_ja
			FROM derived_category c
			JOIN derived_video_category vc ON c.id = vc.category_id
			WHERE vc.content_id = $1
			ORDER BY c.name_en
		`, video.ContentID)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var cat Category
			if err := rows.Scan(&cat.ID, &cat.NameEn, &cat.NameJa); err == nil {
				video.Categories = append(video.Categories, cat)
			}
		}
	}()

	wg.Wait()
}

func listActresses(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 50)
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var totalCount int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_actress").Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, err := pool.Query(ctx, `
		SELECT id, name_romaji, name_kanji, name_kana, image_url
		FROM derived_actress
		ORDER BY name_romaji
		LIMIT $1 OFFSET $2
	`, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	actresses := []Actress{}
	for rows.Next() {
		var a Actress
		if err := rows.Scan(&a.ID, &a.NameRomaji, &a.NameKanji, &a.NameKana, &a.ImageURL); err == nil {
			actresses = append(actresses, a)
		}
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       actresses,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: (totalCount + pageSize - 1) / pageSize,
	})
}

func getActress(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var a Actress
	err := pool.QueryRow(ctx, `
		SELECT id, name_romaji, name_kanji, name_kana, image_url
		FROM derived_actress WHERE id = $1
	`, id).Scan(&a.ID, &a.NameRomaji, &a.NameKanji, &a.NameKana, &a.ImageURL)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "actress not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, a)
}

func getActressVideos(c *gin.Context) {
	id := c.Param("id")
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	serviceCode := c.Query("service_code")
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	whereClause := "WHERE va.actress_id = $1"
	args := []interface{}{id}
	argIndex := 2
	if serviceCode != "" {
		whereClause += fmt.Sprintf(" AND v.service_code = $%d", argIndex)
		args = append(args, serviceCode)
		argIndex++
	}

	var totalCount int
	countQuery := "SELECT COUNT(*) FROM derived_video_actress va JOIN derived_video v ON va.content_id = v.content_id " + whereClause
	err := pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	args = append(args, pageSize, offset)
	rows, err := pool.Query(ctx, `
		SELECT v.content_id, v.dvd_id, v.title_en, v.title_ja, v.runtime_mins, v.release_date,
			   v.jacket_thumb_url, v.site_id, v.service_code
		FROM derived_video v
		JOIN derived_video_actress va ON v.content_id = va.content_id
		`+whereClause+`
		ORDER BY v.release_date DESC NULLS LAST, v.content_id DESC
		LIMIT $`+fmt.Sprintf("%d", argIndex)+` OFFSET $`+fmt.Sprintf("%d", argIndex+1), args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	videos := []Video{}
	for rows.Next() {
		v, err := scanVideoRow(rows)
		if err != nil {
			continue
		}
		videos = append(videos, v)
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       videos,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: (totalCount + pageSize - 1) / pageSize,
	})
}

func listMakers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, "SELECT id, name_en, name_ja FROM derived_maker ORDER BY name_en")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	makers := []Maker{}
	for rows.Next() {
		var m Maker
		if err := rows.Scan(&m.ID, &m.NameEn, &m.NameJa); err == nil {
			makers = append(makers, m)
		}
	}
	c.JSON(http.StatusOK, makers)
}

func listLabels(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, "SELECT id, name_en, name_ja FROM derived_label ORDER BY name_en")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	labels := []Label{}
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.NameEn, &l.NameJa); err == nil {
			labels = append(labels, l)
		}
	}
	c.JSON(http.StatusOK, labels)
}

func listSeries(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, "SELECT id, name_en, name_ja FROM derived_series ORDER BY name_en")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	seriesList := []Series{}
	for rows.Next() {
		var s Series
		if err := rows.Scan(&s.ID, &s.NameEn, &s.NameJa); err == nil {
			seriesList = append(seriesList, s)
		}
	}
	c.JSON(http.StatusOK, seriesList)
}

func listCategories(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, "SELECT id, name_en, name_ja FROM derived_category ORDER BY name_en")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.NameEn, &cat.NameJa); err == nil {
			categories = append(categories, cat)
		}
	}
	c.JSON(http.StatusOK, categories)
}

func getCategoryStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT c.id, COALESCE(c.name_en, ''), c.name_ja, COUNT(vc.content_id) as video_count
		FROM derived_category c
		LEFT JOIN derived_video_category vc ON c.id = vc.category_id
		GROUP BY c.id, c.name_en, c.name_ja
		ORDER BY video_count DESC, c.name_en
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	categories := []CategoryWithCount{}
	for rows.Next() {
		var cat CategoryWithCount
		if err := rows.Scan(&cat.ID, &cat.NameEn, &cat.NameJa, &cat.VideoCount); err == nil {
			categories = append(categories, cat)
		}
	}
	c.JSON(http.StatusOK, categories)
}

func getStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var videoCount, actressCount, makerCount, seriesCount, labelCount int

	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_video").Scan(&videoCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_actress").Scan(&actressCount)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_maker").Scan(&makerCount)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_series").Scan(&seriesCount)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_label").Scan(&labelCount)

	c.JSON(http.StatusOK, gin.H{
		"videos":    videoCount,
		"actresses": actressCount,
		"makers":    makerCount,
		"series":    seriesCount,
		"labels":    labelCount,
	})
}

func getQueryInt(c *gin.Context, key string, defaultVal int) int {
	if val := c.Query(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
