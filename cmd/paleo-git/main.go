package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "paleo-git",
		Short: "Track code migration progress in git repositories",
	}

	rootCmd.AddCommand(measureCmd())
	rootCmd.AddCommand(scanCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func measureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "measure",
		Short: "Run all metrics at a single commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		},
	}
	cmd.Flags().String("config", "", "Path to config file (required)")
	cmd.Flags().String("commit", "HEAD", "Commit to measure")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func scanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Traverse history and measure metrics at sampled commits",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		},
	}
	cmd.Flags().String("config", "", "Path to config file (required)")
	cmd.Flags().String("skip", "", "Path to NDJSON file of prior results to skip")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}
