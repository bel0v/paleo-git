package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bel0v/paleo-git/config"
	"github.com/bel0v/paleo-git/engine"
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

func measureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "measure",
		Short: "Run all metrics at a single commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			commitRef, _ := cmd.Flags().GetString("commit")
			repoPath, _ := cmd.Flags().GetString("repo")

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			results, err := engine.Measure(cmd.Context(), cfg, repoPath, commitRef)
			if err != nil {
				return err
			}

			out, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return fmt.Errorf("marshalling results: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(out))

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
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func scanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Traverse history and measure metrics at sampled commits",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			skipPath, _ := cmd.Flags().GetString("skip")
			repoPath, _ := cmd.Flags().GetString("repo")

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}

			var opts engine.ScanOptions
			if skipPath != "" {
				keys, err := readSkipFile(skipPath)
				if err != nil {
					return err
				}
				opts.AlreadyMeasured = keys
			}

			return engine.Scan(cmd.Context(), cfg, repoPath, opts, func(r engine.Result) {
				line, err := json.Marshal(r)
				if err != nil {
					fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
					return
				}
				fmt.Fprintln(os.Stdout, string(line))
			})
		},
	}
	cmd.Flags().String("config", "", "Path to config file (required)")
	cmd.Flags().String("skip", "", "Path to NDJSON file of prior results to skip")
	cmd.Flags().String("repo", ".", "Path to git repository")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func readSkipFile(path string) ([]engine.MeasuredKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading skip file: %w", err)
	}
	defer f.Close()

	seen := make(map[string]bool)
	var keys []engine.MeasuredKey
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var r engine.Result
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		if r.Commit == "" {
			continue
		}
		key := r.MetricID + ":" + r.MetricHash + ":" + r.Commit
		if !seen[key] {
			seen[key] = true
			keys = append(keys, engine.MeasuredKey{MetricID: r.MetricID, MetricHash: r.MetricHash, Commit: r.Commit})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading skip file: %w", err)
	}
	return keys, nil
}
