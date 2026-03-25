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

// ErrCASExhausted is returned when all CAS retry attempts are exhausted due to
// repeated 409 conflict responses.
var ErrCASExhausted = errors.New("CAS write exhausted")

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
//
// removedTestIDs is the set of test IDs that the caller explicitly removed from
// the local state (e.g., via unquarantine). After a CAS conflict and merge,
// any of these IDs that reappear in the merged result are returned as
// re-quarantined test IDs. The caller is responsible for logging the warning.
// Pass nil or an empty slice if no tests were removed.
//
// The first return value is the list of re-quarantined test IDs (present in
// merged but listed in removedTestIDs). Empty when no conflict occurred or
// when no removed tests reappeared.
func WriteStateWithCAS(
	ctx context.Context,
	client githubContentsClient,
	localState *quarantine.State,
	content []byte,
	sha string,
	branch string,
	maxRetries int,
	removedTestIDs []string,
) ([]string, error) {
	currentContent := content
	currentSHA := sha
	var lastErr error
	var lastReQuarantined []string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := client.UpdateContents(ctx, "quarantine.json", branch, "chore: update quarantine state", currentContent, currentSHA)
		if err == nil {
			return lastReQuarantined, nil
		}

		var apiErr *ghclient.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 {
			// Non-retryable error.
			return nil, err
		}

		lastErr = err

		// 409 conflict: re-read and merge.
		remoteContent, remoteSHA, readErr := client.GetContents(ctx, "quarantine.json", branch)
		if readErr != nil {
			return nil, fmt.Errorf("re-read after CAS conflict: %w", readErr)
		}

		var remoteState *quarantine.State
		if len(remoteContent) > 0 {
			rs, parseErr := quarantine.ParseState(bytes.NewReader(remoteContent))
			if parseErr != nil {
				return nil, fmt.Errorf("parse remote state after CAS conflict: %w", parseErr)
			}
			remoteState = rs
		} else {
			remoteState = quarantine.NewEmptyState()
		}

		merged := quarantine.Merge(localState, remoteState)
		mergedBytes, marshalErr := merged.Marshal()
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal merged state: %w", marshalErr)
		}

		// Detect re-quarantined tests: in removedTestIDs and present in merged.
		lastReQuarantined = detectReQuarantined(removedTestIDs, merged)

		currentContent = mergedBytes
		currentSHA = remoteSHA
	}

	return nil, fmt.Errorf("%w: CAS write failed after %d attempts: %w", ErrCASExhausted, maxRetries, lastErr)
}

// detectReQuarantined returns the subset of removedTestIDs that reappeared in
// the merged state. These are tests that the local side explicitly removed
// (unquarantined) but which came back due to quarantine-wins merge semantics.
// Pure function — no I/O.
func detectReQuarantined(removedTestIDs []string, merged *quarantine.State) []string {
	if len(removedTestIDs) == 0 {
		return nil
	}
	var ids []string
	for _, id := range removedTestIDs {
		if merged.HasTest(id) {
			ids = append(ids, id)
		}
	}
	return ids
}
