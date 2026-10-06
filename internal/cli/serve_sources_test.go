package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeCLIOverridesPersistAcrossPolls(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	other := filepath.Join(base, "other")
	sourceProject(t, root, "sources: [../missing]\nexperiments_dir: local/trials\nremote_url: https://local.example/experiments")
	sourceProject(t, other, "experiments_dir: remote/trials\nremote_url: https://other.example/experiments")
	sourceRecordIn(t, root, "local/trials", "2026-10-06/same", "Local winner", "")
	sourceRecordIn(t, other, "remote/trials", "2026-10-06/same", "Other loser", "")
	sourceRecordIn(t, other, "remote/trials", "2026-10-06/unique", "Unique other", "")
	nested := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	output := make(startupOutput, 2)
	finished := make(chan error, 1)
	go func() {
		finished <- Run(ctx, []string{"serve", "--source=.", "--source=../other", "--port=0"}, nested, time.Time{}, Streams{Out: output})
	}()
	defer func() {
		cancel()
		select {
		case err := <-finished:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("serve did not stop")
		}
	}()
	var address string
	select {
	case text := <-output:
		address = strings.TrimPrefix(strings.Split(text, "\n")[0], "Serving experiments at ")
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not start")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	poll := func(path string) (int, string) {
		t.Helper()
		response, err := client.Get(address + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, string(body)
	}
	assertSnapshot := func(path, localURL string) {
		t.Helper()
		status, body := poll(path)
		if status != 200 || !strings.Contains(body, "Local winner") || strings.Contains(body, "Other loser") || !strings.Contains(body, "Unique other") || !strings.Contains(body, localURL+"/2026-10-06/same") || !strings.Contains(body, "https://other.example/experiments/2026-10-06/unique") {
			t.Fatalf("snapshot %s: status %d, %s", path, status, body)
		}
	}
	assertSnapshot("/", "https://local.example/experiments")
	sourceProject(t, root, "sources: [../other]\nexperiments_dir: local/trials\nremote_url: https://changed.example/experiments")
	assertSnapshot("/snapshot", "https://changed.example/experiments")
	sourceWrite(t, filepath.Join(other, "remote", "trials", "2026-10-06", "same", "experiment.yaml"), "bad record")
	status, body := poll("/snapshot")
	if status != 500 || strings.Contains(body, "Local winner") {
		t.Fatalf("partial snapshot on failure: %d %s", status, body)
	}
	sourceRecordIn(t, other, "remote/trials", "2026-10-06/same", "Other loser", "")
	assertSnapshot("/snapshot", "https://changed.example/experiments")
}
