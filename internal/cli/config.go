package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/lHumaNl/echowarp/internal/config"
)

// newConfigCmd creates the "config" command group for configuration management.
// Subcommands: init, show, validate.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage EchoWarp configuration",
	}

	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigValidateCmd())
	return cmd
}

// newConfigInitCmd creates the "config init" subcommand.
// Creates a default configuration file at the specified path,
// or ~/.config/echowarp/config.yaml if not specified.
func newConfigInitCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a configuration file",
		Long:  `Create a configuration file for EchoWarp with sensible defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				outputPath = filepath.Join(config.EchoWarpDir(), "config.yaml")
			}

			cfg := config.DefaultConfig()
			if err := cfg.SaveToFile(outputPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Configuration saved to %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: ~/.config/echowarp/config.yaml)")
	return cmd
}

// newConfigShowCmd creates the "config show" subcommand.
// Loads the effective config (file + env vars + defaults) and prints it as YAML.
// By default, secrets are redacted via SafeString(); use --raw to show plaintext.
func newConfigShowCmd() *cobra.Command {
	var configPath string
	var raw bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the effective configuration",
		Long: `Load and display the effective EchoWarp configuration.

Merges values from the config file, environment variables (ECHOWARP_*),
and built-in defaults. Secrets (passwords, credentials) are redacted by
default; pass --raw to see the unredacted values.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithViper(configPath, config.ModeServer)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if raw {
				b, err := yaml.Marshal(&cfg)
				if err != nil {
					return fmt.Errorf("failed to marshal config: %w", err)
				}
				fmt.Print(string(b))
			} else {
				fmt.Print(cfg.SafeString())
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	cmd.Flags().BoolVar(&raw, "raw", false, "Show secrets without redaction")
	return cmd
}

// newConfigValidateCmd creates the "config validate" subcommand.
// Loads config from a required --config file and runs Validate().
// Exits with code 1 if validation fails.
func newConfigValidateCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a configuration file",
		Long: `Load and validate a configuration file.

Checks all fields for valid values and required settings.
Exits with code 0 if the configuration is valid, or code 1 with a
list of errors if it is not.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				configPath = defaultConfigPath()
			}

			cfg, err := config.LoadFromFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			errs := cfg.Validate()
			if len(errs) == 0 {
				fmt.Println("Configuration is valid.")
				return nil
			}

			fmt.Fprintf(os.Stderr, "Configuration is invalid (%d error(s)):\n", len(errs))
			for i, e := range errs {
				fmt.Fprintf(os.Stderr, "  %d. %v\n", i+1, e)
			}
			os.Exit(1)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file (default: ~/.config/echowarp/config.yaml)")
	return cmd
}
