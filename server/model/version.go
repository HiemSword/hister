// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"time"
)

// DocumentVersion stores unified text diffs of a document's HTML and plain
// text fields captured each time the document is re-indexed and its URL
// matches a versioning rule. Either field may be empty when the corresponding
// content was absent or unchanged.
type DocumentVersion struct {
	ID        uint      `gorm:"primaryKey"                json:"id"`
	CreatedAt time.Time `gorm:"index:idx_version_url_user" json:"created_at"`
	URL       string    `gorm:"index:idx_version_url_user" json:"url"`
	UserID    uint      `gorm:"index:idx_version_url_user" json:"user_id"`
	HTMLDiff  string    `gorm:"type:text"                  json:"html_diff"`
	TextDiff  string    `gorm:"type:text"                  json:"text_diff"`
}

// SaveDocumentVersion creates a new version entry for the given URL and user.
func SaveDocumentVersion(url string, userID uint, htmlDiff, textDiff string) error {
	v := &DocumentVersion{
		URL:      url,
		UserID:   userID,
		HTMLDiff: htmlDiff,
		TextDiff: textDiff,
	}
	return DB.Create(v).Error
}

// MoveDocumentVersions assigns stored versions of a document to a new owner.
func MoveDocumentVersions(url string, fromUserID, toUserID uint) error {
	if fromUserID == toUserID {
		return nil
	}
	return DB.Model(&DocumentVersion{}).
		Where("url = ? AND user_id = ?", url, fromUserID).
		Update("user_id", toUserID).Error
}

// CountDocumentVersions returns the number of stored versions for a URL and user.
func CountDocumentVersions(url string, userID uint) (int64, error) {
	var count int64
	if err := DB.Model(&DocumentVersion{}).
		Where("url = ? AND user_id = ?", url, userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetDocumentVersions returns all stored version diffs for a URL and user,
// ordered from newest to oldest.
func GetDocumentVersions(url string, userID uint) ([]*DocumentVersion, error) {
	var versions []*DocumentVersion
	if err := DB.Where("url = ? AND user_id = ?", url, userID).
		Order("created_at DESC").
		Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// GetDocumentVersionsUntil returns the version diffs needed to reconstruct the
// document state just before the version with the given ID was applied. It
// returns all versions for the URL/user whose ID is greater than or equal to
// versionID, ordered newest first, so callers can apply their reverse diffs in
// sequence to walk the document back to that point in history.
func GetDocumentVersionsUntil(url string, userID uint, versionID uint) ([]*DocumentVersion, error) {
	var versions []*DocumentVersion
	if err := DB.Where("url = ? AND user_id = ? AND id >= ?", url, userID, versionID).
		Order("id DESC").
		Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}
