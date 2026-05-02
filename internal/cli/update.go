package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/updater"
)

// newUpdateCmd creates the "update" subcommand.
func newUpdateCmd() *cobra.Command {
	var (
		checkOnly bool
		version   string
		dryRun    bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: i18n.T("cli_update_short"),
		Long: `Update EchoWarp to the latest or a specific version from GitHub releases.

Examples:
  # Check for updates without installing
  echowarp update --check

  # Update to the latest version
  echowarp update

  # Update to a specific version
  echowarp update --version 1.2.0

  # Show what would be updated without making changes
  echowarp update --dry-run

  # Force update even if already up to date
  echowarp update --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(checkOnly, version, dryRun, force)
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, don't install")
	cmd.Flags().StringVar(&version, "version", "", "Update to a specific version (e.g., 1.2.0)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
	cmd.Flags().BoolVar(&force, "force", false, "Force update even if already up to date")

	return cmd
}

// runUpdate executes the update process.
func runUpdate(checkOnly bool, targetVersion string, dryRun bool, force bool) error {
	ctx := createUpdateContext()

	u, err := updater.New()
	if err != nil {
		return fmt.Errorf("failed to initialize updater: %w", err)
	}

	release, err := checkForUpdateRelease(ctx, u, targetVersion)
	if err != nil {
		return err
	}

	if shouldSkipUpdate(u, release, force, checkOnly) {
		return nil
	}

	if checkOnly {
		return handleUpdateCheckOnly(u, release)
	}

	if dryRun {
		return handleUpdateDryRun(u, release.TagName)
	}

	return performUpdate(ctx, u, release)
}

// createUpdateContext creates a context with 2-minute timeout and signal handling.
func createUpdateContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	return ctx
}

// checkForUpdateRelease checks for updates and displays version information.
func checkForUpdateRelease(ctx context.Context, u *updater.Updater, targetVersion string) (*updater.Release, error) {
	fmt.Printf("Current version: %s\n", u.GetCurrentVersion())
	if targetVersion != "" {
		fmt.Printf("Target version: %s\n", targetVersion)
	} else {
		fmt.Println("Checking for updates...")
	}

	release, err := u.CheckForUpdate(ctx, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}

	fmt.Printf("Latest available version: %s\n", release.TagName)
	return release, nil
}

// shouldSkipUpdate determines if update should be skipped based on current state.
func shouldSkipUpdate(u *updater.Updater, release *updater.Release, force, checkOnly bool) bool {
	if !u.IsNewer(release) && !force {
		fmt.Println("You're already running the latest version!")
		if !checkOnly {
			fmt.Println("Use --force to update anyway.")
		}
		return true
	}

	if force && !u.IsNewer(release) {
		fmt.Println("Forcing update to same version...")
	}
	return false
}

// handleUpdateCheckOnly displays update information without installing.
func handleUpdateCheckOnly(u *updater.Updater, release *updater.Release) error {
	fmt.Printf("\nUpdate available: %s -> %s\n", u.GetCurrentVersion(), release.TagName)
	fmt.Printf("Release notes: %s\n", release.HTMLURL)
	return nil
}

// handleUpdateDryRun shows what would be updated without making changes.
func handleUpdateDryRun(u *updater.Updater, releaseVersion string) error {
	fmt.Printf("\n[DRY RUN] Would perform the following actions:\n")
	fmt.Printf("  - Download version %s from GitHub\n", releaseVersion)
	fmt.Printf("  - Backup current binary to %s.backup\n", u.GetExecutablePath())
	fmt.Printf("  - Replace binary at %s\n", u.GetExecutablePath())
	fmt.Printf("  - Verify installation\n")
	return nil
}

// performUpdate downloads and installs the update.
func performUpdate(ctx context.Context, u *updater.Updater, release *updater.Release) error {
	fmt.Printf("\nUpdating to version %s...\n", release.TagName)

	tmpPath := u.GetExecutablePath() + ".new"
	fmt.Println("Downloading...")

	err := u.DownloadRelease(ctx, release, tmpPath, func(percentage int) {
		if percentage%10 == 0 {
			fmt.Printf("  Progress: %d%%\n", percentage)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	fmt.Println("Installing...")
	if err := u.Install(tmpPath); err != nil {
		return fmt.Errorf("failed to install update: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated to version %s!\n", release.TagName)
	fmt.Println("You may need to restart any running EchoWarp processes.")
	return nil
}
