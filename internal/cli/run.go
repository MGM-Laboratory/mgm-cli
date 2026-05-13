package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var (
		sel        selectionFlags
		passEnv    bool
		shellExec  bool
	)
	c := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Execute a command with Infisical secrets injected as env vars",
		Long: "Pulls the secrets at the selected project/env/folder, merges them into the current environment " +
			"(secrets win over inherited vars unless --no-inherit), then execs <command>.",
		Example: "  mgm env run -- node server.js\n" +
			"  mgm env run --env prod -- ./bin/migrate\n" +
			"  mgm env run --shell -- 'echo $DATABASE_URL'",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rt, err := resolveRuntime(ctx, globalProfile)
			if err != nil {
				return err
			}
			projectID, environment, folder, err := rt.resolveSelection(ctx, sel)
			if err != nil {
				return err
			}
			secrets, err := rt.Client.ListSecrets(ctx, projectID, environment, folder)
			if err != nil {
				return err
			}

			env := []string{}
			if passEnv {
				env = append(env, os.Environ()...)
			}
			env = append(env,
				"MGM_INFISICAL_PROJECT_ID="+projectID,
				"MGM_INFISICAL_ENVIRONMENT="+environment,
				"MGM_INFISICAL_FOLDER="+folder,
			)
			for _, s := range secrets {
				env = append(env, s.SecretKey+"="+s.SecretValue)
			}

			var ex *exec.Cmd
			if shellExec {
				joined := strings.Join(args, " ")
				if runtime.GOOS == "windows" {
					ex = exec.Command("powershell", "-NoProfile", "-Command", joined)
				} else {
					ex = exec.Command("sh", "-c", joined)
				}
			} else {
				ex = exec.Command(args[0], args[1:]...)
			}
			ex.Env = env
			ex.Stdin = os.Stdin
			ex.Stdout = os.Stdout
			ex.Stderr = os.Stderr
			if err := ex.Run(); err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					os.Exit(ee.ExitCode())
				}
				return fmt.Errorf("run: %w", err)
			}
			return nil
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().BoolVar(&passEnv, "inherit", true, "Inherit the parent process environment in addition to the secrets")
	c.Flags().BoolVar(&shellExec, "shell", false, "Run the command via the system shell")
	return c
}
