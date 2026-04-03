package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/version"
)

// newVersionCmd creates the "version" subcommand.
// Prints the EchoWarp version, commit, and build date.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of EchoWarp",
		Run: func(cmd *cobra.Command, args []string) {
			var info strings.Builder
			fmt.Fprintf(&info, "EchoWarp %s\n", version.Version)
			if version.Commit != "unknown" {
				fmt.Fprintf(&info, "  Commit:    %s\n", version.Commit)
			}
			if version.BuildDate != "unknown" {
				fmt.Fprintf(&info, "  Built:     %s\n", version.BuildDate)
			}
			fmt.Print(info.String())
		},
	}
}
