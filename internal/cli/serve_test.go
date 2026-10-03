package cli

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeStartsAndStops(t *testing.T) {
	for _, withRemote := range []bool{false, true} {
		name := "local only"
		if withRemote {
			name = "remote link"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "project"))); err != nil {
				t.Fatal(err)
			}
			config := "{}\n"
			if withRemote {
				config = "remote_url: https://example.com/experiments\n"
			}
			if err := os.WriteFile(filepath.Join(root, "expledger.yaml"), []byte(config), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", t.TempDir())
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
			body, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr != nil || strings.Contains(string(body), `class="remote-link"`) != withRemote {
				t.Fatalf("remote links: %v, %s", readErr, body)
			}
			if withRemote && !strings.Contains(string(body), "https://example.com/experiments/20260924-baseline") {
				t.Fatal("missing configured link")
			}
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

type startupOutput chan string

func (out startupOutput) Write(p []byte) (int, error) {
	out <- string(p)
	return len(p), nil
}
