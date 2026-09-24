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

func TestServeHelpAndArguments(t *testing.T) {
	for _, args := range [][]string{{"serve", "--help"}, {"help", "serve"}} {
		root := t.TempDir()
		t.Setenv("PATH", t.TempDir())
		var output strings.Builder
		if err := Run(args, root, time.Now(), &output); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"--port", "127.0.0.1", "Ctrl+C"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("help missing %q: %s", want, output.String())
			}
		}
	}
	for _, args := range [][]string{{"serve", "extra"}, {"serve", "--port=-1"}, {"serve", "--port=65536"}} {
		var output strings.Builder
		err := Run(args, t.TempDir(), time.Now(), &output)
		if err == nil || strings.Contains(err.Error(), "Git") {
			t.Fatalf("expected argument error before Git lookup for %v, got %v", args, err)
		}
	}
}

func TestServeStartsAndStops(t *testing.T) {
	root := t.TempDir()
	serveGit(t, root, "init", "--quiet", "--initial-branch=research/v2")
	serveGit(t, root, "remote", "add", "origin", "git@github.com:owner/repo.git")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := serveExperimentsCommand(&application{repoRoot: root})
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--port=0"})
	output := make(startupOutput, 2)
	cmd.SetOut(output)
	finished := make(chan error, 1)
	go func() { finished <- cmd.Execute() }()
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
}

func TestGitHubLocationRequiresOriginAndBranch(t *testing.T) {
	root := t.TempDir()
	serveGit(t, root, "init", "--quiet", "--initial-branch=research/v2")
	if _, _, err := githubLocation(root); err == nil || !strings.Contains(err.Error(), "origin remote") {
		t.Fatalf("missing origin error = %v", err)
	}
	serveGit(t, root, "remote", "add", "origin", "git@github.com:owner/repo.git")
	remote, branch, err := githubLocation(root)
	if err != nil || remote != "https://github.com/owner/repo" || branch != "research/v2" {
		t.Fatalf("GitHub location = %q, %q, %v", remote, branch, err)
	}
	serveGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "initial")
	serveGit(t, root, "checkout", "--quiet", "--detach", "HEAD")
	if _, _, err := githubLocation(root); err == nil || !strings.Contains(err.Error(), "checked-out branch") {
		t.Fatalf("detached HEAD error = %v", err)
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
