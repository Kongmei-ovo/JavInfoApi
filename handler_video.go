package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

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

	for i := range videos {
		enrichVideoLight(&videos[i])
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
	if contentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content_id is required"})
		return
	}

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

	enrichVideo(&video)

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
	labelID := getQueryInt(c, "label_id", 0)
	labelName := c.Query("label_name")
	siteID := getQueryInt(c, "site_id", 0)
	year := getQueryInt(c, "year", 0)
	yearFrom := getQueryInt(c, "year_from", 0)
	yearTo := getQueryInt(c, "year_to", 0)
	runtimeMin := getQueryInt(c, "runtime_min", 0)
	runtimeMax := getQueryInt(c, "runtime_max", 0)
	releaseDateFrom := c.Query("release_date_from")
	releaseDateTo := c.Query("release_date_to")
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

	// Validate date format
	if releaseDateFrom != "" && !isValidDate(releaseDateFrom) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "release_date_from must be YYYY-MM-DD format"})
		return
	}
	if releaseDateTo != "" && !isValidDate(releaseDateTo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "release_date_to must be YYYY-MM-DD format"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	// Full-text search
	if query != "" {
		whereClause += fmt.Sprintf(" AND (title_en ILIKE $%d OR title_ja ILIKE $%d OR comment_en ILIKE $%d OR comment_ja ILIKE $%d)",
			argIndex, argIndex+1, argIndex+2, argIndex+3)
		searchPattern := "%" + query + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
		argIndex += 4
	}

	// Exact match filters
	if contentID != "" {
		whereClause += fmt.Sprintf(" AND content_id = $%d", argIndex)
		args = append(args, contentID)
		argIndex++
	}

	if dvdID != "" {
		// Normalize: uppercase for dvd_id match
		normalizedDvdID := strings.ToUpper(dvdID)
		// Try dvd_id ILIKE match first
		whereClause += fmt.Sprintf(" AND (dvd_id ILIKE $%d", argIndex)
		args = append(args, "%"+normalizedDvdID+"%")
		argIndex++

		// Fallback: extract prefix-number, lowercase as content_id
		matches := dvdCodeRegex.FindStringSubmatch(dvdID)
		if len(matches) >= 3 && matches[1] != "" && matches[2] != "" {
			contentIDFallback := strings.ToLower(matches[1] + matches[2])
			whereClause += fmt.Sprintf(" OR content_id = $%d)", argIndex)
			args = append(args, contentIDFallback)
			argIndex++
		} else {
			whereClause += ")"
		}
	}

	// Name resolution filters (extracted to reduce duplication)
	argIndex = applyNameFilter(ctx, "derived_maker", "maker_id", makerID, makerName,
		[]string{"name_en", "name_ja"}, &whereClause, &args, argIndex)

	argIndex = applyNameFilter(ctx, "derived_series", "series_id", seriesID, seriesName,
		[]string{"name_en", "name_ja"}, &whereClause, &args, argIndex)

	argIndex = applyActressFilter(ctx, actressID, actressName, &whereClause, &args, argIndex)

	argIndex = applyCategoryFilter(ctx, categoryID, categoryName, &whereClause, &args, argIndex)

	argIndex = applyNameFilter(ctx, "derived_label", "label_id", labelID, labelName,
		[]string{"name_en", "name_ja"}, &whereClause, &args, argIndex)

	// Direct filters
	if siteID > 0 {
		whereClause += fmt.Sprintf(" AND site_id = $%d", argIndex)
		args = append(args, siteID)
		argIndex++
	}

	if year > 0 {
		whereClause += fmt.Sprintf(" AND EXTRACT(YEAR FROM release_date) = $%d", argIndex)
		args = append(args, year)
		argIndex++
	}
	if yearFrom > 0 {
		whereClause += fmt.Sprintf(" AND EXTRACT(YEAR FROM release_date) >= $%d", argIndex)
		args = append(args, yearFrom)
		argIndex++
	}
	if yearTo > 0 {
		whereClause += fmt.Sprintf(" AND EXTRACT(YEAR FROM release_date) <= $%d", argIndex)
		args = append(args, yearTo)
		argIndex++
	}

	if runtimeMin > 0 {
		whereClause += fmt.Sprintf(" AND runtime_mins >= $%d", argIndex)
		args = append(args, runtimeMin)
		argIndex++
	}
	if runtimeMax > 0 {
		whereClause += fmt.Sprintf(" AND runtime_mins <= $%d", argIndex)
		args = append(args, runtimeMax)
		argIndex++
	}

	if releaseDateFrom != "" {
		whereClause += fmt.Sprintf(" AND release_date >= $%d", argIndex)
		args = append(args, releaseDateFrom)
		argIndex++
	}
	if releaseDateTo != "" {
		whereClause += fmt.Sprintf(" AND release_date <= $%d", argIndex)
		args = append(args, releaseDateTo)
		argIndex++
	}

	if serviceCode != "" {
		whereClause += fmt.Sprintf(" AND service_code = $%d", argIndex)
		args = append(args, serviceCode)
		argIndex++
	}

	// Count
	var totalCount int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_video "+whereClause, args...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Sort
	var orderClause string
	if random == "1" {
		orderClause = "ORDER BY RANDOM()"
	} else {
		orderClause = parseSortClause(sortBy, "")
	}

	// Query
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

	for i := range videos {
		enrichVideoLight(&videos[i])
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       videos,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: (totalCount + pageSize - 1) / pageSize,
	})
}

// applyNameFilter resolves a *_name parameter to ID(s) and adds to WHERE clause.
// Handles both direct ID match and name-based resolution.
func applyNameFilter(ctx context.Context, table, idColumn string, directID int, name string,
	searchCols []string, whereClause *string, args *[]interface{}, argIndex int) int {

	if directID > 0 {
		*whereClause += fmt.Sprintf(" AND %s = $%d", idColumn, argIndex)
		*args = append(*args, directID)
		return argIndex + 1
	}

	if name == "" {
		return argIndex
	}

	ids, err := resolveIDs(ctx, table, name, searchCols...)
	if err != nil || len(ids) == 0 {
		return argIndex
	}

	clause := buildInClause("AND "+idColumn, ids, args, &argIndex)
	*whereClause += clause
	return argIndex
}

// applyActressFilter resolves actress name/id to a subquery filter.
func applyActressFilter(ctx context.Context, directID int, name string,
	whereClause *string, args *[]interface{}, argIndex int) int {

	if directID > 0 {
		*whereClause += fmt.Sprintf(" AND content_id IN (SELECT content_id FROM derived_video_actress WHERE actress_id = $%d)", argIndex)
		*args = append(*args, directID)
		return argIndex + 1
	}

	if name == "" {
		return argIndex
	}

	ids, err := resolveIDs(ctx, "derived_actress", name, "name_romaji", "name_kanji", "name_kana")
	if err != nil || len(ids) == 0 {
		return argIndex
	}

	clause := buildInClause("AND content_id IN (SELECT content_id FROM derived_video_actress WHERE actress_id", ids, args, &argIndex)
	*whereClause += clause + ")"
	return argIndex
}

// applyCategoryFilter resolves category name/id to a subquery filter.
func applyCategoryFilter(ctx context.Context, directID int, name string,
	whereClause *string, args *[]interface{}, argIndex int) int {

	if directID > 0 {
		*whereClause += fmt.Sprintf(" AND content_id IN (SELECT content_id FROM derived_video_category WHERE category_id = $%d)", argIndex)
		*args = append(*args, directID)
		return argIndex + 1
	}

	if name == "" {
		return argIndex
	}

	ids, err := resolveIDs(ctx, "derived_category", name, "name_en", "name_ja")
	if err != nil || len(ids) == 0 {
		return argIndex
	}

	clause := buildInClause("AND content_id IN (SELECT content_id FROM derived_video_category WHERE category_id", ids, args, &argIndex)
	*whereClause += clause + ")"
	return argIndex
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
			SELECT a.id, a.name_romaji, a.name_kanji, a.name_kana, a.image_url, va.ordinality
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
			if err := rows.Scan(&a.ID, &a.NameRomaji, &a.NameKanji, &a.NameKana, &a.ImageURL, &a.Ordinality); err == nil {
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

	// Load directors
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := pool.Query(ctx, `
			SELECT d.id, d.name_romaji, d.name_kanji, d.name_kana
			FROM derived_director d
			JOIN derived_video_director vd ON d.id = vd.director_id
			WHERE vd.content_id = $1
		`, video.ContentID)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var d Director
			if err := rows.Scan(&d.ID, &d.NameRomaji, &d.NameKanji, &d.NameKana); err == nil {
				video.Directors = append(video.Directors, d)
			}
		}
	}()

	// Load actors (male performers)
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := pool.Query(ctx, `
			SELECT a.id, a.name_kanji, a.name_kana
			FROM derived_actor a
			JOIN derived_video_actor va ON a.id = va.actor_id
			WHERE va.content_id = $1
			ORDER BY va.ordinality
		`, video.ContentID)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var a Actor
			if err := rows.Scan(&a.ID, &a.NameKanji, &a.NameKana); err == nil {
				video.Actors = append(video.Actors, a)
			}
		}
	}()

	// Load authors
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := pool.Query(ctx, `
			SELECT a.id, a.name_kanji, a.name_kana
			FROM derived_author a
			JOIN derived_video_author va ON a.id = va.author_id
			WHERE va.content_id = $1
		`, video.ContentID)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var a Author
			if err := rows.Scan(&a.ID, &a.NameKanji, &a.NameKana); err == nil {
				video.Authors = append(video.Authors, a)
			}
		}
	}()

	wg.Wait()

	// Supplement missing actress names via Wikidata
	if len(video.Actresses) > 0 {
		supplementActressNames(video.Actresses)
	}
}

// loadRelatedDataBatch loads related data for multiple videos efficiently.
func loadRelatedDataBatch(ctx context.Context, videos []Video) {
	if len(videos) == 0 {
		return
	}

	// Collect unique IDs
	makerIDs := make(map[int]bool)
	labelIDs := make(map[int]bool)
	seriesIDs := make(map[int]bool)
	contentIDs := make([]string, len(videos))
	videoMap := make(map[string]*Video)

	for i := range videos {
		v := &videos[i]
		contentIDs[i] = v.ContentID
		videoMap[v.ContentID] = v
		if v.MakerID != nil && *v.MakerID > 0 {
			makerIDs[*v.MakerID] = true
		}
		if v.LabelID != nil && *v.LabelID > 0 {
			labelIDs[*v.LabelID] = true
		}
		if v.SeriesID != nil && *v.SeriesID > 0 {
			seriesIDs[*v.SeriesID] = true
		}
	}

	var wg sync.WaitGroup

	// Batch load makers
	if len(makerIDs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := mapKeys(makerIDs)
			placeholders, args := makePlaceholders(ids)
			rows, err := pool.Query(ctx,
				fmt.Sprintf("SELECT id, name_en, name_ja FROM derived_maker WHERE id IN (%s)", placeholders), args...)
			if err != nil {
				return
			}
			defer rows.Close()
			makerMap := make(map[int]Maker)
			for rows.Next() {
				var m Maker
				if rows.Scan(&m.ID, &m.NameEn, &m.NameJa) == nil {
					makerMap[m.ID] = m
				}
			}
			for _, v := range videos {
				if v.MakerID != nil {
					if m, ok := makerMap[*v.MakerID]; ok {
						v.Maker = &m
					}
				}
			}
		}()
	}

	// Batch load labels
	if len(labelIDs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := mapKeys(labelIDs)
			placeholders, args := makePlaceholders(ids)
			rows, err := pool.Query(ctx,
				fmt.Sprintf("SELECT id, name_en, name_ja FROM derived_label WHERE id IN (%s)", placeholders), args...)
			if err != nil {
				return
			}
			defer rows.Close()
			labelMap := make(map[int]Label)
			for rows.Next() {
				var l Label
				if rows.Scan(&l.ID, &l.NameEn, &l.NameJa) == nil {
					labelMap[l.ID] = l
				}
			}
			for _, v := range videos {
				if v.LabelID != nil {
					if l, ok := labelMap[*v.LabelID]; ok {
						v.Label = &l
					}
				}
			}
		}()
	}

	// Batch load series
	if len(seriesIDs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := mapKeys(seriesIDs)
			placeholders, args := makePlaceholders(ids)
			rows, err := pool.Query(ctx,
				fmt.Sprintf("SELECT id, name_en, name_ja FROM derived_series WHERE id IN (%s)", placeholders), args...)
			if err != nil {
				return
			}
			defer rows.Close()
			seriesMap := make(map[int]Series)
			for rows.Next() {
				var s Series
				if rows.Scan(&s.ID, &s.NameEn, &s.NameJa) == nil {
					seriesMap[s.ID] = s
				}
			}
			for _, v := range videos {
				if v.SeriesID != nil {
					if s, ok := seriesMap[*v.SeriesID]; ok {
						v.Series = &s
					}
				}
			}
		}()
	}

	// Batch load actresses
	wg.Add(1)
	go func() {
		defer wg.Done()
		placeholders, args := makePlaceholdersStr(contentIDs)
		rows, err := pool.Query(ctx, fmt.Sprintf(`
			SELECT va.content_id, a.id, a.name_romaji, a.name_kanji, a.name_kana, a.image_url, va.ordinality
			FROM derived_actress a
			JOIN derived_video_actress va ON a.id = va.actress_id
			WHERE va.content_id IN (%s)
			ORDER BY va.content_id, va.ordinality
		`, placeholders), args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var contentID string
			var a Actress
			if rows.Scan(&contentID, &a.ID, &a.NameRomaji, &a.NameKanji, &a.NameKana, &a.ImageURL, &a.Ordinality) == nil {
				if v, ok := videoMap[contentID]; ok {
					v.Actresses = append(v.Actresses, a)
				}
			}
		}
	}()

	// Batch load categories
	wg.Add(1)
	go func() {
		defer wg.Done()
		placeholders, args := makePlaceholdersStr(contentIDs)
		rows, err := pool.Query(ctx, fmt.Sprintf(`
			SELECT vc.content_id, c.id, c.name_en, c.name_ja
			FROM derived_category c
			JOIN derived_video_category vc ON c.id = vc.category_id
			WHERE vc.content_id IN (%s)
			ORDER BY vc.content_id, c.name_en
		`, placeholders), args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var contentID string
			var cat Category
			if rows.Scan(&contentID, &cat.ID, &cat.NameEn, &cat.NameJa) == nil {
				if v, ok := videoMap[contentID]; ok {
					v.Categories = append(v.Categories, cat)
				}
			}
		}
	}()

	// Batch load directors
	wg.Add(1)
	go func() {
		defer wg.Done()
		placeholders, args := makePlaceholdersStr(contentIDs)
		rows, err := pool.Query(ctx, fmt.Sprintf(`
			SELECT vd.content_id, d.id, d.name_romaji, d.name_kanji, d.name_kana
			FROM derived_director d
			JOIN derived_video_director vd ON d.id = vd.director_id
			WHERE vd.content_id IN (%s)
		`, placeholders), args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var contentID string
			var d Director
			if rows.Scan(&contentID, &d.ID, &d.NameRomaji, &d.NameKanji, &d.NameKana) == nil {
				if v, ok := videoMap[contentID]; ok {
					v.Directors = append(v.Directors, d)
				}
			}
		}
	}()

	// Batch load actors
	wg.Add(1)
	go func() {
		defer wg.Done()
		placeholders, args := makePlaceholdersStr(contentIDs)
		rows, err := pool.Query(ctx, fmt.Sprintf(`
			SELECT va.content_id, a.id, a.name_kanji, a.name_kana
			FROM derived_actor a
			JOIN derived_video_actor va ON a.id = va.actor_id
			WHERE va.content_id IN (%s)
			ORDER BY va.content_id, va.ordinality
		`, placeholders), args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var contentID string
			var a Actor
			if rows.Scan(&contentID, &a.ID, &a.NameKanji, &a.NameKana) == nil {
				if v, ok := videoMap[contentID]; ok {
					v.Actors = append(v.Actors, a)
				}
			}
		}
	}()

	// Batch load authors
	wg.Add(1)
	go func() {
		defer wg.Done()
		placeholders, args := makePlaceholdersStr(contentIDs)
		rows, err := pool.Query(ctx, fmt.Sprintf(`
			SELECT va.content_id, a.id, a.name_kanji, a.name_kana
			FROM derived_author a
			JOIN derived_video_author va ON a.id = va.author_id
			WHERE va.content_id IN (%s)
		`, placeholders), args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var contentID string
			var a Author
			if rows.Scan(&contentID, &a.ID, &a.NameKanji, &a.NameKana) == nil {
				if v, ok := videoMap[contentID]; ok {
					v.Authors = append(v.Authors, a)
				}
			}
		}
	}()

	wg.Wait()

	// Supplement missing actress names via Wikidata
	for _, v := range videos {
		if len(v.Actresses) > 0 {
			supplementActressNames(v.Actresses)
		}
	}
}

func mapKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func makePlaceholders(ids []int) (string, []interface{}) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	return strings.Join(placeholders, ","), args
}

func makePlaceholdersStr(ids []string) (string, []interface{}) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	return strings.Join(placeholders, ","), args
}

func isValidDate(s string) bool {
	if len(s) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
