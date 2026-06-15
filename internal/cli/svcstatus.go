package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/MGM-Laboratory/mgm-cli/internal/config"
	"github.com/MGM-Laboratory/mgm-cli/internal/gatus"
	"github.com/MGM-Laboratory/mgm-cli/internal/ui"
	"github.com/MGM-Laboratory/mgm-cli/internal/version"
)

// newServiceStatusCommand builds `mgm status ...` — checks against Gatus.
func newServiceStatusCommand() *cobra.Command {
	var (
		jsonOut bool
		watch   time.Duration
	)
	c := &cobra.Command{
		Use:   "status [SERVICE]",
		Short: "Check MGM service health (powered by Gatus)",
		Long: "Without arguments, prints a table of every monitored service and its current status.\n" +
			"With a SERVICE argument, drills into a single endpoint: latest checks, response time, recent uptime.\n\n" +
			"SERVICE can be a Gatus key (e.g. core_api), an exact name, a \"group/name\" path, " +
			"or a unique substring. Use `mgm status list` to see what's available.",
		Example: "  mgm status\n  mgm status api\n  mgm status core/api --json\n  mgm status --watch 10s",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cl, _, err := newGatusClient()
			if err != nil {
				return err
			}

			run := func() error {
				if len(args) == 0 {
					return runStatusOverview(ctx, cl, jsonOut)
				}
				return runStatusDetail(ctx, cl, args[0], jsonOut)
			}
			if watch <= 0 {
				return run()
			}
			return runWatch(watch, run)
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON instead of human output")
	c.Flags().DurationVarP(&watch, "watch", "w", 0, "Refresh on the given interval, e.g. 5s, 30s, 1m")

	c.AddCommand(
		newStatusListCommand(),
		newStatusConfigureCommand(),
		newStatusUptimeCommand(),
		newStatusIncidentsCommand(),
		newStatusOpenCommand(),
	)
	return c
}

// ---------- subcommands ----------

func newStatusListCommand() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List every service Gatus knows about",
		Long:  "Prints one row per Gatus endpoint with its key, name, group, last hostname, and current up/down state.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cl, _, err := newGatusClient()
			if err != nil {
				return err
			}
			endpoints, err := cl.ListEndpoints(ctx)
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(endpoints)
			}
			sortEndpoints(endpoints)
			tw := tabwriter.NewWriter(ui.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "STATUS\tNAME\tGROUP\tKEY\tHOSTNAME")
			for _, ep := range endpoints {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					healthSymbol(ep), ep.Name, dashIfEmpty(ep.Group), ep.Key, lastHostname(ep))
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return c
}

func newStatusConfigureCommand() *cobra.Command {
	var (
		urlFlag, tokenFlag string
		nonInteractive     bool
		test               bool
	)
	c := &cobra.Command{
		Use:   "configure",
		Short: "Set Gatus URL (and optional bearer token) for this profile",
		Long: "Stores the Gatus instance URL and an optional API token under the active profile in ~/.mgm/config. " +
			"Most MGM users only need the URL — token is for instances behind auth.",
		Example: "  mgm status configure\n  mgm status configure --url https://status.labmgm.org\n  mgm status configure --url https://status.labmgm.org --token $TOKEN --test",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.New(globalProfile)
			if err != nil {
				return err
			}
			p := cfg.Load()

			ui.Title(fmt.Sprintf("Configure Gatus (profile: %s)", cfg.ProfileName()))
			ui.Infof("Settings will be saved to %s [%s]", cfg.Path(), cfg.ProfileName())

			if urlFlag == "" && !nonInteractive {
				cur := p.GatusURL
				if cur == "" {
					cur = config.DefaultGatusURL
				}
				urlFlag, err = ui.PromptString("Gatus URL", cur)
				if err != nil {
					return err
				}
			} else if urlFlag == "" {
				urlFlag = p.GatusURL
				if urlFlag == "" {
					urlFlag = config.DefaultGatusURL
				}
			}

			if tokenFlag == "" && !nonInteractive {
				tokenFlag, err = ui.PromptSecret("Gatus API token (optional)", p.GatusToken)
				if err != nil {
					return err
				}
			} else if tokenFlag == "" {
				tokenFlag = p.GatusToken
			}

			p.GatusURL = strings.TrimRight(urlFlag, "/")
			p.GatusToken = tokenFlag

			if test {
				ui.Infof("Verifying %s ...", p.GatusURL)
				cl := gatus.New(p.GatusURL, p.GatusToken, "mgm-cli/"+version.Version)
				if _, err := cl.ListEndpoints(context.Background()); err != nil {
					ui.Errorf("Verification failed: %v", err)
					return err
				}
				ui.Successf("Reached Gatus.")
			}

			if err := cfg.Save(p); err != nil {
				return err
			}
			ui.Successf("Saved %s [%s]", cfg.Path(), cfg.ProfileName())
			return nil
		},
	}
	c.Flags().StringVar(&urlFlag, "url", "", "Gatus base URL (default https://status.labmgm.org)")
	c.Flags().StringVar(&tokenFlag, "token", "", "Optional Gatus bearer token")
	c.Flags().BoolVar(&nonInteractive, "no-input", false, "Fail instead of prompting for missing values")
	c.Flags().BoolVar(&test, "test", true, "Verify by hitting Gatus after saving")
	return c
}

func newStatusUptimeCommand() *cobra.Command {
	var window string
	c := &cobra.Command{
		Use:   "uptime SERVICE",
		Short: "Show uptime ratio over a window (1h | 24h | 7d | 30d)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cl, _, err := newGatusClient()
			if err != nil {
				return err
			}
			ep, err := resolveEndpoint(ctx, cl, args[0])
			if err != nil {
				return err
			}
			if !validUptimeWindow(window) {
				return fmt.Errorf("invalid window %q (use 1h, 24h, 7d, or 30d)", window)
			}
			v, err := cl.Uptime(ctx, ep.Key, window)
			if err != nil {
				return err
			}
			ui.KV("service", ep.FullName())
			ui.KV("window", window)
			ui.KV("uptime", fmt.Sprintf("%.2f%%", v*100))
			return nil
		},
	}
	c.Flags().StringVar(&window, "window", "24h", "Window: 1h | 24h | 7d | 30d")
	return c
}

func newStatusIncidentsCommand() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "incidents",
		Short: "Show services that are currently failing",
		Long:  "Lists every service whose latest check is unsuccessful. Exits 1 when there is at least one incident, so it can be wired into CI or shell scripts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cl, _, err := newGatusClient()
			if err != nil {
				return err
			}
			endpoints, err := cl.ListEndpoints(ctx)
			if err != nil {
				return err
			}
			down := make([]gatus.Endpoint, 0)
			for _, ep := range endpoints {
				if ep.LatestResult() == nil {
					continue
				}
				if !ep.Healthy() {
					down = append(down, ep)
				}
			}
			if jsonOut {
				return writeJSON(down)
			}
			if len(down) == 0 {
				ui.Successf("All services healthy (%d checked).", len(endpoints))
				return nil
			}
			sortEndpoints(down)
			ui.Title(fmt.Sprintf("Incidents (%d)", len(down)))
			tw := tabwriter.NewWriter(ui.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tGROUP\tSINCE\tFAILED CONDITION")
			for _, ep := range down {
				r := ep.LatestResult()
				cond := firstFailedCondition(r)
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
					ep.Name, dashIfEmpty(ep.Group), humanAgo(r.Timestamp), cond)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			os.Exit(1)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return c
}

func newStatusOpenCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "open [SERVICE]",
		Short: "Open the Gatus dashboard (or a specific service) in your browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cl, _, err := newGatusClient()
			if err != nil {
				return err
			}
			target := cl.BaseURL()
			if len(args) == 1 {
				ep, err := resolveEndpoint(ctx, cl, args[0])
				if err != nil {
					return err
				}
				target = cl.EndpointPageURL(ep.Key)
			}
			ui.Infof("Opening %s", target)
			return openBrowser(target)
		},
	}
	return c
}

// ---------- runners ----------

func runStatusOverview(ctx context.Context, cl *gatus.Client, jsonOut bool) error {
	endpoints, err := cl.ListEndpoints(ctx)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(endpoints)
	}
	sortEndpoints(endpoints)
	up, down, unknown := 0, 0, 0
	for _, ep := range endpoints {
		r := ep.LatestResult()
		switch {
		case r == nil:
			unknown++
		case r.Success:
			up++
		default:
			down++
		}
	}
	ui.Title(fmt.Sprintf("MGM services — %s", cl.BaseURL()))
	tw := tabwriter.NewWriter(ui.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tNAME\tGROUP\tLAST CHECK\tDURATION\tHTTP")
	for _, ep := range endpoints {
		r := ep.LatestResult()
		when, dur, code := "-", "-", "-"
		if r != nil {
			when = humanAgo(r.Timestamp)
			dur = humanDuration(r.Duration)
			if r.Status > 0 {
				code = fmt.Sprintf("%d", r.Status)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			healthSymbol(ep), ep.Name, dashIfEmpty(ep.Group), when, dur, code)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(ui.Out)
	ui.Infof("%s up   %s down   %s unknown   %s total",
		ui.Key(fmt.Sprintf("%d", up)),
		ui.Key(fmt.Sprintf("%d", down)),
		ui.Key(fmt.Sprintf("%d", unknown)),
		ui.Key(fmt.Sprintf("%d", len(endpoints))),
	)
	return nil
}

func runStatusDetail(ctx context.Context, cl *gatus.Client, query string, jsonOut bool) error {
	ep, err := resolveEndpoint(ctx, cl, query)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(ep)
	}
	ui.Title(ep.FullName())
	ui.KV("key", ep.Key)
	ui.KV("status", healthLabel(*ep))
	ui.KV("page", cl.EndpointPageURL(ep.Key))

	if r := ep.LatestResult(); r != nil {
		ui.KV("last check", humanAgo(r.Timestamp)+" ("+r.Timestamp.Local().Format(time.RFC3339)+")")
		ui.KV("response", fmt.Sprintf("HTTP %d in %s", r.Status, humanDuration(r.Duration)))
		if r.Hostname != "" {
			ui.KV("hostname", r.Hostname)
		}
		if len(r.Errors) > 0 {
			ui.KV("errors", strings.Join(r.Errors, "; "))
		}
		if len(r.ConditionResults) > 0 {
			ui.Infof("\n%s", ui.Key("Conditions"))
			for _, cr := range r.ConditionResults {
				mark := "OK "
				if !cr.Success {
					mark = "FAIL"
				}
				fmt.Fprintf(ui.Out, "  [%s] %s\n", mark, cr.Condition)
			}
		}
	}

	if windows := []string{"1h", "24h", "7d"}; len(windows) > 0 {
		ui.Infof("\n%s", ui.Key("Uptime"))
		for _, w := range windows {
			v, err := cl.Uptime(ctx, ep.Key, w)
			if err != nil {
				ui.KV(w, ui.Dim("(n/a)"))
				continue
			}
			ui.KV(w, fmt.Sprintf("%.2f%%", v*100))
		}
	}

	if len(ep.Result) > 1 {
		ui.Infof("\n%s", ui.Key("Recent checks"))
		tw := tabwriter.NewWriter(ui.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "WHEN\tOK\tHTTP\tDURATION")
		recent := ep.Result
		if len(recent) > 10 {
			recent = recent[len(recent)-10:]
		}
		for i := len(recent) - 1; i >= 0; i-- {
			r := recent[i]
			ok := "OK"
			if !r.Success {
				ok = "FAIL"
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n",
				humanAgo(r.Timestamp), ok, r.Status, humanDuration(r.Duration))
		}
		return tw.Flush()
	}
	return nil
}

func runWatch(interval time.Duration, run func() error) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		fmt.Fprint(ui.Out, "\033[2J\033[H") // clear + home
		ui.Infof("%s", ui.Dim(fmt.Sprintf("watching every %s — Ctrl-C to stop — %s", interval, time.Now().Format(time.Kitchen))))
		if err := run(); err != nil {
			ui.Errorf("%v", err)
		}
		<-ticker.C
	}
}

// ---------- helpers ----------

func newGatusClient() (*gatus.Client, config.Profile, error) {
	cfg, err := config.New(globalProfile)
	if err != nil {
		return nil, config.Profile{}, err
	}
	p := cfg.Load()
	if !p.HasGatus() {
		if !ui.IsInteractive() {
			return nil, p, fmt.Errorf("no Gatus URL configured for profile %q. Run `mgm status configure` or set MGM_GATUS_URL", cfg.ProfileName())
		}
		ui.Title(fmt.Sprintf("Configure Gatus (profile: %s)", cfg.ProfileName()))
		ui.Infof("Settings will be saved to %s [%s]", cfg.Path(), cfg.ProfileName())
		urlVal, err := ui.PromptString("Gatus URL", config.DefaultGatusURL)
		if err != nil {
			return nil, p, err
		}
		token, err := ui.PromptSecret("Gatus API token (optional)", "")
		if err != nil {
			return nil, p, err
		}
		p.GatusURL = strings.TrimRight(urlVal, "/")
		p.GatusToken = token
		if err := cfg.Save(p); err != nil {
			return nil, p, err
		}
		ui.Successf("Saved %s [%s]", cfg.Path(), cfg.ProfileName())
	}
	cl := gatus.New(p.GatusURL, p.GatusToken, "mgm-cli/"+version.Version)
	return cl, p, nil
}

func resolveEndpoint(ctx context.Context, cl *gatus.Client, query string) (*gatus.Endpoint, error) {
	endpoints, err := cl.ListEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	ep, err := gatus.FindEndpoint(endpoints, query)
	if err != nil {
		return nil, err
	}
	if ep != nil {
		return ep, nil
	}
	if !ui.IsInteractive() {
		return nil, fmt.Errorf("no service matches %q (try `mgm status list`)", query)
	}
	choices := make([]ui.Choice, 0, len(endpoints))
	sortEndpoints(endpoints)
	for _, e := range endpoints {
		choices = append(choices, ui.Choice{
			Label: e.FullName(),
			Value: e.Key,
			Hint:  e.Key,
		})
	}
	ui.Warnf("No exact match for %q — pick a service:", query)
	picked, err := ui.SelectOne("Service", choices)
	if err != nil {
		return nil, err
	}
	for i := range endpoints {
		if endpoints[i].Key == picked {
			return &endpoints[i], nil
		}
	}
	return nil, fmt.Errorf("internal: picked key %q not in list", picked)
}

func writeJSON(v any) error {
	enc := json.NewEncoder(ui.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func sortEndpoints(eps []gatus.Endpoint) {
	sort.Slice(eps, func(i, j int) bool {
		if eps[i].Group != eps[j].Group {
			return eps[i].Group < eps[j].Group
		}
		return eps[i].Name < eps[j].Name
	})
}

func healthSymbol(ep gatus.Endpoint) string {
	r := ep.LatestResult()
	if r == nil {
		return ui.Dim("?")
	}
	if r.Success {
		return ui.SuccessText("UP")
	}
	return ui.ErrorText("DOWN")
}

func healthLabel(ep gatus.Endpoint) string {
	r := ep.LatestResult()
	switch {
	case r == nil:
		return ui.Dim("unknown")
	case r.Success:
		return ui.SuccessText("UP")
	default:
		return ui.ErrorText("DOWN")
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func lastHostname(ep gatus.Endpoint) string {
	r := ep.LatestResult()
	if r == nil || r.Hostname == "" {
		return "-"
	}
	return r.Hostname
}

func firstFailedCondition(r *gatus.Result) string {
	if r == nil {
		return "-"
	}
	for _, c := range r.ConditionResults {
		if !c.Success {
			return c.Condition
		}
	}
	if len(r.Errors) > 0 {
		return r.Errors[0]
	}
	return "-"
}

func humanAgo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func humanDuration(ns int64) string {
	d := time.Duration(ns)
	switch {
	case d <= 0:
		return "-"
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1e3)
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func validUptimeWindow(w string) bool {
	switch w {
	case "1h", "24h", "7d", "30d":
		return true
	}
	return false
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
