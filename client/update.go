// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"bytes"
	"encoding/json"
	"net/http"

	servertypes "github.com/asciimoo/hister/server/types"
)

// UpdateDocuments changes attributes on documents selected by a search query.
func (c *Client) UpdateDocuments(request servertypes.UpdateDocumentsRequest) (_ *servertypes.UpdateDocumentsResult, err error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(http.MethodPost, "/api/update", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeBody(resp, &err)
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	var result servertypes.UpdateDocumentsResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
