package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagGroup defines a named group of flags for organized help output.
type flagGroup struct {
	title string
	flags []string
}

// applyGroupedUsage sets a custom usage function that renders flags by group.
func applyGroupedUsage(cmd *cobra.Command, groups []flagGroup) {
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		_, _ = fmt.Fprintf(c.OutOrStderr(), "Usage:\n  %s\n", c.UseLine())

		if c.HasExample() {
			_, _ = fmt.Fprintf(c.OutOrStderr(), "\nExamples:\n%s\n", c.Example)
		}

		grouped := make(map[string]bool)
		for _, g := range groups {
			var fs pflag.FlagSet
			for _, name := range g.flags {
				f := c.Flags().Lookup(name)
				if f != nil && !f.Hidden {
					fs.AddFlag(f)
					grouped[name] = true
				}
			}
			if usage := fs.FlagUsages(); strings.TrimSpace(usage) != "" {
				_, _ = fmt.Fprintf(c.OutOrStderr(), "\n%s:\n%s", g.title, usage)
			}
		}

		// Remaining ungrouped flags
		var remaining pflag.FlagSet
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if !f.Hidden && !grouped[f.Name] {
				remaining.AddFlag(f)
			}
		})
		if usage := remaining.FlagUsages(); strings.TrimSpace(usage) != "" {
			_, _ = fmt.Fprintf(c.OutOrStderr(), "\nOther flags:\n%s", usage)
		}

		return nil
	})
}
