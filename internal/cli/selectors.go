package cli

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/MGM-Laboratory/mgm-cli/internal/infisical"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
)

// selectionFlags holds the fast-path overrides every env command accepts.
type selectionFlags struct {
	Project     string // ID or slug
	Environment string
	Folder      string
}

// resolveSelection produces a fully-qualified (projectID, environment, folder)
// triple. It honours flags first, then the project file, then the profile
// defaults, and finally falls back to interactive prompts when something is
// still missing and stdin is a TTY.
func (rt *runtimeCtx) resolveSelection(ctx context.Context, sel selectionFlags) (projectID, environment, folder string, err error) {
	projectID = sel.Project
	environment = sel.Environment
	folder = sel.Folder

	if projectID == "" && rt.Project != nil {
		projectID = rt.Project.ProjectID
	}
	if projectID == "" {
		projectID = rt.Profile.DefaultProjectID
	}

	if environment == "" && rt.Project != nil {
		environment = rt.Project.Environment
	}
	if environment == "" {
		environment = rt.Profile.DefaultEnvironment
	}

	if folder == "" && rt.Project != nil {
		folder = rt.Project.Folder
	}
	if folder == "" {
		folder = rt.Profile.DefaultFolder
	}
	if folder == "" {
		folder = "/"
	}

	// Resolve a slug to an ID if necessary.
	if projectID != "" && !looksLikeID(projectID) {
		projectID, err = rt.resolveProjectSlug(ctx, projectID)
		if err != nil {
			return "", "", "", err
		}
	}

	if projectID == "" {
		projectID, err = rt.pickProject(ctx)
		if err != nil {
			return "", "", "", err
		}
	}
	if environment == "" {
		environment, err = rt.pickEnvironment(ctx, projectID)
		if err != nil {
			return "", "", "", err
		}
	}
	folder, err = rt.maybePickFolder(ctx, projectID, environment, folder)
	if err != nil {
		return "", "", "", err
	}
	return projectID, environment, folder, nil
}

func (rt *runtimeCtx) pickProject(ctx context.Context) (string, error) {
	projects, err := rt.Client.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", fmt.Errorf("no Infisical projects accessible to this identity")
	}
	if !ui.IsInteractive() {
		return "", fmt.Errorf("project not specified; pass --project <id|slug>")
	}
	choices := make([]ui.Choice, 0, len(projects))
	for _, p := range projects {
		choices = append(choices, ui.Choice{
			Label: p.Name,
			Value: p.ID,
			Hint:  p.Slug,
		})
	}
	return ui.SelectOne("Project", choices)
}

func (rt *runtimeCtx) resolveProjectSlug(ctx context.Context, slug string) (string, error) {
	projects, err := rt.Client.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if p.Slug == slug || p.Name == slug {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("no project with slug or name %q", slug)
}

func (rt *runtimeCtx) pickEnvironment(ctx context.Context, projectID string) (string, error) {
	envs, err := rt.Client.ListEnvironments(ctx, projectID)
	if err != nil {
		return "", err
	}
	if len(envs) == 0 {
		return "", fmt.Errorf("no environments configured on project")
	}
	if !ui.IsInteractive() {
		return "", fmt.Errorf("environment not specified; pass --env <slug>")
	}
	choices := make([]ui.Choice, 0, len(envs))
	for _, e := range envs {
		choices = append(choices, ui.Choice{
			Label: e.Name,
			Value: e.Slug,
			Hint:  e.Slug,
		})
	}
	return ui.SelectOne("Environment", choices)
}

// maybePickFolder lets the user drill into subfolders. A folder explicitly
// passed via flag is taken at face value; otherwise we offer a navigator
// starting from "/".
func (rt *runtimeCtx) maybePickFolder(ctx context.Context, projectID, environment, requested string) (string, error) {
	if requested != "" && requested != "/" {
		return requested, nil
	}
	if !ui.IsInteractive() {
		return "/", nil
	}
	current := "/"
	for {
		folders, err := rt.Client.ListFolders(ctx, projectID, environment, current)
		if err != nil {
			return "", err
		}
		choices := []ui.Choice{
			{Label: fmt.Sprintf("[ use this folder: %s ]", current), Value: "__use__"},
		}
		if current != "/" {
			choices = append(choices, ui.Choice{Label: ".. (up)", Value: "__up__"})
		}
		for _, f := range folders {
			choices = append(choices, ui.Choice{
				Label: f.Name + "/",
				Value: f.Name,
				Hint:  path.Join(current, f.Name),
			})
		}
		picked, err := ui.SelectOne(fmt.Sprintf("Folder (%s)", current), choices)
		if err != nil {
			return "", err
		}
		switch picked {
		case "__use__":
			return current, nil
		case "__up__":
			current = path.Dir(current)
			if current == "" {
				current = "/"
			}
		default:
			current = path.Join(current, picked)
		}
	}
}

func looksLikeID(s string) bool {
	// Infisical workspace IDs are UUID-shaped. Slugs are kebab-case without dashes-as-id.
	return len(s) >= 32 && strings.Count(s, "-") >= 4
}

// asInfisicalSecrets converts our local entries to API payloads.
func asInfisicalSecrets(in map[string]string) []infisical.Secret {
	out := make([]infisical.Secret, 0, len(in))
	for k, v := range in {
		out = append(out, infisical.Secret{SecretKey: k, SecretValue: v})
	}
	return out
}
