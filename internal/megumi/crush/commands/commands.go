package commands

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/crush/agent/tools/mcp"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/crush/config"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/crush/home"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/crush/skills"
)

var namedArgPattern = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)

const (
	userCommandPrefix    = "user:"
	projectCommandPrefix = "project:"
)

// Argument represents a command argument with its metadata.
type Argument struct {
	ID          string
	Title       string
	Description string
	Required    bool
}

// MCPPrompt represents a custom command loaded from an MCP server.
type MCPPrompt struct {
	ID          string
	Title       string
	Description string
	PromptID    string
	ClientID    string
	Arguments   []Argument
}

// CustomCommand represents a user-defined custom command loaded from markdown files.
type CustomCommand struct {
	ID        string
	Name      string
	Content   string
	Arguments []Argument
	// Skill is set when this command represents a user-invocable skill
	Skill *skills.Skill
}

type commandSource struct {
	path   string
	prefix string
}

// LoadCustomCommands loads custom commands from multiple sources including
// XDG config directory, home directory, and project directory.
func LoadCustomCommands(cfg *config.Config) ([]CustomCommand, error) {
	return loadAll(buildCommandSources(cfg))
}

// FromSkillCatalog converts user-invocable catalog entries into custom
// command entries for the command palette.
func FromSkillCatalog(entries []skills.CatalogEntry) []CustomCommand {
	commands := make([]CustomCommand, 0, len(entries))
	for _, entry := range entries {
		if !entry.UserInvocable {
			continue
		}
		name := entry.Label
		if name == "" {
			name = userCommandPrefix + entry.Name
		}
		commands = append(commands, CustomCommand{
			ID:   name,
			Name: name,
			Skill: &skills.Skill{
				Name:          entry.Name,
				Description:   entry.Description,
				SkillFilePath: entry.ID,
			},
		})
	}
	return commands
}

// LoadMCPPrompts loads custom commands from available MCP servers.
func LoadMCPPrompts() ([]MCPPrompt, error) {
	var commands []MCPPrompt
	for mcpName, prompts := range mcp.Prompts() {
		for _, prompt := range prompts {
			key := mcpName + ":" + prompt.Name
			var args []Argument
			for _, arg := range prompt.Arguments {
				title := arg.Title
				if title == "" {
					title = arg.Name
				}
				args = append(args, Argument{
					ID:          arg.Name,
					Title:       title,
					Description: arg.Description,
					Required:    arg.Required,
				})
			}
			commands = append(commands, MCPPrompt{
				ID:          key,
				Title:       prompt.Title,
				Description: prompt.Description,
				PromptID:    prompt.Name,
				ClientID:    mcpName,
				Arguments:   args,
			})
		}
	}
	return commands, nil
}

func buildCommandSources(cfg *config.Config) []commandSource {
	candidates := []commandSource{
		// User/global commands live alongside the global config file:
		//   Megumi:        ~/.mgm/megumi/commands
		//   upstream Crush: ~/.config/crush/commands
		{
			path:   filepath.Join(filepath.Dir(config.GlobalConfig()), "commands"),
			prefix: userCommandPrefix,
		},
		// Legacy ~/.crush/commands, kept for backward compatibility.
		{
			path:   filepath.Join(home.Dir(), ".crush", "commands"),
			prefix: userCommandPrefix,
		},
		// Data-directory commands (under Megumi this is the global
		// ~/.mgm/megumi store; in upstream Crush it is the project .crush dir).
		{
			path:   filepath.Join(cfg.Options.DataDirectory, "commands"),
			prefix: projectCommandPrefix,
		},
	}
	// Project-local commands relative to the working directory, matching
	// Claude Code's per-repo .<tool>/commands convention (.megumi preferred,
	// .crush kept for compatibility).
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			commandSource{path: filepath.Join(wd, ".megumi", "commands"), prefix: projectCommandPrefix},
			commandSource{path: filepath.Join(wd, ".crush", "commands"), prefix: projectCommandPrefix},
		)
	}

	// De-duplicate by path: under Megumi the global-config dir and the data
	// directory resolve to the same place, and the data dir may coincide with a
	// project-local dir in upstream Crush. Keep the first occurrence so a
	// directory's commands aren't loaded twice under different prefixes.
	seen := make(map[string]bool, len(candidates))
	sources := make([]commandSource, 0, len(candidates))
	for _, s := range candidates {
		if s.path == "" || seen[s.path] {
			continue
		}
		seen[s.path] = true
		sources = append(sources, s)
	}
	return sources
}

func loadAll(sources []commandSource) ([]CustomCommand, error) {
	var commands []CustomCommand

	for _, source := range sources {
		if cmds, err := loadFromSource(source); err == nil {
			commands = append(commands, cmds...)
		}
	}

	return commands, nil
}

func loadFromSource(source commandSource) ([]CustomCommand, error) {
	if _, err := os.Stat(source.path); os.IsNotExist(err) {
		return nil, nil
	}

	var commands []CustomCommand

	err := filepath.WalkDir(source.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isMarkdownFile(d.Name()) {
			return err
		}

		cmd, err := loadCommand(path, source.path, source.prefix)
		if err != nil {
			return nil // Skip invalid files
		}

		commands = append(commands, cmd)
		return nil
	})

	return commands, err
}

func loadCommand(path, baseDir, prefix string) (CustomCommand, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return CustomCommand{}, err
	}

	id := buildCommandID(path, baseDir, prefix)

	return CustomCommand{
		ID:        id,
		Name:      id,
		Content:   string(content),
		Arguments: extractArgNames(string(content)),
	}, nil
}

func extractArgNames(content string) []Argument {
	matches := namedArgPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var args []Argument

	for _, match := range matches {
		arg := match[1]
		if !seen[arg] {
			seen[arg] = true
			// for normal custom commands, all args are required
			args = append(args, Argument{ID: arg, Title: arg, Required: true})
		}
	}

	return args
}

func buildCommandID(path, baseDir, prefix string) string {
	relPath, _ := filepath.Rel(baseDir, path)
	parts := strings.Split(relPath, string(filepath.Separator))

	// Remove .md extension from last part
	if len(parts) > 0 {
		lastIdx := len(parts) - 1
		parts[lastIdx] = strings.TrimSuffix(parts[lastIdx], filepath.Ext(parts[lastIdx]))
	}

	return prefix + strings.Join(parts, ":")
}

func isMarkdownFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".md")
}

func GetMCPPrompt(cfg *config.ConfigStore, clientID, promptID string, args map[string]string) (string, error) {
	// Create a context with timeout since tea.Cmd doesn't support context passing.
	// The MCP client has its own timeout, but this provides an additional safeguard.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mcp.GetPromptMessages(ctx, cfg, clientID, promptID, args)
	if err != nil {
		return "", err
	}
	return strings.Join(result, " "), nil
}
