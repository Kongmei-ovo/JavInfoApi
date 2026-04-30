package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func listActresses(c *gin.Context) {
	q := c.Query("q")
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

	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if q != "" {
		whereClause += fmt.Sprintf(" AND (name_romaji ILIKE $%d OR name_kanji ILIKE $%d OR name_kana ILIKE $%d)", argIndex, argIndex+1, argIndex+2)
		searchPattern := "%" + q + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
		argIndex += 3
	}

	var totalCount int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_actress "+whereClause, args...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	args = append(args, pageSize, offset)
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT a.id, a.name_romaji, a.name_kanji, a.name_kana, a.image_url,
			   (SELECT COUNT(*) FROM derived_video_actress va WHERE va.actress_id = a.id) as movie_count
		FROM derived_actress a
		%s
		ORDER BY movie_count DESC, a.name_romaji
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1), args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	actresses := []ActressWithCount{}
	for rows.Next() {
		var a ActressWithCount
		if err := rows.Scan(&a.ID, &a.NameRomaji, &a.NameKanji, &a.NameKana, &a.ImageURL, &a.MovieCount); err == nil {
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
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var a Actress
	err = pool.QueryRow(ctx, `
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
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return
	}

	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	serviceCode := c.Query("service_code")
	year := getQueryInt(c, "year", 0)
	makerID := getQueryInt(c, "maker_id", 0)
	makerName := c.Query("maker_name")
	categoryID := getQueryInt(c, "category_id", 0)
	categoryName := c.Query("category_name")
	sortBy := c.Query("sort_by")

	if pageSize > 100 {
		pageSize = 100
	}
	if page < 1 {
		page = 1
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

	if year > 0 {
		whereClause += fmt.Sprintf(" AND EXTRACT(YEAR FROM v.release_date) = $%d", argIndex)
		args = append(args, year)
		argIndex++
	}

	// Use helper for maker name resolution
	argIndex = applyNameFilter(ctx, "derived_maker", "v.maker_id", makerID, makerName,
		[]string{"name_en", "name_ja"}, &whereClause, &args, argIndex)

	argIndex = applyCategoryFilter(ctx, categoryID, categoryName, &whereClause, &args, argIndex)

	// Count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM derived_video_actress va JOIN derived_video v ON va.content_id = v.content_id " + whereClause
	err = pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Sort
	orderClause := parseSortClause(sortBy, "v.")

	args = append(args, pageSize, offset)
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT v.content_id, v.dvd_id, v.title_en, v.title_ja, v.runtime_mins, v.release_date,
			   v.jacket_thumb_url, v.site_id, v.service_code
		FROM derived_video v
		JOIN derived_video_actress va ON v.content_id = va.content_id
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderClause, argIndex, argIndex+1), args...)
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

func batchActressVideos(c *gin.Context) {
	var req BatchActressVideosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids is required and must not be empty"})
		return
	}
	if len(req.IDs) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 20 actress ids per request"})
		return
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	type ActressVideosResult struct {
		TotalCount int     `json:"total_count"`
		Videos     []Video `json:"videos"`
	}

	result := make(map[string]ActressVideosResult)

	// Use goroutines for parallelism
	type resultEntry struct {
		key   string
		value ActressVideosResult
	}

	ch := make(chan resultEntry, len(req.IDs))
	var wg sync.WaitGroup

	for _, actressID := range req.IDs {
		wg.Add(1)
		go func(aid int) {
			defer wg.Done()
			var totalCount int
			err := pool.QueryRow(ctx,
				"SELECT COUNT(*) FROM derived_video_actress WHERE actress_id = $1", aid,
			).Scan(&totalCount)
			if err != nil {
				return
			}

			rows, err := pool.Query(ctx, `
				SELECT v.content_id, v.dvd_id, v.title_en, v.title_ja, v.runtime_mins, v.release_date,
					   v.jacket_thumb_url, v.site_id, v.service_code
				FROM derived_video v
				JOIN derived_video_actress va ON v.content_id = va.content_id
				WHERE va.actress_id = $1
				ORDER BY v.release_date DESC NULLS LAST
				LIMIT $2 OFFSET $3
			`, aid, pageSize, offset)
			if err != nil {
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

			ch <- resultEntry{
				key: strconv.Itoa(aid),
				value: ActressVideosResult{
					TotalCount: totalCount,
					Videos:     videos,
				},
			}
		}(actressID)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for entry := range ch {
		result[entry.key] = entry.value
	}

	c.JSON(http.StatusOK, result)
}
