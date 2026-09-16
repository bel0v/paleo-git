package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bel0v/paleo-git/config"
	"github.com/bel0v/paleo-git/engine"
	"github.com/bel0v/paleo-git/store"
	"github.com/bel0v/paleo-git/vcs"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := buildRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "paleo-git",
		Short:         "Track code migration progress in git repositories",
		Version:       fmt.Sprintf("%s (%s)", version, commit),
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once
	}

	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress stdout output")

	rootCmd.AddCommand(measureCmd())
	rootCmd.AddCommand(scanCmd())

	return rootCmd
}

func loadConfig(path string) (config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config.Config{}, fmt.Errorf("reading config: %w", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return config.Config{}, err
	}
	if err := config.Validate(cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

// reportFailures prints every failed result to w and returns an error
// summarising them, so failures reach the caller even with --quiet.
func reportFailures(w io.Writer, results []engine.Result) error {
	failed := 0
	for _, r := range results {
		if r.Status == engine.StatusError {
			failed++
			fmt.Fprintln(w, failureLine(r))
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d metric(s) failed", failed, len(results))
	}
	return nil
}

func failureLine(r engine.Result) string {
	commit := r.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return fmt.Sprintf("error: %s at %s: %s", r.MetricID, commit, r.Error)
}

func measureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "measure",
		Short: "Run all metrics at a single commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			commitRef, _ := cmd.Flags().GetString("commit")
			repoPath, _ := cmd.Flags().GetString("repo")
			loadDir, _ := cmd.Flags().GetString("load-dir")
			saveDir, _ := cmd.Flags().GetString("save-dir")
			quiet, _ := cmd.Flags().GetBool("quiet")

			ctx := cmd.Context()

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			meta, err := vcs.ResolveCommit(ctx, repoPath, commitRef)
			if err != nil {
				return fmt.Errorf("resolving commit %q: %w", commitRef, err)
			}

			// Drop metrics already measured at this commit so they are neither
			// re-run nor re-appended.
			if loadDir != "" {
				keys, err := store.NewDir(loadDir).AlreadyMeasured(ctx)
				if err != nil {
					return fmt.Errorf("loading data: %w", err)
				}
				skip := make(map[engine.MeasuredKey]bool, len(keys))
				for _, k := range keys {
					skip[k] = true
				}
				pending := make([]config.Metric, 0, len(cfg.Metrics))
				for _, m := range cfg.Metrics {
					if !skip[engine.MeasuredKey{MetricID: m.ID, MetricHash: config.MetricHash(m), Commit: meta.SHA}] {
						pending = append(pending, m)
					}
				}
				cfg.Metrics = pending
			}

			results, err := engine.Measure(ctx, cfg, repoPath, meta.SHA)
			if err != nil {
				return err
			}
			if results == nil {
				results = []engine.Result{}
			}

			if !quiet {
				out, err := json.MarshalIndent(results, "", "  ")
				if err != nil {
					return fmt.Errorf("marshalling results: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			}

			if saveDir != "" && len(results) > 0 {
				if err := store.NewDir(saveDir).Append(ctx, results); err != nil {
					return fmt.Errorf("saving results: %w", err)
				}
			}

			return reportFailures(cmd.ErrOrStderr(), results)
		},
	}
	cmd.Flags().String("config", "", "Path to config file (required)")
	cmd.Flags().String("commit", "HEAD", "Commit to measure")
	cmd.Flags().String("repo", ".", "Path to git repository")
	cmd.Flags().String("load-dir", "", "Load prior results from data directory")
	cmd.Flags().String("save-dir", "", "Save results to data directory")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func scanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Traverse history and measure metrics at sampled commits",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			loadDir, _ := cmd.Flags().GetString("load-dir")
			saveDir, _ := cmd.Flags().GetString("save-dir")
			repoPath, _ := cmd.Flags().GetString("repo")
			quiet, _ := cmd.Flags().GetBool("quiet")

			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			var opts engine.ScanOptions
			if loadDir != "" {
				keys, err := store.NewDir(loadDir).AlreadyMeasured(ctx)
				if err != nil {
					return fmt.Errorf("loading data: %w", err)
				}
				opts.AlreadyMeasured = keys
			}

			var saveStore store.Dir
			if saveDir != "" {
				saveStore = store.NewDir(saveDir)
			}

			var saveErr error
			var failed []engine.Result
			err = engine.Scan(ctx, cfg, repoPath, opts, func(r engine.Result) {
				if !quiet {
					line, err := json.Marshal(r)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "marshal error: %v\n", err)
						return
					}
					fmt.Fprintln(out, string(line))
				}
				if r.Status == engine.StatusError {
					failed = append(failed, r)
				}
				if saveDir != "" && saveErr == nil {
					saveErr = saveStore.Append(ctx, []engine.Result{r})
				}
			})
			if saveErr != nil {
				return fmt.Errorf("saving results: %w", saveErr)
			}
			if err != nil {
				return err
			}

			for _, r := range failed {
				fmt.Fprintln(cmd.ErrOrStderr(), failureLine(r))
			}
			if len(failed) > 0 {
				return fmt.Errorf("%d measurement(s) failed", len(failed))
			}
			return nil
		},
	}
	cmd.Flags().String("config", "", "Path to config file (required)")
	cmd.Flags().String("load-dir", "", "Load prior results from data directory")
	cmd.Flags().String("save-dir", "", "Save results to data directory")
	cmd.Flags().String("repo", ".", "Path to git repository")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}
