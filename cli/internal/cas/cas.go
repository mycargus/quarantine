// Package cas provides compare-and-swap (CAS) write logic for quarantine state
// updates via the GitHub Contents API.
package cas

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	ghclient "github.com/mycargus/quarantine/internal/github"
	"github.com/mycargus/quarantine/internal/quarantine"
)

// githubContentsClient is the subset of the GitHub client interface needed for CAS writes.
type githubContentsClient interface {
	GetContents(ctx context.Context, path, ref string) (content []byte, sha string, err error)
	UpdateContents(ctx context.Context, path, branch, message string, content []byte, sha string) error
}

// WriteStateWithCAS writes content to the GitHub Contents API using
// compare-and-swap semantics. On a 409 conflict it re-reads the remote state,
// merges with localState (quarantine wins per ADR-012), and retries.
// It retries up to maxRetries times total. On non-409 errors or after
// exhausting retries, it returns an error.
func WriteStateWithCAS(
	ctx context.Context,
	client githubContentsClient,
	localState *quarantine.State,
	content []byte,
	sha string,
	branch string,
	maxRetries int,
) error {
	currentContent := content
	currentSHA := sha
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := client.UpdateContents(ctx, "quarantine.json", branch, "chore: update quarantine state", currentContent, currentSHA)
		if err == nil {
			return nil
		}

		var apiErr *ghclient.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 {
			// Non-retryable error.
			return err
		}

		lastErr = err

		// 409 conflict: re-read and merge.
		remoteContent, remoteSHA, readErr := client.GetContents(ctx, "quarantine.json", branch)
		if readErr != nil {
			return fmt.Errorf("re-read after CAS conflict: %w", readErr)
		}

		var remoteState *quarantine.State
		if len(remoteContent) > 0 {
			rs, parseErr := quarantine.ParseState(bytes.NewReader(remoteContent))
			if parseErr != nil {
				return fmt.Errorf("parse remote state after CAS conflict: %w", parseErr)
			}
			remoteState = rs
		} else {
			remoteState = quarantine.NewEmptyState()
		}

		merged := quarantine.Merge(localState, remoteState)
		mergedBytes, marshalErr := merged.Marshal()
		if marshalErr != nil {
			return fmt.Errorf("marshal merged state: %w", marshalErr)
		}

		currentContent = mergedBytes
		currentSHA = remoteSHA
	}

	return fmt.Errorf("CAS write failed after %d attempts: %w", maxRetries, lastErr)
}

