package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type RemoteDefinition struct {
	Aliases []string `yaml:"aliases"`
	URL     string   `yaml:"url"`
}

type OperationDefaults struct {
	Remotes []string `yaml:"remotes"`
}

type Defaults struct {
	Remotes    []string          `yaml:"remotes"`
	PathPrefix string            `yaml:"path_prefix"`
	Verbose    bool              `yaml:"verbose"`
	Linear     bool              `yaml:"linear"`
	Pull       OperationDefaults `yaml:"pull"`
	Push       OperationDefaults `yaml:"push"`
	Fetch      OperationDefaults `yaml:"fetch"`
}

type RepoRemotes map[string]string

type Repo struct {
	Path    string
	Remotes RepoRemotes
}

type Config struct {
	Remotes  map[string]RemoteDefinition
	Defaults Defaults
	Repos    map[string]Repo
}

type rawConfig struct {
	Remotes  map[string]RemoteDefinition `yaml:"remotes"`
	Defaults Defaults                    `yaml:"defaults"`
	Repos    map[string]yaml.Node        `yaml:"repos"`
}

type remoteOverride struct {
	User string `yaml:"user"`
	Repo string `yaml:"repo"`
}

func Load(override string) (Config, error) {
	path := override

	if path == "" {
		var err error

		path, err = Path()
		if err != nil {
			return Config{}, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var raw rawConfig

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, err
	}

	return expand(raw)
}

func expand(raw rawConfig) (Config, error) {
	cfg := Config{
		Remotes:  raw.Remotes,
		Defaults: raw.Defaults,
		Repos:    make(map[string]Repo),
	}

	for name, node := range raw.Repos {
		repo, err := expandRepo(name, node, raw)
		if err != nil {
			return Config{}, err
		}

		cfg.Repos[name] = repo
	}

	return cfg, nil
}

func expandRepo(name string, node yaml.Node, raw rawConfig) (Repo, error) {
	user, repoName := splitRepoName(name)
	repo := Repo{
		Remotes: make(RepoRemotes),
	}

	var parsed map[string]yaml.Node

	if err := node.Decode(&parsed); err != nil {
		parsed = make(map[string]yaml.Node)
	}

	if pathNode, ok := parsed["path"]; ok {
		var path string

		pathNode.Decode(&path)

		repo.Path = path
	} else if raw.Defaults.PathPrefix != "" {
		repo.Path = filepath.Join(raw.Defaults.PathPrefix, repoName)
	}

	remoteList := raw.Defaults.Remotes

	if remotesNode, ok := parsed["remotes"]; ok {
		var list []string

		if err := remotesNode.Decode(&list); err == nil {
			remoteList = list
		} else {
			var oldStyle map[string]string

			if err := remotesNode.Decode(&oldStyle); err == nil {
				repo.Remotes = oldStyle

				return repo, nil
			}
		}
	}

	for _, remoteName := range remoteList {
		remoteUser, remoteRepo := user, repoName

		if overrideNode, ok := parsed[remoteName]; ok {
			var override remoteOverride

			if err := overrideNode.Decode(&override); err == nil {
				if override.User != "" {
					remoteUser = override.User
				}

				if override.Repo != "" {
					remoteRepo = override.Repo
				}
			} else {
				var urlOverride string

				if err := overrideNode.Decode(&urlOverride); err == nil {
					repo.Remotes[remoteName] = urlOverride

					continue
				}
			}
		}

		if def, ok := raw.Remotes[remoteName]; ok && def.URL != "" {
			url := expandURL(def.URL, remoteUser, remoteRepo)

			repo.Remotes[remoteName] = url
		}
	}

	return repo, nil
}

func expandURL(template, user, repo string) string {
	url := strings.ReplaceAll(template, "${user}", user)
	url = strings.ReplaceAll(url, "${repo}", repo)

	return url
}

func splitRepoName(name string) (user, repo string) {
	parts := strings.SplitN(name, "/", 2)

	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return "", parts[0]
}

func Path() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")

	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		configDir = filepath.Join(home, ".config")
	}

	return filepath.Join(configDir, "mugi", "config.yaml"), nil
}

func (c Config) FindRepo(name string) (string, Repo, bool) {
	if name == "." {
		return c.FindRepoByPath(".")
	}

	if repo, ok := c.Repos[name]; ok {
		return name, repo, true
	}

	var matches []string

	for fullName := range c.Repos {
		repoName := filepath.Base(fullName)

		if repoName == name {
			matches = append(matches, fullName)
		}
	}

	if len(matches) == 1 {
		return matches[0], c.Repos[matches[0]], true
	}

	return "", Repo{}, false
}

func (c Config) AllRepos() []string {
	repos := make([]string, 0, len(c.Repos))

	for name := range c.Repos {
		repos = append(repos, name)
	}

	return repos
}

func (c Config) ResolveAlias(alias string) string {
	for name, def := range c.Remotes {
		if name == alias || slices.Contains(def.Aliases, alias) {
			return name
		}
	}

	return alias
}

func (r Repo) ExpandPath() string {
	path := r.Path

	if len(path) > 0 && path[0] == '~' {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[1:])
		}
	}

	return path
}

func (d Defaults) RemotesFor(operation string) []string {
	switch operation {
	case "pull":
		if len(d.Pull.Remotes) > 0 {
			return d.Pull.Remotes
		}
	case "push":
		if len(d.Push.Remotes) > 0 {
			return d.Push.Remotes
		}
	case "fetch":
		if len(d.Fetch.Remotes) > 0 {
			return d.Fetch.Remotes
		}
	}

	return d.Remotes
}

func (c Config) FindRepoByPath(path string) (string, Repo, bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", Repo{}, false
	}

	for name, repo := range c.Repos {
		repoPath := repo.ExpandPath()

		absRepoPath, err := filepath.Abs(repoPath)
		if err != nil {
			continue
		}

		if absPath == absRepoPath {
			return name, repo, true
		}
	}

	return "", Repo{}, false
}
