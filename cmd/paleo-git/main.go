package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bel0v/paleo-git/config"
	"github.com/bel0v/paleo-git/engine"
	"github.com/bel0v/paleo-git/store"
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
		Use:          "paleo-git",
		Short:        "Track code migration progress in git repositories",
		Version:      fmt.Sprintf("%s (%s)", version, commit),
		SilenceUsage: true,
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

func openStoreDir(path string) (store.Dir, error) {
	d, err := store.NewDir(path)
	if err != nil {
		return store.Dir{}, fmt.Errorf("invalid data directory: %w", err)
	}
	return d, nil
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

			// Build skip set from existing data
			skip := make(map[engine.MeasuredKey]bool)
			if loadDir != "" {
				d, err := openStoreDir(loadDir)
				if err != nil {
					return err
				}
				keys, err := d.AlreadyMeasured(ctx)
				if err != nil {
					return fmt.Errorf("loading data: %w", err)
				}
				for _, k := range keys {
					skip[k] = true
				}
			}

			results, err := engine.Measure(ctx, cfg, repoPath, commitRef)
			if err != nil {
				return err
			}

			// Filter out already-measured results
			var newResults []engine.Result
			for _, r := range results {
				k := engine.MeasuredKey{MetricID: r.MetricID, MetricHash: r.MetricHash, Commit: r.Commit}
				if !skip[k] {
					newResults = append(newResults, r)
				}
			}

			if !quiet {
				out, err := json.MarshalIndent(newResults, "", "  ")
				if err != nil {
					return fmt.Errorf("marshalling results: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			}

			if saveDir != "" && len(newResults) > 0 {
				d, err := openStoreDir(saveDir)
				if err != nil {
					return err
				}
				if err := d.Append(ctx, newResults); err != nil {
					return fmt.Errorf("saving results: %w", err)
				}
			}

			for _, r := range results {
				if r.Status == engine.StatusError {
					return fmt.Errorf("one or more metrics failed")
				}
			}
			return nil
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
				d, err := openStoreDir(loadDir)
				if err != nil {
					return err
				}
				keys, err := d.AlreadyMeasured(ctx)
				if err != nil {
					return fmt.Errorf("loading data: %w", err)
				}
				opts.AlreadyMeasured = keys
			}

			var toSave []engine.Result

			err = engine.Scan(ctx, cfg, repoPath, opts, func(r engine.Result) {
				if !quiet {
					line, err := json.Marshal(r)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "marshal error: %v\n", err)
						return
					}
					fmt.Fprintln(out, string(line))
				}
				if saveDir != "" {
					toSave = append(toSave, r)
				}
			})
			if err != nil {
				return err
			}

			if saveDir != "" && len(toSave) > 0 {
				d, err := openStoreDir(saveDir)
				if err != nil {
					return err
				}
				if err := d.Append(ctx, toSave); err != nil {
					return fmt.Errorf("saving results: %w", err)
				}
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
