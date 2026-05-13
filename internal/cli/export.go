package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/dotenv"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
	"gopkg.in/yaml.v3"
)

func newExportCommand() *cobra.Command {
	var (
		sel    selectionFlags
		format string
	)
	c := &cobra.Command{
		Use:   "export",
		Short: "Export secrets to stdout in a chosen format",
		Long: "Formats: dotenv (default), json, yaml, shell (eval-able `export FOO=bar` lines), " +
			"and csv. Useful for piping into other tools.",
		Example: "  mgm env export --format json\n" +
			"  eval \"$(mgm env export --format shell --env prod)\"",
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

			pairs := make(map[string]string, len(secrets))
			for _, s := range secrets {
				pairs[s.SecretKey] = s.SecretValue
			}

			switch strings.ToLower(format) {
			case "dotenv", "env", "":
				return dotenvToWriter(dotenv.FromMap(pairs), true, ui.Out)
			case "json":
				enc := json.NewEncoder(ui.Out)
				enc.SetIndent("", "  ")
				return enc.Encode(pairs)
			case "yaml", "yml":
				enc := yaml.NewEncoder(ui.Out)
				defer enc.Close()
				return enc.Encode(pairs)
			case "shell", "sh", "bash":
				keys := make([]string, 0, len(pairs))
				for k := range pairs {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(ui.Out, "export %s=%s\n", k, shellQuote(pairs[k]))
				}
				return nil
			case "csv":
				keys := make([]string, 0, len(pairs))
				for k := range pairs {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				fmt.Fprintln(ui.Out, "key,value")
				for _, k := range keys {
					fmt.Fprintf(ui.Out, "%s,%s\n", csvEscape(k), csvEscape(pairs[k]))
				}
				return nil
			default:
				return fmt.Errorf("unknown --format %q (use dotenv|json|yaml|shell|csv)", format)
			}
		},
	}
	addSelectionFlags(c, &sel)
	c.Flags().StringVar(&format, "format", "dotenv", "Output format: dotenv | json | yaml | shell | csv")
	return c
}

func shellQuote(s string) string {
	if !strings.ContainsAny(s, " \t\"'$`\\\n#&|;<>(){}[]*?~!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
