package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ebisu/mugi/internal/config"
	"github.com/ebisu/mugi/internal/git"
	"github.com/ebisu/mugi/internal/remote"
)

type Task struct {
	RepoName   string
	RemoteName string
	RemoteURL  string
	RepoPath   string
	Op         remote.Operation
}

type taskState int

const (
	taskPending taskState = iota
	taskRunning
	taskSuccess
	taskFailed
)

type taskResult struct {
	task   Task
	result git.Result
}

type Model struct {
	tasks       []Task
	states      map[string]taskState
	results     map[string]git.Result
	spinner     spinner.Model
	operation   remote.Operation
	verbose     bool
	force       bool
	linear      bool
	currentTask int
	done        bool
}

func NewModel(op remote.Operation, tasks []Task, verbose, force, linear bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	states := make(map[string]taskState)

	for _, t := range tasks {
		states[taskKey(t)] = taskPending
	}

	return Model{
		tasks:     tasks,
		states:    states,
		results:   make(map[string]git.Result),
		spinner:   s,
		operation: op,
		verbose:   verbose,
		force:     force,
		linear:    linear,
	}
}

func taskKey(t Task) string {
	return t.RepoName + ":" + t.RemoteName
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}

	if m.linear {
		if len(m.tasks) > 0 {
			cmds = append(cmds, m.runTask(m.tasks[0]))
		}
	} else {
		for _, task := range m.tasks {
			cmds = append(cmds, m.runTask(task))
		}
	}

	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case taskResult:
		key := taskKey(msg.task)

		if msg.result.Error != nil {
			m.states[key] = taskFailed
		} else {
			m.states[key] = taskSuccess
		}

		m.results[key] = msg.result
		m.currentTask++

		if m.allDone() {
			m.done = true

			return m, tea.Quit
		}

		if m.linear && m.currentTask < len(m.tasks) {
			return m, m.runTask(m.tasks[m.currentTask])
		}
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render(fmt.Sprintf("%s repositories", m.operation.Verb()))

	b.WriteString(title + "\n\n")

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	for _, task := range m.tasks {
		key := taskKey(task)
		state := m.states[key]

		var status string
		switch state {
		case taskPending:
			status = dimStyle.Render("○")
		case taskRunning:
			status = m.spinner.View()
		case taskSuccess:
			status = successStyle.Render("✓")
		case taskFailed:
			status = failStyle.Render("✗")
		}

		repoName := filepath.Base(task.RepoName)
		line := fmt.Sprintf("%s %s → %s", status, repoName, task.RemoteName)

		if result, ok := m.results[key]; ok && result.Output != "" {
			if m.verbose {
				line += "\n" + indentOutput(result.Output, dimStyle)
			} else if state == taskFailed {
				line += dimStyle.Render(" " + firstLine(result.Output))
			}
		}

		b.WriteString(line + "\n")
	}

	if m.done {
		b.WriteString("\n")

		success, failed := m.summary()

		if failed > 0 {
			b.WriteString(failStyle.Render(fmt.Sprintf("%d failed", failed)))
			b.WriteString(", ")
		}

		b.WriteString(successStyle.Render(fmt.Sprintf("%d succeeded", success)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) runTask(task Task) tea.Cmd {
	return func() tea.Msg {
		op := task.Op

		if op == 0 {
			op = m.operation
		}

		result := git.Execute(context.Background(), op, task.RepoPath, task.RemoteName, m.force)

		return taskResult{task: task, result: result}
	}
}

func (m Model) allDone() bool {
	for _, state := range m.states {
		if state == taskPending || state == taskRunning {
			return false
		}
	}

	return true
}

func (m Model) summary() (success, failed int) {
	for _, state := range m.states {
		switch state {
		case taskSuccess:
			success++
		case taskFailed:
			failed++
		}
	}
	return
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx != -1 {
		return s[:idx]
	}

	return s
}

func indentOutput(s string, style lipgloss.Style) string {
	lines := strings.Split(s, "\n")

	for i, line := range lines {
		lines[i] = "    " + style.Render(line)
	}

	return strings.Join(lines, "\n")
}

func Run(op remote.Operation, tasks []Task, verbose, force, linear bool) error {
	if op == remote.Pull {
		inits := NeedsInit(tasks)
		if len(inits) > 0 {
			if err := runInit(inits, verbose); err != nil {
				return err
			}
		}
	}

	syncRemotes(tasks)

	if op == remote.Pull {
		tasks = adjustPullTasks(tasks)
	}

	model := NewModel(op, tasks, verbose, force, linear)
	p := tea.NewProgram(model)

	_, err := p.Run()

	return err
}

func syncRemotes(tasks []Task) {
	ctx := context.Background()
	seen := make(map[string]bool)

	for _, task := range tasks {
		key := task.RepoPath + ":" + task.RemoteName

		if seen[key] {
			continue
		}

		seen[key] = true

		if !git.IsRepo(task.RepoPath) {
			continue
		}

		currentURL := git.GetRemoteURL(task.RepoPath, task.RemoteName)

		if currentURL == "" {
			git.AddRemote(ctx, task.RepoPath, task.RemoteName, task.RemoteURL)
		} else if currentURL != task.RemoteURL {
			git.SetRemoteURL(ctx, task.RepoPath, task.RemoteName, task.RemoteURL)
		}
	}
}

func adjustPullTasks(tasks []Task) []Task {
	firstPerRepo := make(map[string]bool)
	result := make([]Task, len(tasks))

	for i, task := range tasks {
		result[i] = task

		if firstPerRepo[task.RepoPath] {
			result[i].Op = remote.Fetch
		} else {
			result[i].Op = remote.Pull
			firstPerRepo[task.RepoPath] = true
		}
	}

	return result
}

func runInit(inits []RepoInit, verbose bool) error {
	model := NewInitModel(inits, verbose)
	p := tea.NewProgram(model)

	m, err := p.Run()
	if err != nil {
		return err
	}

	if initModel, ok := m.(InitModel); ok {
		for _, state := range initModel.states {
			if state == taskFailed {
				return fmt.Errorf("repository initialisation failed")
			}
		}
	}

	return nil
}

func BuildTasks(cfg config.Config, repoName string, remoteNames []string) []Task {
	var tasks []Task

	repos := resolveRepos(cfg, repoName)

	for _, fullName := range repos {
		repo := cfg.Repos[fullName]
		remotes := resolveRemotes(cfg, repo, remoteNames)

		for _, remoteName := range remotes {
			if url, ok := repo.Remotes[remoteName]; ok {
				tasks = append(tasks, Task{
					RepoName:   fullName,
					RemoteName: remoteName,
					RemoteURL:  url,
					RepoPath:   repo.ExpandPath(),
				})
			}
		}
	}

	return tasks
}

type RepoInit struct {
	Name    string
	Path    string
	Remotes map[string]string
}

type InitResult struct {
	Repo    string
	Output  string
	Error   error
	Success bool
}

type initTaskResult struct {
	init   RepoInit
	result InitResult
}

type InitModel struct {
	inits   []RepoInit
	states  map[string]taskState
	results map[string]InitResult
	spinner spinner.Model
	verbose bool
	done    bool
}

func NewInitModel(inits []RepoInit, verbose bool) InitModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	states := make(map[string]taskState)

	for _, init := range inits {
		states[init.Path] = taskPending
	}

	return InitModel{
		inits:   inits,
		states:  states,
		results: make(map[string]InitResult),
		spinner: s,
		verbose: verbose,
	}
}

func (m InitModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}

	for _, init := range m.inits {
		cmds = append(cmds, m.runInit(init))
	}

	return tea.Batch(cmds...)
}

func (m InitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case initTaskResult:
		if msg.result.Success {
			m.states[msg.init.Path] = taskSuccess
		} else {
			m.states[msg.init.Path] = taskFailed
		}

		m.results[msg.init.Path] = msg.result

		if m.allDone() {
			m.done = true

			return m, tea.Quit
		}
	}

	return m, nil
}

func (m InitModel) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render("Initialising repositories")

	b.WriteString(title + "\n\n")

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	for _, init := range m.inits {
		state := m.states[init.Path]

		var status string
		switch state {
		case taskPending:
			status = dimStyle.Render("○")
		case taskRunning:
			status = m.spinner.View()
		case taskSuccess:
			status = successStyle.Render("✓")
		case taskFailed:
			status = failStyle.Render("✗")
		}

		repoName := filepath.Base(init.Name)
		line := fmt.Sprintf("%s %s", status, repoName)

		if result, ok := m.results[init.Path]; ok && result.Output != "" {
			if m.verbose || !result.Success {
				line += "\n" + indentOutput(result.Output, dimStyle)
			}
		}

		b.WriteString(line + "\n")
	}

	if m.done {
		b.WriteString("\n")
	}

	return b.String()
}

func (m *InitModel) runInit(init RepoInit) tea.Cmd {
	return func() tea.Msg {
		result := InitRepo(context.Background(), init)

		return initTaskResult{init: init, result: result}
	}
}

func (m InitModel) allDone() bool {
	for _, state := range m.states {
		if state == taskPending || state == taskRunning {
			return false
		}
	}

	return true
}

func NeedsInit(tasks []Task) []RepoInit {
	seen := make(map[string]bool)

	var inits []RepoInit

	for _, task := range tasks {
		if seen[task.RepoPath] {
			continue
		}

		seen[task.RepoPath] = true

		if _, err := os.Stat(task.RepoPath); os.IsNotExist(err) {
			inits = append(inits, collectRepoInit(tasks, task.RepoPath, task.RepoName))

			continue
		}

		if !git.IsRepo(task.RepoPath) {
			inits = append(inits, collectRepoInit(tasks, task.RepoPath, task.RepoName))
		}
	}

	return inits
}

func collectRepoInit(tasks []Task, path, name string) RepoInit {
	remotes := make(map[string]string)

	for _, t := range tasks {
		if t.RepoPath == path {
			remotes[t.RemoteName] = t.RemoteURL
		}
	}

	return RepoInit{Name: name, Path: path, Remotes: remotes}
}

func InitRepo(ctx context.Context, init RepoInit) InitResult {
	result := InitResult{Repo: init.Name}

	var firstRemote, firstURL string

	for name, url := range init.Remotes {
		firstRemote = name
		firstURL = url

		break
	}

	if err := os.MkdirAll(filepath.Dir(init.Path), 0o755); err != nil {
		result.Error = err
		result.Output = err.Error()

		return result
	}

	cloneResult := git.Clone(ctx, firstURL, init.Path)

	if cloneResult.Error != nil {
		result.Error = cloneResult.Error
		result.Output = cloneResult.Output

		return result
	}

	outputs := []string{fmt.Sprintf("Cloned from %s", firstRemote)}

	renameResult := git.RenameRemote(ctx, init.Path, "origin", firstRemote)

	if renameResult.Error != nil {
		result.Error = renameResult.Error
		result.Output = renameResult.Output

		return result
	}

	for name, url := range init.Remotes {
		if name == firstRemote {
			continue
		}

		addResult := git.AddRemote(ctx, init.Path, name, url)

		if addResult.Error != nil {
			result.Error = addResult.Error
			result.Output = addResult.Output

			return result
		}

		outputs = append(outputs, fmt.Sprintf("Added remote %s", name))
	}

	result.Success = true
	result.Output = strings.Join(outputs, "\n")

	return result
}

func resolveRepos(cfg config.Config, name string) []string {
	if name == remote.All {
		return cfg.AllRepos()
	}

	if fullName, _, ok := cfg.FindRepo(name); ok {
		return []string{fullName}
	}

	return nil
}

func resolveRemotes(cfg config.Config, repo config.Repo, names []string) []string {
	if len(names) == 1 && names[0] == remote.All {
		remotes := make([]string, 0, len(repo.Remotes))

		for name := range repo.Remotes {
			remotes = append(remotes, name)
		}

		return remotes
	}

	resolved := make([]string, 0, len(names))

	for _, name := range names {
		resolved = append(resolved, cfg.ResolveAlias(name))
	}

	return resolved
}
