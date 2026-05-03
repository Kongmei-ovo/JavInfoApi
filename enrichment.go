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

// buildImageURL constructs a full image URL from a relative jacket path and service_code.
func buildImageURL(path *string, serviceCode string) *string {
	if path == nil || *path == "" {
		return nil
	}
	var url string
	switch serviceCode {
	case "digital":
		url = "https://awsimgsrc.dmm.com/dig/" + *path + ".jpg"
	case "mono":
		clean := strings.ReplaceAll(*path, "adult/", "")
		url = "https://awsimgsrc.dmm.com/dig/" + clean + ".jpg"
	default:
		url = "https://pics.dmm.co.jp/" + *path + ".jpg"
	}
	return &url
}

// resolveJacketURLs replaces relative jacket paths with full accessible URLs.
func resolveJacketURLs(video *Video) {
	video.JacketFullURL = buildImageURL(video.JacketFullURL, video.ServiceCode)
	video.JacketThumbURL = buildImageURL(video.JacketThumbURL, video.ServiceCode)
}

// enrichVideo applies all output enrichment to a video.
func enrichVideo(video *Video) {
	// 1. dvd_code extraction from title
	if video.DvdID == nil {
		video.DvdID = extractDvdCode(video.TitleJa, video.TitleEn)
	}

	// 2. Resolve jacket URLs to full accessible URLs
	resolveJacketURLs(video)

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
	resolveJacketURLs(video)
}
