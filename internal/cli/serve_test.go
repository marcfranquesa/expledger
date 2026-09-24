package cli

import (
	"context"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestGitHubRepositoryURL(t *testing.T) {
	for _, remote := range []string{
		"git@github.com:owner/repo.git",
		"ssh://git@github.com/owner/repo.git",
		"https://github.com/owner/repo.git",
		"https://github.com/owner/repo/",
		"https://username:password@github.com/owner/repo.git",
	} {
		t.Run(remote, func(t *testing.T) {
			got, err := githubRepositoryURL(remote)
			if err != nil || got != "https://github.com/owner/repo" {
				t.Fatalf("GitHub URL = %q, %v", got, err)
			}
		})
	}
	for _, remote := range []string{
		"https://gitlab.com/owner/repo.git", "/tmp/repository", "file:///tmp/repository",
		"https://github.com/owner", "https://github.com/owner/repo/extra",
		"https://github.com/owner/..", "https://github.com/owner/.git",
		"https://username:secret@github.com/owner/repo?token=secret",
	} {
		t.Run(remote, func(t *testing.T) {
			if got, err := githubRepositoryURL(remote); err == nil {
				t.Fatalf("unsupported remote accepted: %q", got)
			} else if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error includes credentials: %v", err)
			}
		})
	}
}

func TestServeStartsAndStops(t *testing.T) {
	for _, withRemote := range []bool{false, true} {
		name := "local only"
		if withRemote {
			name = "GitHub remote"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			serveGit(t, root, "init", "--quiet", "--initial-branch=research/v2")
			if withRemote {
				serveGit(t, root, "remote", "add", "origin", "git@github.com:owner/repo.git")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			output := make(startupOutput, 2)
			finished := make(chan error, 1)
			go func() { finished <- Run(ctx, []string{"serve", "--port=0"}, root, time.Time{}, Streams{Out: output}) }()
			var address string
			select {
			case text := <-output:
				address = strings.TrimPrefix(strings.Split(text, "\n")[0], "Serving experiments at ")
			case err := <-finished:
				t.Fatalf("server stopped before startup: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("server did not report its address")
			}
			if !strings.HasPrefix(address, "http://127.0.0.1:") {
				t.Fatalf("server address = %q", address)
			}
			client := &http.Client{Timeout: 5 * time.Second}
			response, err := client.Get(address)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("GET status = %d", response.StatusCode)
			}
			cancel()
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("server did not stop after cancellation")
			}
			connection, err := net.DialTimeout("tcp", strings.TrimPrefix(address, "http://"), time.Second)
			if err == nil {
				connection.Close()
				t.Fatal("server listener remains open")
			}
		})
	}
}

func TestGitHubLocationOptional(t *testing.T) {
	root := t.TempDir()
	serveGit(t, root, "init", "--quiet", "--initial-branch=research/v2")
	if repositoryURL, branch, err := githubLocation(context.Background(), root); err != nil || repositoryURL != "" || branch != "research/v2" {
		t.Fatalf("local location = %q, %q, %v", repositoryURL, branch, err)
	}
	serveGit(t, root, "remote", "add", "origin", "https://gitlab.com/owner/repo.git")
	if repositoryURL, branch, err := githubLocation(context.Background(), root); err != nil || repositoryURL != "" || branch != "research/v2" {
		t.Fatalf("non-GitHub location = %q, %q, %v", repositoryURL, branch, err)
	}
	serveGit(t, root, "remote", "set-url", "origin", "git@github.com:owner/repo.git")
	repositoryURL, branch, err := githubLocation(context.Background(), root)
	if err != nil || repositoryURL != "https://github.com/owner/repo" || branch != "research/v2" {
		t.Fatalf("GitHub location = %q, %q, %v", repositoryURL, branch, err)
	}
	serveGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "initial")
	serveGit(t, root, "checkout", "--quiet", "--detach", "HEAD")
	if repositoryURL, branch, err := githubLocation(context.Background(), root); err != nil || repositoryURL != "" || branch != "" {
		t.Fatalf("detached location = %q, %q, %v", repositoryURL, branch, err)
	}
	if _, _, err := githubLocation(context.Background(), t.TempDir()); err == nil {
		t.Fatal("Git discovery errors must not be hidden")
	}
}

type startupOutput chan string

func (out startupOutput) Write(p []byte) (int, error) {
	out <- string(p)
	return len(p), nil
}

func serveGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
