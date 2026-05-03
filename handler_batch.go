package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func batchGetVideos(c *gin.Context) {
	var req BatchIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids is required and must not be empty"})
		return
	}
	if len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 100 ids per request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	placeholders := make([]string, len(req.IDs))
	args := make([]interface{}, len(req.IDs))
	for i, id := range req.IDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT v.content_id, v.dvd_id, v.title_en, v.title_ja, v.comment_en, v.comment_ja,
			   v.runtime_mins, v.release_date, COALESCE(v.sample_url, t.url) as sample_url,
			   v.maker_id, v.label_id, v.series_id,
			   v.jacket_full_url, v.jacket_thumb_url, v.gallery_thumb_first, v.gallery_thumb_last,
			   v.site_id, v.service_code
		FROM derived_video v
		LEFT JOIN source_dmm_trailer t ON v.content_id = t.content_id
		WHERE v.content_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	videos := []Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			continue
		}
		videos = append(videos, v)
	}

	// Batch load related data (much more efficient than per-video)
	loadRelatedDataBatch(ctx, videos)

	c.JSON(http.StatusOK, videos)
}

func batchLookupVideos(c *gin.Context) {
	var req BatchDvdIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.DvdIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dvd_ids is required and must not be empty"})
		return
	}
	if len(req.DvdIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 100 dvd_ids per request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Normalize dvd_ids: lowercase, strip hyphens for matching
	normalizedIDs := make([]string, len(req.DvdIDs))
	for i, id := range req.DvdIDs {
		normalizedIDs[i] = strings.ToLower(strings.ReplaceAll(id, "-", ""))
	}

	placeholders := make([]string, len(normalizedIDs))
	args := make([]interface{}, len(normalizedIDs))
	for i, id := range normalizedIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT v.content_id, v.dvd_id, v.title_en, v.title_ja, v.comment_en, v.comment_ja,
			   v.runtime_mins, v.release_date, COALESCE(v.sample_url, t.url) as sample_url,
			   v.maker_id, v.label_id, v.series_id,
			   v.jacket_full_url, v.jacket_thumb_url, v.gallery_thumb_first, v.gallery_thumb_last,
			   v.site_id, v.service_code
		FROM derived_video v
		LEFT JOIN source_dmm_trailer t ON v.content_id = t.content_id
		WHERE LOWER(REPLACE(v.dvd_id, '-', '')) IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	videos := []Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			continue
		}
		videos = append(videos, v)
	}

	// Batch load related data
	loadRelatedDataBatch(ctx, videos)

	// Find which normalized IDs were not matched by dvd_id
	matchedNorms := make(map[string]bool)
	for _, v := range videos {
		if v.DvdID != nil {
			matchedNorms[strings.ToLower(strings.ReplaceAll(*v.DvdID, "-", ""))] = true
		}
	}

	// Collect unmatched IDs and try content_id fallback
	var unmatched []string
	for _, id := range req.DvdIDs {
		norm := strings.ToLower(strings.ReplaceAll(id, "-", ""))
		if !matchedNorms[norm] {
			matches := dvdCodeRegex.FindStringSubmatch(id)
			if len(matches) >= 3 && matches[1] != "" && matches[2] != "" {
				fallback := strings.ToLower(matches[1] + matches[2])
				unmatched = append(unmatched, fallback)
			}
		}
	}

	// Query content_id for unmatched
	if len(unmatched) > 0 {
		placeholders2 := make([]string, len(unmatched))
		args2 := make([]interface{}, len(unmatched))
		for i, id := range unmatched {
			placeholders2[i] = fmt.Sprintf("$%d", i+1)
			args2[i] = id
		}

		fallbackQuery := fmt.Sprintf(`
			SELECT v.content_id, v.dvd_id, v.title_en, v.title_ja, v.comment_en, v.comment_ja,
				   v.runtime_mins, v.release_date, COALESCE(v.sample_url, t.url) as sample_url,
				   v.maker_id, v.label_id, v.series_id,
				   v.jacket_full_url, v.jacket_thumb_url, v.gallery_thumb_first, v.gallery_thumb_last,
				   v.site_id, v.service_code
			FROM derived_video v
			LEFT JOIN source_dmm_trailer t ON v.content_id = t.content_id
			WHERE v.content_id IN (%s)
		`, strings.Join(placeholders2, ","))

		rows2, err := pool.Query(ctx, fallbackQuery, args2...)
		if err == nil {
			defer rows2.Close()
			for rows2.Next() {
				v, err := scanVideo(rows2)
				if err != nil {
					continue
				}
				videos = append(videos, v)
			}
		}
	}

	// Build result map keyed by normalized dvd_id for easy lookup
	result := make(map[string]Video)
	// Also build a normalized->original mapping
	normalizedToOriginal := make(map[string]string)
	for _, id := range req.DvdIDs {
		normalizedToOriginal[strings.ToLower(strings.ReplaceAll(id, "-", ""))] = id
	}

	for _, v := range videos {
		if v.DvdID != nil {
			normalizedKey := strings.ToLower(strings.ReplaceAll(*v.DvdID, "-", ""))
			// Use the original dvd_id from the database as the key
			result[*v.DvdID] = v
			_ = normalizedKey
		}
	}

	c.JSON(http.StatusOK, result)
}
