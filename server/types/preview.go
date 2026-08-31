// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import "time"

// PreviewDocumentDetails contains the stored document properties that can be
// used to understand and construct search queries. Large content and internal
// storage fields are deliberately excluded.
type PreviewDocumentDetails struct {
	Type     string         `json:"type"`
	Language string         `json:"language"`
	Label    string         `json:"label"`
	Visits   uint           `json:"visits"`
	UserID   uint           `json:"user_id"`
	Metadata map[string]any `json:"metadata"`
}

// DocumentPreviewResponse is the API response for a rendered document
// preview and its searchable properties.
type DocumentPreviewResponse struct {
	Title            string                 `json:"title"`
	Content          string                 `json:"content"`
	Template         string                 `json:"template"`
	Added            int64                  `json:"added"`
	Updated          int64                  `json:"updated"`
	Details          PreviewDocumentDetails `json:"details"`
	Meta             map[string]any         `json:"meta,omitempty"`
	VersionID        uint                   `json:"version_id,omitempty"`
	VersionCreatedAt *time.Time             `json:"version_created_at,omitempty"`
	VersionCount     int64                  `json:"version_count,omitempty"`
}
