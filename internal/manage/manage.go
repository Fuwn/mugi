package manage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ebisu/mugi/internal/config"
	"gopkg.in/yaml.v3"
)

type RepoInfo struct {
	Name    string
	Path    string
	Remotes map[string]string
}

func Add(path, configPath string, remoteDefs map[string]config.RemoteDefinition) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if !isGitRepo(absPath) {
		return fmt.Errorf("not a git repository: %s", absPath)
	}

	info, err := extractRepoInfo(absPath, remoteDefs)
	if err != nil {
		return err
	}

	return appendToConfig(configPath, info)
}

func Remove(name, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	fullName, _, found := cfg.FindRepo(name)
	if !found {
		return fmt.Errorf("repository not found: %s", name)
	}

	return removeFromConfig(configPath, fullName)
}

func List(configPath string) ([]RepoInfo, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	var repos []RepoInfo

	for name, repo := range cfg.Repos {
		repos = append(repos, RepoInfo{
			Name:    name,
			Path:    repo.ExpandPath(),
			Remotes: repo.Remotes,
		})
	}

	return repos, nil
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path

	return cmd.Run() == nil
}

func extractRepoInfo(path string, remoteDefs map[string]config.RemoteDefinition) (RepoInfo, error) {
	info := RepoInfo{
		Path:    path,
		Remotes: make(map[string]string),
	}

	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = path

	out, err := cmd.Output()
	if err != nil {
		return info, fmt.Errorf("failed to get remotes: %w", err)
	}

	remoteURLs := parseRemotes(string(out))

	for remoteName, url := range remoteURLs {
		knownRemote := matchRemoteURL(url, remoteDefs)

		if knownRemote != "" {
			info.Remotes[knownRemote] = url
		} else {
			info.Remotes[remoteName] = url
		}
	}

	info.Name = inferRepoName(path, remoteURLs)

	return info, nil
}

func parseRemotes(output string) map[string]string {
	remotes := make(map[string]string)

	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, "(fetch)") {
			continue
		}

		parts := strings.Fields(line)

		if len(parts) >= 2 {
			remotes[parts[0]] = parts[1]
		}
	}

	return remotes
}

func matchRemoteURL(url string, remoteDefs map[string]config.RemoteDefinition) string {
	for name, def := range remoteDefs {
		template := def.URL

		if template == "" {
			continue
		}

		pattern := strings.ReplaceAll(template, "${user}", "")
		pattern = strings.ReplaceAll(pattern, "${repo}", "")

		base := strings.Split(pattern, ":")[0]

		if strings.Contains(url, base) || strings.Contains(url, name) {
			return name
		}
	}

	return ""
}

func inferRepoName(path string, remotes map[string]string) string {
	for _, url := range remotes {
		name := extractRepoNameFromURL(url)

		if name != "" {
			return name
		}
	}

	return filepath.Base(path)
}

func extractRepoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")

	if strings.Contains(url, ":") {
		parts := strings.Split(url, ":")

		if len(parts) == 2 {
			return strings.TrimPrefix(parts[1], "~")
		}
	}

	if strings.Contains(url, "/") {
		parts := strings.Split(url, "/")

		if len(parts) >= 2 {
			return parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
	}

	return ""
}

func appendToConfig(configPath string, info RepoInfo) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var raw map[string]yaml.Node

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	reposNode, ok := raw["repos"]
	if !ok {
		return fmt.Errorf("repos section not found in config")
	}

	repoEntry := map[string]any{
		"path":    info.Path,
		"remotes": info.Remotes,
	}

	entryBytes, err := yaml.Marshal(map[string]any{info.Name: repoEntry})
	if err != nil {
		return err
	}

	var entryNode yaml.Node

	if err := yaml.Unmarshal(entryBytes, &entryNode); err != nil {
		return err
	}

	if reposNode.Kind == yaml.MappingNode && len(entryNode.Content) > 0 && len(entryNode.Content[0].Content) >= 2 {
		reposNode.Content = append(reposNode.Content, entryNode.Content[0].Content...)
		raw["repos"] = reposNode
	}

	output, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, output, 0o644)
}

func removeFromConfig(configPath, name string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var raw map[string]yaml.Node

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	reposNode, ok := raw["repos"]
	if !ok {
		return fmt.Errorf("repos section not found in config")
	}

	if reposNode.Kind != yaml.MappingNode {
		return fmt.Errorf("repos section is not a mapping")
	}

	var newContent []*yaml.Node

	for i := 0; i < len(reposNode.Content); i += 2 {
		if i+1 >= len(reposNode.Content) {
			break
		}

		if reposNode.Content[i].Value != name {
			newContent = append(newContent, reposNode.Content[i], reposNode.Content[i+1])
		}
	}

	reposNode.Content = newContent
	raw["repos"] = reposNode

	output, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, output, 0o644)
}
