package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/model"
)

// isolateGit returns an environment in which git reads no configuration of
// the machine it runs on: the user's global file, the system file and any
// hook can only make a push behave differently from the next machine.
func isolateGit(t *testing.T, home string) []string {
	t.Helper()
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		// 禁止模板 hook 和凭据 helper 影响测试或触及用户凭据。
		"GIT_TEMPLATE_DIR=" + t.TempDir(),
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=core.hooksPath",
		"GIT_CONFIG_VALUE_0=" + t.TempDir(),
		"GIT_CONFIG_KEY_1=credential.helper",
		"GIT_CONFIG_VALUE_1=",
		"GIT_TERMINAL_PROMPT=0",
		// No credential helper and no askpass: a push must never wait on
		// a prompt in a test.
		"GIT_ASKPASS=",
		"GIT_AUTHOR_NAME=jumpboard",
		"GIT_AUTHOR_EMAIL=jumpboard@example.test",
		"GIT_COMMITTER_NAME=jumpboard",
		"GIT_COMMITTER_EMAIL=jumpboard@example.test",
		"LC_ALL=C",
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		env = append(env, "TMPDIR="+tmp)
	}
	return env
}

// git runs one git command in env and fails the test on a non-zero exit.
func git(t *testing.T, env []string, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(env, dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// runGit runs git and returns its combined output and error.
func runGit(env []string, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = dir
	cmd.Env = env
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// gitBackendRequest records the request before any CGI adaptation.
type gitBackendRequest struct {
	host             string
	method           string
	path             string
	transferEncoding []string
	contentLength    int64
}

// gitHTTPBackend enforces the test virtual host before invoking git. This
// local smart-HTTP fixture is not a GitLab reproduction or proof of the
// reported deployment's root cause.
func gitHTTPBackend(t *testing.T, projectRoot string, env []string, expectedHost string) (http.Handler, func() []gitBackendRequest) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	backend := &cgi.Handler{
		Path: gitPath,
		Args: []string{"http-backend"},
		Env: append([]string{
			"GIT_PROJECT_ROOT=" + projectRoot,
			"GIT_HTTP_EXPORT_ALL=1",
		}, env...),
		InheritEnv: nil,
	}

	var mu sync.Mutex
	var requests []gitBackendRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, gitBackendRequest{
			host: r.Host, method: r.Method, path: r.URL.Path,
			transferEncoding: append([]string(nil), r.TransferEncoding...),
			contentLength:    r.ContentLength,
		})
		mu.Unlock()
		if r.Host != expectedHost {
			http.Error(w, "unexpected Host: "+r.Host, http.StatusMisdirectedRequest)
			return
		}
		// CGI requires CONTENT_LENGTH. Go's HTTP server has already decoded
		// chunk framing; buffer only here, after recording the real request,
		// to adapt an unknown-length body to CGI. The production proxy keeps
		// streaming. This fixture does not exercise GitLab's HTTP server.
		if r.ContentLength < 0 {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			r.TransferEncoding = nil
		}
		backend.ServeHTTP(w, r)
	})
	snapshot := func() []gitBackendRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]gitBackendRequest(nil), requests...)
	}
	return handler, snapshot
}

// TestGitPushThroughProxyReachesLocalGitHTTPBackend checks real Git pushes
// against a strict local virtual host, including streamed packs. Git and its
// subprocesses read isolated configuration. postBuffer selects the test's
// transport shape; increasing it is not a proxy fix or user workaround.
func TestGitPushThroughProxyReachesLocalGitHTTPBackend(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	for _, tc := range []struct {
		name       string
		size       int
		postBuffer string
		chunked    bool
		auth       bool
	}{
		{name: "small", size: 4096, postBuffer: "10485760"},
		{name: "small_auth_challenge", size: 4096, postBuffer: "10485760", auth: true},
		{name: "large_chunked", size: 2 << 20, postBuffer: "1024", chunked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			env := isolateGit(t, root)
			bareRoot := filepath.Join(root, "srv")
			bare := filepath.Join(bareRoot, "repo.git")
			if err := os.MkdirAll(bare, 0o755); err != nil {
				t.Fatal(err)
			}
			git(t, env, root, "init", "--quiet", "--bare", bare)
			git(t, env, bare, "config", "http.receivepack", "true")

			work := filepath.Join(root, "work")
			if err := os.MkdirAll(work, 0o755); err != nil {
				t.Fatal(err)
			}
			git(t, env, root, "init", "--quiet", work)
			git(t, env, work, "checkout", "--quiet", "-b", "main")
			writeFile(t, filepath.Join(work, "README.md"), "# jumpboard push test\n")
			git(t, env, work, "add", "README.md")
			git(t, env, work, "commit", "--quiet", "-m", "first")

			// Random bytes keep the large pack incompressible and above 1 MiB.
			payload := make([]byte, tc.size)
			if _, err := rand.Read(payload); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(work, "blob.bin"), string(payload))
			git(t, env, work, "add", "blob.bin")
			git(t, env, work, "commit", "--quiet", "-m", "payload")

			const clientHost = "gitlab.example.test"
			upstream, requestsSeen := gitHTTPBackend(t, bareRoot, env, clientHost)
			var authMu sync.Mutex
			var challenges, authenticated int
			if tc.auth {
				backend := upstream
				upstream = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					user, password, ok := r.BasicAuth()
					authMu.Lock()
					if !ok || user != "test-user" || password != "test-pass" {
						challenges++
						authMu.Unlock()
						w.Header().Set("WWW-Authenticate", `Basic realm="local-test"`)
						http.Error(w, "test authentication required", http.StatusUnauthorized)
						return
					}
					authenticated++
					authMu.Unlock()
					backend.ServeHTTP(w, r)
				})
			}
			up := httptest.NewServer(upstream)
			defer up.Close()
			listen := proxyWirePort(t)
			e := NewLocal()
			defer e.Stop()
			if _, err := e.Apply(Spec{Mappings: []model.Mapping{{
				ID: "git", Name: "git", Protocol: model.HTTP, Enabled: true,
				ListenPort: listen, UpstreamHost: "127.0.0.1", UpstreamPort: portOfURL(t, up.URL),
				ListenPath: "/", ProxyMode: model.ProxyStandard,
			}}}); err != nil {
				t.Fatalf("apply: %v", err)
			}

			remote := fmt.Sprintf("http://127.0.0.1:%d/repo.git", listen)
			if tc.auth {
				// URL 仅包含固定测试凭据，且请求只发往本地临时服务。
				remote = fmt.Sprintf("http://test-user:test-pass@127.0.0.1:%d/repo.git", listen)
			}
			out, err := runGit(env, work,
				"-c", "http.extraHeader=Host: "+clientHost,
				"-c", "http.postBuffer="+tc.postBuffer,
				"push", remote, "main")
			if err != nil {
				t.Fatalf("git push through the proxy failed: %v\n%s", err, out)
			}
			t.Logf("git push succeeded:\n%s", out)
			if tc.auth {
				authMu.Lock()
				gotChallenges, gotAuthenticated := challenges, authenticated
				authMu.Unlock()
				if gotChallenges == 0 || gotAuthenticated == 0 {
					t.Fatalf("401 challenges=%d authenticated requests=%d, want both positive", gotChallenges, gotAuthenticated)
				}
				t.Logf("verified 401 retry: challenges=%d authenticated requests=%d", gotChallenges, gotAuthenticated)
			}

			wantCommit := strings.TrimSpace(git(t, env, work, "rev-parse", "HEAD"))
			gotCommit := strings.TrimSpace(git(t, env, bare, "rev-parse", "refs/heads/main"))
			if gotCommit != wantCommit {
				t.Fatalf("refs/heads/main = %q, want %q", gotCommit, wantCommit)
			}
			git(t, env, bare, "cat-file", "-e", gotCommit+"^{commit}")
			gotBlob := git(t, env, bare, "cat-file", "blob", gotCommit+":blob.bin")
			if !bytes.Equal([]byte(gotBlob), payload) {
				t.Fatalf("backend blob differs from %d-byte source payload", len(payload))
			}

			var posts int
			var sawChunked bool
			for _, r := range requestsSeen() {
				if r.host != clientHost {
					t.Errorf("upstream Host = %q, want %q", r.host, clientHost)
				}
				if r.method != http.MethodPost || r.path != "/repo.git/git-receive-pack" {
					continue
				}
				posts++
				chunked := len(r.transferEncoding) == 1 && r.transferEncoding[0] == "chunked"
				sawChunked = sawChunked || chunked
				t.Logf("receive-pack POST Host=%q TransferEncoding=%v ContentLength=%d", r.host, r.transferEncoding, r.contentLength)
				if chunked && r.contentLength != -1 {
					t.Errorf("chunked POST ContentLength = %d, want -1", r.contentLength)
				}
				if !tc.chunked && (chunked || r.contentLength <= 0) {
					t.Errorf("small POST must have known positive length and no chunked encoding")
				}
			}
			if posts == 0 || sawChunked != tc.chunked {
				t.Errorf("receive-pack POSTs=%d, chunked=%v, want at least one POST and chunked=%v", posts, sawChunked, tc.chunked)
			}
			t.Logf("verified backend ref, commit and %d-byte blob", len(payload))
		})
	}
}

// writeFile writes content, failing the test if it cannot.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
