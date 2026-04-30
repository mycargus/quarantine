package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	riteway "github.com/mycargus/riteway-golang"
)

// --- Scenario 179: doctor reads owner/repo from config, calls reachability once, exits 0 ---
//
// Per ADR-037, `quarantine doctor` MUST read `github.owner` and `github.repo`
// from `.quarantine/config.yml` (NOT from `git remote -v`), make exactly one
// `GET /repos/{owner}/{repo}` reachability call, and on 200 print a success
// summary. The doctor MUST NOT inspect `response.permissions` or
// `response.has_issues` (those are token-scope diagnostics, out of scope).
// The doctor MUST NOT emit a warning about the origin URL — Gerrit, Bitbucket,
// or other non-GitHub origins are explicitly supported when config carries the
// canonical owner/repo.

func TestDoctorReadsConfigAndCallsReachabilityOnce(t *testing.T) {
	dir := t.TempDir()
	// Gerrit origin proves doctor does NOT scan origin in M20: a Gerrit URL
	// would fail `ParseRemote`'s `u.Host == "github.com"` check and surface as
	// "not a GitHub URL" if the legacy auto-detect path were still in use.
	setupFakeGitRepo(t, dir, "https://gerrit.example.com/foo/bar.git")
	preCreateValidPhase2Config(t, dir)
	chdirTest(t, dir)

	// Mock GitHub API server with a counter for /repos/my-org/my-project hits.
	// Wraps newInitTestServer's mux indirectly: we only care about the
	// reachability endpoint here, so use a fresh mux that counts calls.
	var repoGetCount int32
	var otherCallCount int32
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/my-org/my-project", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			atomic.AddInt32(&otherCallCount, 1)
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&repoGetCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 1,
			"full_name": "my-org/my-project",
			"default_branch": "main",
			"private": false
		}`))
	})
	// Tripwire for any other endpoint — the spec says doctor MUST NOT call
	// permissions / has_issues / branch ref / search etc.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&otherCallCount, 1)
		http.NotFound(w, r)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	stdout, err := executeDoctorCmd(t, nil, map[string]string{
		"QUARANTINE_GITHUB_TOKEN":        "ghp_test",
		"QUARANTINE_GITHUB_API_BASE_URL": server.URL,
	})

	riteway.Assert(t, riteway.Case[int]{
		Given:    "valid config with github.owner/repo and a 200 reachability response",
		Should:   "exit with code 0",
		Actual:   exitCodeFromError(err),
		Expected: 0,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "doctor finished after a successful 200 reachability check",
		Should:   "print the M20 reachability summary on stdout",
		Actual:   strings.Contains(stdout, formatDoctorReachableSummary("my-org", "my-project")),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[bool]{
		Given:    "the git origin is a Gerrit URL but config has owner/repo",
		Should:   "not emit any 'not a GitHub URL' warning about the origin",
		Actual:   !strings.Contains(stdout, "not a GitHub URL"),
		Expected: true,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "ADR-037 mandates exactly one reachability call",
		Should:   "hit GET /repos/my-org/my-project exactly once",
		Actual:   atomic.LoadInt32(&repoGetCount),
		Expected: 1,
	})

	riteway.Assert(t, riteway.Case[int32]{
		Given:    "doctor must not introspect permissions / has_issues / branches",
		Should:   "make zero calls to any other GitHub endpoint",
		Actual:   atomic.LoadInt32(&otherCallCount),
		Expected: 0,
	})
}

// --- Pure unit test for formatDoctorReachableSummary ---
//
// The success summary text is deterministic — it depends only on the owner and
// repo strings. Extract it as a pure function so the format is unit-testable
// without needing a temp dir, env vars, or an HTTP server.

func TestFormatDoctorReachableSummaryReturnsCanonicalText(t *testing.T) {
	riteway.Assert(t, riteway.Case[string]{
		Given:    "owner='my-org' and repo='my-project'",
		Should:   "render the M20 doctor reachability summary verbatim",
		Actual:   formatDoctorReachableSummary("my-org", "my-project"),
		Expected: "github.owner:  my-org (from config)\ngithub.repo:   my-project (from config)\ntoken:         authenticated\ntarget:        reachable\n",
	})
}

