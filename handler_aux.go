package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// Generic list handler for simple entities (makers, labels, series, categories).
// All follow the same pattern: optional q search, pagination, ORDER BY name_en.
func listSimpleEntity(c *gin.Context, table string, searchCols []string, scanFn func(pgx.Rows) (interface{}, error)) {
	listEntity(listEntityParams{
		ctx:         c.Request.Context(),
		c:           c,
		table:       table,
		searchCols:  searchCols,
		countQuery:  "SELECT COUNT(*) FROM " + table,
		selectQuery: "SELECT id, name_en, name_ja FROM " + table,
		defaultSort: "ORDER BY name_en",
		scanFn:      scanFn,
	})
}

func listMakers(c *gin.Context) {
	listSimpleEntity(c, "derived_maker", []string{"name_en", "name_ja"}, func(rows pgx.Rows) (interface{}, error) {
		var m Maker
		err := rows.Scan(&m.ID, &m.NameEn, &m.NameJa)
		return m, err
	})
}

func listLabels(c *gin.Context) {
	listSimpleEntity(c, "derived_label", []string{"name_en", "name_ja"}, func(rows pgx.Rows) (interface{}, error) {
		var l Label
		err := rows.Scan(&l.ID, &l.NameEn, &l.NameJa)
		return l, err
	})
}

func listSeries(c *gin.Context) {
	listSimpleEntity(c, "derived_series", []string{"name_en", "name_ja"}, func(rows pgx.Rows) (interface{}, error) {
		var s Series
		err := rows.Scan(&s.ID, &s.NameEn, &s.NameJa)
		return s, err
	})
}

func listCategories(c *gin.Context) {
	listSimpleEntity(c, "derived_category", []string{"name_en", "name_ja"}, func(rows pgx.Rows) (interface{}, error) {
		var cat Category
		err := rows.Scan(&cat.ID, &cat.NameEn, &cat.NameJa)
		return cat, err
	})
}

func listDirectors(c *gin.Context) {
	listEntity(listEntityParams{
		ctx:         c.Request.Context(),
		c:           c,
		table:       "derived_director",
		searchCols:  []string{"name_romaji", "name_kanji", "name_kana"},
		countQuery:  "SELECT COUNT(*) FROM derived_director",
		selectQuery: "SELECT id, name_romaji, name_kanji, name_kana FROM derived_director",
		defaultSort: "ORDER BY name_kanji",
		scanFn: func(rows pgx.Rows) (interface{}, error) {
			var d Director
			err := rows.Scan(&d.ID, &d.NameRomaji, &d.NameKanji, &d.NameKana)
			return d, err
		},
	})
}

func listActors(c *gin.Context) {
	listEntity(listEntityParams{
		ctx:         c.Request.Context(),
		c:           c,
		table:       "derived_actor",
		searchCols:  []string{"name_kanji", "name_kana"},
		countQuery:  "SELECT COUNT(*) FROM derived_actor",
		selectQuery: "SELECT id, name_kanji, name_kana FROM derived_actor",
		defaultSort: "ORDER BY name_kanji",
		scanFn: func(rows pgx.Rows) (interface{}, error) {
			var a Actor
			err := rows.Scan(&a.ID, &a.NameKanji, &a.NameKana)
			return a, err
		},
	})
}

func listAuthors(c *gin.Context) {
	listEntity(listEntityParams{
		ctx:         c.Request.Context(),
		c:           c,
		table:       "derived_author",
		searchCols:  []string{"name_kanji", "name_kana"},
		countQuery:  "SELECT COUNT(*) FROM derived_author",
		selectQuery: "SELECT id, name_kanji, name_kana FROM derived_author",
		defaultSort: "ORDER BY name_kanji",
		scanFn: func(rows pgx.Rows) (interface{}, error) {
			var a Author
			err := rows.Scan(&a.ID, &a.NameKanji, &a.NameKana)
			return a, err
		},
	})
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

	// Run counts in parallel
	type countResult struct {
		count int
	}
	ch := make(chan countResult, 5)

	go func() {
		var n int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_video").Scan(&n)
		ch <- countResult{n}
	}()
	go func() {
		var n int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_actress").Scan(&n)
		ch <- countResult{n}
	}()
	go func() {
		var n int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_maker").Scan(&n)
		ch <- countResult{n}
	}()
	go func() {
		var n int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_series").Scan(&n)
		ch <- countResult{n}
	}()
	go func() {
		var n int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM derived_label").Scan(&n)
		ch <- countResult{n}
	}()

	for i := 0; i < 5; i++ {
		r := <-ch
		switch i {
		case 0:
			videoCount = r.count
		case 1:
			actressCount = r.count
		case 2:
			makerCount = r.count
		case 3:
			seriesCount = r.count
		case 4:
			labelCount = r.count
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"videos":    videoCount,
		"actresses": actressCount,
		"makers":    makerCount,
		"series":    seriesCount,
		"labels":    labelCount,
	})
}
