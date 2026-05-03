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
