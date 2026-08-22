// SPDX-License-Identifier: AGPL-3.0-or-later

package types

// DocumentChanges describes mutable indexed document attributes. Pointer
// fields distinguish an omitted change from setting a field to its zero value.
type DocumentChanges struct {
	UserID   *uint   `json:"user_id,omitempty"`
	Label    *string `json:"label,omitempty"`
	Title    *string `json:"title,omitempty"`
	Language *string `json:"language,omitempty"`
}

// Empty reports whether no document attribute changes were requested.
func (c DocumentChanges) Empty() bool {
	return c.UserID == nil && c.Label == nil && c.Title == nil && c.Language == nil
}

// UpdateDocumentsRequest selects documents and describes the changes to apply.
type UpdateDocumentsRequest struct {
	Query   string          `json:"query"`
	Changes DocumentChanges `json:"changes"`
	DryRun  bool            `json:"dry_run,omitempty"`
}

// DocumentOwnershipChange records an applied document identity change.
// OwnershipChanges is internal server state and is not included in responses.
type DocumentOwnershipChange struct {
	URL        string
	FromUserID uint
	ToUserID   uint
}

// UpdateDocumentsResult summarizes a document update operation.
type UpdateDocumentsResult struct {
	Matched          int                       `json:"matched"`
	Updated          int                       `json:"updated"`
	Unchanged        int                       `json:"unchanged"`
	Conflicts        int                       `json:"conflicts"`
	OwnershipChanges []DocumentOwnershipChange `json:"-"`
}
