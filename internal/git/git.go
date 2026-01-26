package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/ebisu/mugi/internal/remote"
)

const sshEnv = "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new"

func gitEnv() []string {
	return append(os.Environ(), sshEnv)
}

type Result struct {
	Repo     string
	Remote   string
	Output   string
	Error    error
	ExitCode int
}

func (r *Result) setError(err error) {
	r.Error = err

	var exitErr *exec.ExitError

	if errors.As(err, &exitErr) {
		r.ExitCode = exitErr.ExitCode()
	} else {
		r.ExitCode = 1
	}

	if r.Output == "" {
		r.Output = err.Error()
	}
}

func Execute(ctx context.Context, op remote.Operation, repoPath, remoteName string, force bool) Result {
	result := Result{
		Repo:   repoPath,
		Remote: remoteName,
	}

	args := buildArgs(op, remoteName, repoPath, force)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	cmd.Env = gitEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = strings.TrimSpace(stdout.String() + stderr.String())

	if err != nil {
		result.setError(err)
	}

	return result
}

func buildArgs(op remote.Operation, remoteName, repoPath string, force bool) []string {
	switch op {
	case remote.Pull:
		branch := currentBranch(repoPath)
		if branch == "" {
			branch = "HEAD"
		}

		return []string{"pull", remoteName, branch}
	case remote.Push:
		if force {
			return []string{"push", "--force", remoteName}
		}

		return []string{"push", remoteName}
	case remote.Fetch:
		return []string{"fetch", remoteName}
	default:
		return []string{}
	}
}

func currentBranch(repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

func IsRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path

	return cmd.Run() == nil
}

func Clone(ctx context.Context, url, path string) Result {
	result := Result{
		Repo:   path,
		Remote: "origin",
	}

	cmd := exec.CommandContext(ctx, "git", "clone", url, path)
	cmd.Env = gitEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = strings.TrimSpace(stdout.String() + stderr.String())

	if err != nil {
		result.setError(err)
	}

	return result
}

func AddRemote(ctx context.Context, repoPath, name, url string) Result {
	result := Result{
		Repo:   repoPath,
		Remote: name,
	}

	cmd := exec.CommandContext(ctx, "git", "remote", "add", name, url)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = strings.TrimSpace(stdout.String() + stderr.String())

	if err != nil {
		result.setError(err)
	}

	return result
}

func RenameRemote(ctx context.Context, repoPath, oldName, newName string) Result {
	result := Result{
		Repo:   repoPath,
		Remote: newName,
	}

	cmd := exec.CommandContext(ctx, "git", "remote", "rename", oldName, newName)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = strings.TrimSpace(stdout.String() + stderr.String())

	if err != nil {
		result.setError(err)
	}

	return result
}

func HasRemote(repoPath, name string) bool {
	cmd := exec.Command("git", "remote", "get-url", name)
	cmd.Dir = repoPath

	return cmd.Run() == nil
}

func SetRemoteURL(ctx context.Context, repoPath, name, url string) Result {
	result := Result{
		Repo:   repoPath,
		Remote: name,
	}

	cmd := exec.CommandContext(ctx, "git", "remote", "set-url", name, url)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = strings.TrimSpace(stdout.String() + stderr.String())

	if err != nil {
		result.setError(err)
	}

	return result
}

func GetRemoteURL(repoPath, name string) string {
	cmd := exec.Command("git", "remote", "get-url", name)
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}
