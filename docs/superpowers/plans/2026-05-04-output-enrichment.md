# Output Enrichment + Search Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance the JavInfoApi with output enrichment (decensor, image URLs, Wikidata names, dvd_code extraction), new entity support (director, actor, author), and dvd_code search fallback.

**Architecture:** Three new files (`decensor.go`, `enrichment.go`, `wikidata.go`) handle post-processing. Existing `handler_video.go` and `handler_aux.go` get extended with new entity loading and search logic. All enrichment is applied at output time — no database writes.

**Tech Stack:** Go 1.23, Gin, pgx, embed (for CSV), net/http + encoding/json (for Wikidata SPARQL), sync.Map (for caching)

---

### Task 1: Models — New structs and Video fields

**Files:**
- Modify: `models.go`

- [ ] **Step 1: Add Director, Actor, Author structs and Video fields**

Add after the existing `Category` struct (line 66) in `models.go`:

```go
// Director represents a video director
type Director struct {
	ID         int     `json:"id"`
	NameRomaji *string `json:"name_romaji,omitempty"`
	NameKanji  *string `json:"name_kanji,omitempty"`
	NameKana   *string `json:"name_kana,omitempty"`
}

// Actor represents a male actor (histrion)
type Actor struct {
	ID        int     `json:"id"`
	NameKanji *string `json:"name_kanji,omitempty"`
	NameKana  *string `json:"name_kana,omitempty"`
}

// Author represents a video author (manga/doujinshi)
type Author struct {
	ID        int     `json:"id"`
	NameKanji *string `json:"name_kanji,omitempty"`
	NameKana  *string `json:"name_kana,omitempty"`
}
```

Add to the `Video` struct after `Categories` (line 27):

```go
	Directors  []Director `json:"directors,omitempty"`
	Actors     []Actor    `json:"actors,omitempty"`
	Authors    []Author   `json:"authors,omitempty"`
	ImageURL   *string    `json:"image_url,omitempty"`
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors (new fields have zero values, won't break existing code)

- [ ] **Step 3: Commit**

```bash
git add models.go
git commit -m "feat(models): add Director, Actor, Author structs and Video enrichment fields"
```

---

### Task 2: Decensor — CSV loading and string replacement

**Files:**
- Create: `data/decensor.csv`
- Create: `decensor.go`

- [ ] **Step 1: Create data/decensor.csv**

Create `data/decensor.csv` with the following content (101 rows from R18dev_SQL):

```
A***e,Abuse
A****d,Abused
A****p,Asleep
A*****t,Assault
A******d,Assaulted
A*******g,Assaulting
B*****p,Beat Up
B**d,Bled
B******g,Bleeding
B***d,Blood
B*******y,Brutality
C***d,Child
C******n,Children
C*********a,Coprophilia
C***l,Cruel
C******y,Cruelty
D******e,Disgrace
D*******d,Disgraced
D***k,Drink
D**g,Drug
D*****d,Drugged
D******g,Drugging
D***s,Drugs
D***k,Drunk
E***************l,Elementary School
E******d,Enforced
F***e,Force
F****d,Forceful
F******l,Forceful
F*****g,Forcing
G*******g,Gang Bang
G******g,Gangbang
H*********d,Humiliated
H**********g,Humiliating
H**********n,Humiliation
H***o,Hypno
H*******s,Hypnosis
H******c,Hypnotic
H*******m,Hypnotism
H********e,Hypnotize
H*********d,Hypnotized
I******l,Illegal
J***********h,Junior High
J******e,Juvenile
K*d,Kid
K****p,Kidnap
K**s,Kids
K**l,Kill
K****r,Killer
K*****g,Killing
K***********n,Kindergarten
L**i,Loli
L******n,Lolicon
L****a,Lolita
M***********l,Mind Control
M****t,Molest
M*********n,Molestation
M******d,Molested
M******r,Molester
M*******g,Molesting
M**************n,Mother And Son
N******y,Nursery
P*********t,Passed Out
P**********t,Passing Out
P**p,Poop
P********l,Preschool
P****h,Punish
P******d,Punished
P******r,Punisher
P*******g,Punishing
R**e,Rape
R***d,Raped
R***s,Rapes
R****g,Raping
R****t,Rapist
R*****s,Rapists
S**t,Scat
S*******y,Scatology
S*********l,School Girl
S**********s,School Girls
S********l,Schoolgirl
S*********s,Schoolgirls
S***a,Shota
S******n,Shotacon
S***e,Slave
S******g,Sleeping
S*****t,Student
S******s,Students
S*********n,Submission
T******e,Tentacle
T*******s,Tentacles
T******e,Torture
T*******d,Tortured
U**********s,Unconscious
U*******g,Unwilling
V******e,Violate
V*******d,Violated
V********n,Violation
V******e,Violence
V*****t,Violent
Y***********l,Young Girl
```

- [ ] **Step 2: Create decensor.go**

Create `decensor.go`:

```go
package main

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed data/decensor.csv
var decensorCSV string

var (
	decensorOnce    sync.Once
	decensorPairs   [][2]string
)

func initDecensor() {
	decensorOnce.Do(func() {
		lines := strings.Split(strings.TrimSpace(decensorCSV), "\n")
		decensorPairs = make([][2]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) != 2 {
				continue
			}
			decensorPairs = append(decensorPairs, [2]string{parts[0], parts[1]})
		}
	})
}

// decensor replaces censored terms in the input string with their uncensored equivalents.
func decensor(s string) string {
	initDecensor()
	for _, pair := range decensorPairs {
		s = strings.ReplaceAll(s, pair[0], pair[1])
	}
	return s
}

// decensorPtr applies decensor to a *string, returning nil if input is nil.
func decensorPtr(s *string) *string {
	if s == nil {
		return nil
	}
	result := decensor(*s)
	return &result
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add data/decensor.csv decensor.go
git commit -m "feat(decensor): add text decensoring with embedded CSV mapping"
```

---

### Task 3: Enrichment — dvd_code extraction, image URL, enrichVideo

**Files:**
- Create: `enrichment.go`
- Modify: `handler_video.go` (add enrichVideo calls)

- [ ] **Step 1: Create enrichment.go**

Create `enrichment.go`:

```go
package main

import (
	"regexp"
	"strings"
)

// dvdCodeRegex extracts dvd code prefix and number from titles.
// Captures: group(1)=prefix letters, group(2)=number (optionally ending in Z/E)
var dvdCodeRegex = regexp.MustCompile(`(?i).*?([A-Z]+|[3DSVR]+|[T28]+|[T38]+)-?(\d+[Z]?[E]?)(?:-pt)?(\d{1,2})?.*`)

// extractDvdCode attempts to extract a dvd code from title_ja or title_en.
// Returns nil if no match found.
func extractDvdCode(titleJa, titleEn *string) *string {
	for _, title := range []*string{titleJa, titleEn} {
		if title == nil {
			continue
		}
		matches := dvdCodeRegex.FindStringSubmatch(*title)
		if len(matches) >= 3 && matches[1] != "" && matches[2] != "" {
			code := strings.ToUpper(matches[1]) + "-" + matches[2]
			return &code
		}
	}
	return nil
}

// buildImageURL constructs a full image URL from jacket_full_url and service_code.
func buildImageURL(jacketFullURL *string, serviceCode string) *string {
	if jacketFullURL == nil || *jacketFullURL == "" {
		return nil
	}
	var url string
	switch serviceCode {
	case "digital":
		url = "https://awsimgsrc.dmm.com/dig/" + *jacketFullURL + ".jpg"
	case "mono":
		path := strings.ReplaceAll(*jacketFullURL, "adult/", "")
		url = "https://awsimgsrc.dmm.com/dig/" + path + ".jpg"
	default:
		url = "https://pics.dmm.co.jp/" + *jacketFullURL + ".jpg"
	}
	return &url
}

// enrichVideo applies all output enrichment to a video.
func enrichVideo(video *Video) {
	// 1. dvd_code extraction from title
	if video.DvdID == nil {
		video.DvdID = extractDvdCode(video.TitleJa, video.TitleEn)
	}

	// 2. Image URL construction
	video.ImageURL = buildImageURL(video.JacketFullURL, video.ServiceCode)

	// 3. Decensor English text fields
	video.TitleEn = decensorPtr(video.TitleEn)
	video.CommentEn = decensorPtr(video.CommentEn)

	// 4. Decensor related entities
	if video.Maker != nil {
		video.Maker.NameEn = decensorPtr(video.Maker.NameEn)
	}
	if video.Label != nil {
		video.Label.NameEn = decensorPtr(video.Label.NameEn)
	}
	if video.Series != nil {
		video.Series.NameEn = decensorPtr(video.Series.NameEn)
	}
	for i := range video.Categories {
		video.Categories[i].NameEn = decensor(video.Categories[i].NameEn)
	}
	for i := range video.Actresses {
		video.Actresses[i].NameRomaji = decensorPtr(video.Actresses[i].NameRomaji)
	}
	for i := range video.Directors {
		video.Directors[i].NameRomaji = decensorPtr(video.Directors[i].NameRomaji)
	}
}

// enrichVideoLight applies only dvd_code extraction and image URL (for list views).
func enrichVideoLight(video *Video) {
	if video.DvdID == nil {
		video.DvdID = extractDvdCode(video.TitleJa, video.TitleEn)
	}
	video.ImageURL = buildImageURL(video.JacketFullURL, video.ServiceCode)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add enrichment.go
git commit -m "feat(enrichment): add dvd_code extraction, image URL construction, enrichVideo pipeline"
```

---

### Task 4: New entity queries — Director, Actor, Author loading

**Files:**
- Modify: `handler_video.go`

- [ ] **Step 1: Add director/actor/author loading to loadRelatedData**

In `handler_video.go`, add three new goroutines inside `loadRelatedData` function, after the existing categories goroutine (after line 475, before `wg.Wait()`):

```go
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
```

- [ ] **Step 2: Add batch loading for directors/actors/authors to loadRelatedDataBatch**

In `loadRelatedDataBatch`, after the existing categories batch loading goroutine (before `wg.Wait()`), add three new batch loading blocks:

```go
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
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add handler_video.go
git commit -m "feat(handler_video): add director/actor/author loading to single and batch queries"
```

---

### Task 5: Search enhancement — dvd_code to content_id fallback

**Files:**
- Modify: `handler_video.go` (searchVideos)
- Modify: `handler_batch.go` (batchLookupVideos)

- [ ] **Step 1: Enhance searchVideos dvd_id handling**

In `handler_video.go`, replace the existing dvd_id filter block (lines 195-200) with enhanced logic that falls back to content_id lookup:

```go
	if dvdID != "" {
		// Normalize: uppercase, ensure hyphen format
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
```

- [ ] **Step 2: Enhance batchLookupVideos with content_id fallback**

In `handler_batch.go`, after the existing dvd_id query (after line 127, where `videos` is populated), add content_id fallback for unmatched dvd_codes:

```go
	// Find which normalized IDs were not matched by dvd_id
	matchedNorms := make(map[string]bool)
	for _, v := range videos {
		if v.DvdID != nil {
			matchedNorms[strings.ToLower(strings.ReplaceAll(*v.DvdID, "-", ""))] = true
		}
	}

	// Collect unmatched IDs and try content_id fallback
	var unmatched []string
	for _, norm := range normalizedIDs {
		if !matchedNorms[norm] {
			// Try extracting prefix-number from original input
			originalIdx := -1
			for i, id := range req.DvdIDs {
				if strings.ToLower(strings.ReplaceAll(id, "-", "")) == norm {
					originalIdx = i
					break
				}
			}
			if originalIdx >= 0 {
				matches := dvdCodeRegex.FindStringSubmatch(req.DvdIDs[originalIdx])
				if len(matches) >= 3 && matches[1] != "" && matches[2] != "" {
					fallback := strings.ToLower(matches[1] + matches[2])
					unmatched = append(unmatched, fallback)
				}
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
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add handler_video.go handler_batch.go
git commit -m "feat(search): enhance dvd_code lookup with content_id fallback"
```

---

### Task 6: Wikidata — SPARQL query and caching for actress names

**Files:**
- Create: `wikidata.go`

- [ ] **Step 1: Create wikidata.go**

Create `wikidata.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	wikidataCache sync.Map // map[int]*string (actress_id -> japanese name or nil)
	wikidataHTTP  = &http.Client{Timeout: 5 * time.Second}
)

// wikidataQueryResponse represents the Wikidata SPARQL JSON response.
type wikidataQueryResponse struct {
	Results struct {
		Bindings []struct {
			ItemLabel struct {
				Value string `json:"value"`
			} `json:"itemLabel"`
		} `json:"bindings"`
	} `json:"results"`
}

// lookupWikidataJapaneseName queries Wikidata for the Japanese name of an actress by DMM ID.
// Returns the Japanese label or nil if not found. Results are cached.
func lookupWikidataJapaneseName(actressID int) *string {
	// Check cache first
	if cached, ok := wikidataCache.Load(actressID); ok {
		return cached.(*string)
	}

	sparql := fmt.Sprintf(`SELECT DISTINCT ?itemLabel WHERE {
  SERVICE wikibase:label { bd:serviceParam wikibase:language "ja". }
  {
    SELECT DISTINCT ?item WHERE {
      ?item p:P9781 ?statement0.
      ?statement0 (ps:P9781) "%d".
    }
    LIMIT 1
  }
}`, actressID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqURL := "https://query.wikidata.org/sparql?" + url.Values{
		"format": {"json"},
		"query":  {sparql},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		wikidataCache.Store(actressID, (*string)(nil))
		return nil
	}
	req.Header.Set("User-Agent", "JavInfoApi/1.0")

	resp, err := wikidataHTTP.Do(req)
	if err != nil {
		wikidataCache.Store(actressID, (*string)(nil))
		return nil
	}
	defer resp.Body.Close()

	var result wikidataQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		wikidataCache.Store(actressID, (*string)(nil))
		return nil
	}

	if len(result.Results.Bindings) > 0 && result.Results.Bindings[0].ItemLabel.Value != "" {
		name := result.Results.Bindings[0].ItemLabel.Value
		wikidataCache.Store(actressID, &name)
		return &name
	}

	wikidataCache.Store(actressID, (*string)(nil))
	return nil
}

// supplementActressNames fills in missing name_kanji for actresses using Wikidata.
// Modifies the slice in place. Uses goroutines for parallel lookups.
func supplementActressNames(actresses []Actress) {
	var wg sync.WaitGroup
	for i := range actresses {
		if actresses[i].NameKanji != nil {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if name := lookupWikidataJapaneseName(actresses[idx].ID); name != nil {
				actresses[idx].NameKanji = name
			}
		}(i)
	}
	wg.Wait()
}
```

- [ ] **Step 2: Wire up supplementActressNames in loadRelatedData**

In `handler_video.go`, inside `loadRelatedData`, after the actresses goroutine completes (before the directors goroutine), add:

```go
	// Supplement actress names via Wikidata after actresses are loaded
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait for actresses to be loaded (they're in the same WaitGroup,
		// so by the time this runs, actresses are already populated)
	}()
```

Actually, since all goroutines run concurrently and wg.Wait() synchronizes, we need to call `supplementActressNames` after `wg.Wait()`. Modify `loadRelatedData` to call it at the end:

Find `wg.Wait()` at the end of `loadRelatedData` and change to:

```go
	wg.Wait()

	// Supplement missing actress names via Wikidata
	if len(video.Actresses) > 0 {
		supplementActressNames(video.Actresses)
	}
```

- [ ] **Step 3: Wire up supplementActressNames in loadRelatedDataBatch**

In `loadRelatedDataBatch`, after `wg.Wait()`, add:

```go
	// Supplement missing actress names via Wikidata
	for _, v := range videos {
		if len(v.Actresses) > 0 {
			supplementActressNames(v.Actresses)
		}
	}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add wikidata.go handler_video.go
git commit -m "feat(wikidata): add SPARQL lookup for supplementing actress Japanese names"
```

---

### Task 7: EnrichVideo integration — wire up in all handlers

**Files:**
- Modify: `handler_video.go`
- Modify: `handler_actress.go`

- [ ] **Step 1: Add enrichVideo call to getVideo**

In `handler_video.go`, in `getVideo` function, after `loadRelatedData(ctx, &video)` (line 121), add:

```go
	enrichVideo(&video)
```

- [ ] **Step 2: Add enrichVideo calls to listVideos and searchVideos**

In `listVideos`, after the scan loop that populates `videos`, add:

```go
	for i := range videos {
		enrichVideoLight(&videos[i])
	}
```

In `searchVideos`, after the scan loop that populates `videos`, add:

```go
	for i := range videos {
		enrichVideoLight(&videos[i])
	}
```

- [ ] **Step 3: Add enrichVideo calls to batchGetVideos**

In `handler_batch.go`, in `batchGetVideos`, after `loadRelatedDataBatch(ctx, videos)`, add:

```go
	for i := range videos {
		enrichVideo(&videos[i])
	}
```

- [ ] **Step 4: Add enrichVideo calls to batchLookupVideos**

In `handler_batch.go`, in `batchLookupVideos`, after `loadRelatedDataBatch(ctx, videos)` (the existing one and the fallback one), add enrichment before building the result map. Find the section where the result map is built and add before it:

```go
	for i := range videos {
		enrichVideo(&videos[i])
	}
```

- [ ] **Step 5: Add enrichVideoLight to getActressVideos and batchActressVideos**

In `handler_actress.go`, in `getActressVideos`, after the scan loop:

```go
	for i := range videos {
		enrichVideoLight(&videos[i])
	}
```

In `batchActressVideos`, in each goroutine, after the scan loop:

```go
	for i := range videos {
		enrichVideoLight(&videos[i])
	}
```

- [ ] **Step 6: Verify it compiles and test**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add handler_video.go handler_actress.go handler_batch.go
git commit -m "feat(enrichment): wire up enrichVideo in all video handlers"
```

---

### Task 8: Auxiliary handlers — Directors, Actors, Authors list endpoints

**Files:**
- Modify: `handler_aux.go`

- [ ] **Step 1: Add listDirectors**

In `handler_aux.go`, add after `listSeries`:

```go
func listDirectors(c *gin.Context) {
	listSimpleEntity(c, "derived_director", []string{"name_romaji", "name_kanji", "name_kana"}, func(rows pgx.Rows) (interface{}, error) {
		var d Director
		err := rows.Scan(&d.ID, &d.NameRomaji, &d.NameKanji, &d.NameKana)
		return d, err
	})
}
```

Note: `listSimpleEntity` uses `SELECT id, name_en, name_ja` but director has different columns. We need to use `listEntity` directly instead:

```go
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
```

- [ ] **Step 2: Add listActors**

```go
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
```

- [ ] **Step 3: Add listAuthors**

```go
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
```

- [ ] **Step 4: Register new routes in main.go**

In `main.go`, after the auxiliary data endpoints section (after line 62), add:

```go
	r.GET("/api/v1/directors", listDirectors)
	r.GET("/api/v1/actors", listActors)
	r.GET("/api/v1/authors", listAuthors)
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build ./...`

Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add handler_aux.go main.go
git commit -m "feat(api): add director/actor/author list endpoints"
```

---

### Task 9: Verification — Build, start server, test endpoints

**Files:**
- None (testing only)

- [ ] **Step 1: Full build check**

Run: `cd /Users/kongmei/Code/JavInfoApi && go build -o javinfoapi ./...`

Expected: Binary built successfully

- [ ] **Step 2: Start server and test enrichment**

Run: `cd /Users/kongmei/Code/JavInfoApi && ./javinfoapi &`

Wait 2 seconds, then test:

```bash
# Test video detail with enrichment
curl -s http://localhost:8080/api/v1/videos/47jf00345dod | python3 -m json.tool | head -40

# Test search with dvd_code
curl -s "http://localhost:8080/api/v1/videos/search?dvd_id=SSNI-123" | python3 -m json.tool | head -20

# Test new endpoints
curl -s "http://localhost:8080/api/v1/directors?page_size=3" | python3 -m json.tool
curl -s "http://localhost:8080/api/v1/actors?page_size=3" | python3 -m json.tool
curl -s "http://localhost:8080/api/v1/authors?page_size=3" | python3 -m json.tool
```

- [ ] **Step 3: Stop server**

Run: `kill %1 2>/dev/null || pkill -f javinfoapi`

- [ ] **Step 4: Final commit if any fixes needed**

If any fixes were needed during testing, commit them.
