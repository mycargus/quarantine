// Package github provides GitHub API operations for quarantine state management.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Artifact represents a single GitHub Actions artifact entry.
type Artifact struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	ArchiveDownloadURL string `json:"archive_download_url"`
}

// artifactsResponse is the JSON response from GET /repos/{owner}/{repo}/actions/artifacts.
type artifactsResponse struct {
	TotalCount int        `json:"total_count"`
	Artifacts  []Artifact `json:"artifacts"`
}

// ListArtifacts calls GET /repos/{owner}/{repo}/actions/artifacts and returns
// the list of artifacts. The name parameter filters by artifact name.
// The perPage parameter limits how many artifacts are returned.
func (c *Client) ListArtifacts(ctx context.Context, name string, perPage int) ([]Artifact, error) {
	path := fmt.Sprintf("%s/actions/artifacts?name=%s&per_page=%d", c.repoPath(), name, perPage)
	req, err := c.newRequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("list artifacts failed with status %d", resp.StatusCode),
		}
	}

	var ar artifactsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("decode artifacts response: %w", err)
	}
	return ar.Artifacts, nil
}

// DownloadArtifactZip downloads the ZIP archive for an artifact from the given
// absolute URL. The URL is used as-is (not prefixed with baseURL).
// Returns the raw bytes of the ZIP file.
func (c *Client) DownloadArtifactZip(ctx context.Context, archiveURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", archiveURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download artifact: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("download artifact failed with status %d", resp.StatusCode),
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read artifact body: %w", err)
	}
	return data, nil
}
