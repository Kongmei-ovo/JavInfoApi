package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func getQueryInt(c *gin.Context, key string, defaultVal int) int {
	if val := c.Query(key); val != "" {
		if i, err := parsePositiveInt(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func parsePositiveInt(s string) (int, error) {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not a positive integer")
		}
		n = n*10 + int(ch-'0')
	}
	if n == 0 {
		return 0, fmt.Errorf("not a positive integer")
	}
	return n, nil
}

// resolveIDs looks up entity IDs by name using ILIKE on the given columns.
// Returns the matched IDs. Caller should use buildInClause to construct SQL.
func resolveIDs(ctx context.Context, table string, name string, columns ...string) ([]int, error) {
	if name == "" {
		return nil, nil
	}
	var conditions []string
	for _, col := range columns {
		conditions = append(conditions, col+" ILIKE $1")
	}
	query := fmt.Sprintf("SELECT id FROM %s WHERE %s LIMIT 1000", table, strings.Join(conditions, " OR "))

	rows, err := pool.Query(ctx, query, "%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// buildInClause builds an IN (...) clause fragment and appends args.
// Returns the clause fragment (e.g. " AND maker_id IN ($2,$3)") and the updated argIndex.
func buildInClause(prefix string, ids []int, args *[]interface{}, argIndex *int) string {
	if len(ids) == 0 {
		return ""
	}
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		*args = append(*args, id)
		placeholders[i] = fmt.Sprintf("$%d", *argIndex)
		*argIndex++
	}
	return fmt.Sprintf(" %s IN (%s)", prefix, strings.Join(placeholders, ","))
}

var validSortFields = map[string]bool{
	"release_date": true,
	"content_id":   true,
	"dvd_id":       true,
	"title_en":     true,
	"title_ja":     true,
	"runtime_mins": true,
}

var nullsLastFields = map[string]bool{
	"release_date": true,
	"runtime_mins": true,
}

// parseSortClause parses a sort_by parameter like "field1:asc,field2:desc" into an ORDER BY clause.
// tablePrefix is prepended to field names (e.g. "v." for joined queries).
func parseSortClause(sortBy string, tablePrefix string) string {
	if sortBy == "" {
		return "ORDER BY " + tablePrefix + "release_date DESC NULLS LAST, " + tablePrefix + "content_id DESC"
	}

	var clauses []string
	for _, part := range strings.Split(sortBy, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		field := part
		dir := "ASC"
		if idx := strings.LastIndex(part, ":"); idx > 0 {
			field = part[:idx]
			if strings.ToLower(part[idx+1:]) == "desc" {
				dir = "DESC"
			}
		}
		if !validSortFields[field] {
			continue
		}
		clause := tablePrefix + field + " " + dir
		if nullsLastFields[field] {
			clause += " NULLS LAST"
		}
		clauses = append(clauses, clause)
	}

	if len(clauses) == 0 {
		return "ORDER BY " + tablePrefix + "release_date DESC NULLS LAST, " + tablePrefix + "content_id DESC"
	}
	return "ORDER BY " + strings.Join(clauses, ", ")
}

// listEntityParams holds common parameters for paginated list endpoints.
type listEntityParams struct {
	ctx         context.Context
	c           *gin.Context
	table       string
	searchCols  []string // columns to ILIKE search on
	countQuery  string   // base count query, e.g. "SELECT COUNT(*) FROM derived_maker"
	selectQuery string   // base select without WHERE/LIMIT, e.g. "SELECT id, name_en, name_ja FROM derived_maker"
	defaultSort string   // e.g. "ORDER BY name_en"
	scanFn      func(pgx.Rows) (interface{}, error)
}

// listEntity is a generic paginated list handler for simple entities (makers, labels, series, categories).
func listEntity(p listEntityParams) {
	q := p.c.Query("q")
	page := getQueryInt(p.c, "page", 1)
	pageSize := getQueryInt(p.c, "page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if q != "" && len(p.searchCols) > 0 {
		var conditions []string
		for range p.searchCols {
			conditions = append(conditions, fmt.Sprintf("%s ILIKE $%d", "%s", argIndex))
			argIndex++
		}
		// Rebuild with actual column names
		conditions = make([]string, len(p.searchCols))
		for i, col := range p.searchCols {
			conditions[i] = fmt.Sprintf("%s ILIKE $%d", col, i+1)
		}
		whereClause += " AND (" + strings.Join(conditions, " OR ") + ")"
		searchPattern := "%" + q + "%"
		for range p.searchCols {
			args = append(args, searchPattern)
		}
		argIndex = len(p.searchCols) + 1
	}

	var totalCount int
	err := pool.QueryRow(p.ctx, p.countQuery+" "+whereClause, args...).Scan(&totalCount)
	if err != nil {
		p.c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	args = append(args, pageSize, offset)
	rows, err := pool.Query(p.ctx, fmt.Sprintf("%s %s %s LIMIT $%d OFFSET $%d",
		p.selectQuery, whereClause, p.defaultSort, argIndex, argIndex+1), args...)
	if err != nil {
		p.c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	results := []interface{}{}
	for rows.Next() {
		item, err := p.scanFn(rows)
		if err == nil {
			results = append(results, item)
		}
	}

	p.c.JSON(http.StatusOK, PaginatedResponse{
		Data:       results,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: (totalCount + pageSize - 1) / pageSize,
	})
}
