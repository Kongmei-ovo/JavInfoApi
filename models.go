package main

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
	Directors      []Director `json:"directors,omitempty"`
	Actors         []Actor    `json:"actors,omitempty"`
	Authors        []Author   `json:"authors,omitempty"`
	ImageURL       *string    `json:"image_url,omitempty"`
}

// Actress represents an actress
type Actress struct {
	ID         int    `json:"id"`
	NameRomaji *string `json:"name_romaji,omitempty"`
	NameKanji  *string `json:"name_kanji,omitempty"`
	NameKana   *string `json:"name_kana,omitempty"`
	ImageURL   *string `json:"image_url,omitempty"`
	Ordinality *int   `json:"ordinality,omitempty"`
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

// CategoryWithCount represents a category with its video count
type CategoryWithCount struct {
	ID         int    `json:"id"`
	NameEn     string `json:"name_en"`
	NameJa     *string `json:"name_ja,omitempty"`
	VideoCount int64  `json:"video_count"`
}

// ActressWithCount represents an actress with her video count
type ActressWithCount struct {
	ID         int    `json:"id"`
	NameRomaji *string `json:"name_romaji,omitempty"`
	NameKanji  *string `json:"name_kanji,omitempty"`
	NameKana   *string `json:"name_kana,omitempty"`
	ImageURL   *string `json:"image_url,omitempty"`
	MovieCount int64  `json:"movie_count"`
}

// PaginatedResponse is a generic paginated response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalCount int         `json:"total_count"`
	TotalPages int         `json:"total_pages"`
}

// Batch request structs
type BatchIDsRequest struct {
	IDs []string `json:"ids"`
}

type BatchDvdIDsRequest struct {
	DvdIDs []string `json:"dvd_ids"`
}

type BatchActressVideosRequest struct {
	IDs      []int `json:"ids"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}
