package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 181: doctor surfaces a canonical 4xx unreachability error ---
//
// Per ADR-037, when `GET /repos/{owner}/{repo}` returns a 4xx status,
// `quarantine doctor` MUST print the canonical "Cannot reach ... on GitHub
// (NNN)" block (verbatim) and exit 2. GitHub does not distinguish "repo
// missing" from "token cannot see it", so the same canonical message applies
// to 401, 403, and 404 — only the status code differs.
//
// The interface test covers the 404 wiring end-to-end. The exact text is
// pinned by unit tests on the pure formatter so the interface test remains
// focused on routing + status-code extraction.

func TestDoctorPrintsCanonicalUnreachableErrorOn404(t *testing.T) {
	dir := t.TempDir()
	setupFakeGitRepo(t, dir, "https://github.com/my-org/my-project.git")
	preCreateValidPhase2Config(t, dir)
	chdirTest(t, dir)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found","documentation_url":"https://docs.github.com/rest"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	output, err := executeDoctorCmd(t, nil, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "GET /repos/my-org/my-project returns 404",
		Should:   "exit with code 2",
		Actual:   exitCodeFromError(err),
		Expected: 2,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "doctor received a 404 from the reachability call",
		Should:   "print the canonical scenario-181 unreachable error block verbatim",
		Actual:   strings.Contains(output, formatDoctorUnreachableError("my-org", "my-project", 404)),
		Expected: true,
	})
}

// --- Pure unit tests for formatDoctorUnreachableError ---
//
// The 4xx error message is deterministic — it depends only on owner, repo,
// and the HTTP status code. Pinning the exact text here means the interface
// test asserts via the formatter (so refactors do not require updating the
// interface test) and any text drift surfaces at the unit layer.

func TestFormatDoctorUnreachableErrorRendersCanonical404Block(t *testing.T) {
	expected := "Error: Cannot reach my-org/my-project on GitHub (404).\n" +
		"Either the repository does not exist or the configured token does not\n" +
		"have access to it. Verify github.owner and github.repo in\n" +
		".quarantine/config.yml, and confirm QUARANTINE_GITHUB_TOKEN has access\n" +
		"to the repository."

	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner='my-org', repo='my-project', statusCode=404",
		Should:   "render the canonical scenario-181 unreachable error block verbatim",
		Actual:   formatDoctorUnreachableError("my-org", "my-project", 404),
		Expected: expected,
	})
}

func TestFormatDoctorUnreachableErrorRendersCanonical403Block(t *testing.T) {
	expected := "Error: Cannot reach my-org/my-project on GitHub (403).\n" +
		"Either the repository does not exist or the configured token does not\n" +
		"have access to it. Verify github.owner and github.repo in\n" +
		".quarantine/config.yml, and confirm QUARANTINE_GITHUB_TOKEN has access\n" +
		"to the repository."

	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner='my-org', repo='my-project', statusCode=403",
		Should:   "render the canonical block with status 403",
		Actual:   formatDoctorUnreachableError("my-org", "my-project", 403),
		Expected: expected,
	})
}

func TestFormatDoctorUnreachableErrorRendersCanonical401Block(t *testing.T) {
	expected := "Error: Cannot reach my-org/my-project on GitHub (401).\n" +
		"Either the repository does not exist or the configured token does not\n" +
		"have access to it. Verify github.owner and github.repo in\n" +
		".quarantine/config.yml, and confirm QUARANTINE_GITHUB_TOKEN has access\n" +
		"to the repository."

	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner='my-org', repo='my-project', statusCode=401",
		Should:   "render the canonical block with status 401",
		Actual:   formatDoctorUnreachableError("my-org", "my-project", 401),
		Expected: expected,
	})
}
