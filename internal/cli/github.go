package cli

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

func githubLocation(root string) (repositoryURL, branch string, err error) {
	remote, err := gitText(root, "remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("serve requires an origin remote pointing to GitHub: %w", err)
	}
	repositoryURL, err = githubRepositoryURL(remote)
	if err != nil {
		return "", "", err
	}
	branch, err = gitText(root, "branch", "--show-current")
	if err != nil {
		return "", "", fmt.Errorf("read current Git branch: %w", err)
	}
	if branch == "" {
		return "", "", errors.New("serve requires a checked-out branch; switch to a branch and try again")
	}
	return repositoryURL, branch, nil
}

func gitText(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

func githubRepositoryURL(remote string) (string, error) {
	if host, path, ok := strings.Cut(remote, ":"); ok && strings.EqualFold(host, "git@github.com") {
		remote = "ssh://git@github.com/" + path
	}
	u, err := url.Parse(remote)
	if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") || !strings.EqualFold(u.Hostname(), "github.com") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("origin must be a GitHub HTTPS or SSH remote")
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
	if len(parts) != 2 {
		return "", errors.New("origin must point to a GitHub repository (owner/repository)")
	}
	parts[1] = strings.TrimSuffix(parts[1], ".git")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("origin must point to a GitHub repository (owner/repository)")
		}
	}
	return "https://github.com/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]), nil
}
