package ytdlp

import "time"

// subtitleTrack represents a single subtitle format entry from yt-dlp.
type subtitleTrack struct {
	Ext  string `json:"ext"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// chapter represents a video chapter from yt-dlp.
type chapter struct {
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Title     string  `json:"title"`
}

// videoInfo holds the subset of yt-dlp JSON output we care about.
type videoInfo struct {
	Title             string                     `json:"title"`
	Description       string                     `json:"description"`
	Uploader          string                     `json:"uploader"`
	Channel           string                     `json:"channel"`
	Duration          float64                    `json:"duration"`
	ViewCount         int64                      `json:"view_count"`
	LikeCount         int64                      `json:"like_count"`
	UploadDate        string                     `json:"upload_date"`
	Thumbnail         string                     `json:"thumbnail"`
	WebpageURL        string                     `json:"webpage_url"`
	Categories        []string                   `json:"categories"`
	Tags              []string                   `json:"tags"`
	Chapters          []chapter                  `json:"chapters"`
	Subtitles         map[string][]subtitleTrack `json:"subtitles"`
	AutomaticCaptions map[string][]subtitleTrack `json:"automatic_captions"`
	Language          string                     `json:"language"`
	PlaylistTitle     string                     `json:"playlist_title"`
	PlaylistIndex     *int                       `json:"playlist_index"`
	PlaylistCount     *int                       `json:"playlist_count"`
}

// videoPreviewData is the structured JSON returned to the frontend
// when Template is "video".
type videoPreviewData struct {
	Title             string           `json:"title"`
	Uploader          string           `json:"uploader"`
	Duration          float64          `json:"duration"`
	DurationFormatted string           `json:"durationFormatted"`
	UploadDate        string           `json:"uploadDate"`
	ViewCount         int64            `json:"viewCount"`
	LikeCount         int64            `json:"likeCount"`
	Categories        []string         `json:"categories"`
	Tags              []string         `json:"tags"`
	Description       string           `json:"description"`
	Thumbnail         string           `json:"thumbnail"`
	Chapters          []chapterPreview `json:"chapters,omitempty"`
	Playlist          *playlistPreview `json:"playlist,omitempty"`
	Transcript        string           `json:"transcript,omitempty"`
	WebpageURL        string           `json:"webpageUrl"`
}

type chapterPreview struct {
	Title     string `json:"title"`
	StartTime string `json:"startTime"`
}

type playlistPreview struct {
	Title string `json:"title"`
	Index int    `json:"index"`
	Count int    `json:"count"`
}

// cachedInfo stores a fetched videoInfo with a timestamp for expiry.
type cachedInfo struct {
	info      *videoInfo
	fetchedAt time.Time
}

// knownDomains lists domains that yt-dlp commonly supports.
// The Match function checks whether the URL's hostname equals or is
// a subdomain of each entry (e.g. "youtube.com" matches "www.youtube.com").
var knownDomains = []string{
	"youtube.com",
	"youtu.be",
	"vimeo.com",
	"dailymotion.com",
	"twitch.tv",
	"bilibili.com",
	"nicovideo.jp",
	"soundcloud.com",
	"bandcamp.com",
	"mixcloud.com",
	"ted.com",
	"media.ccc.de",
	"tiktok.com",
}

// knownHostSubstrings lists hostname fragments matched with strings.Contains
// for platforms with user-chosen subdomains (e.g. instance.peertube.live).
var knownHostSubstrings = []string{
	"peertube",
}
