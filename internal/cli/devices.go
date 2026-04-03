package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// newDevicesCmd creates the "devices" subcommand.
// Lists available audio input and/or output devices with their IDs,
// channels, and sample rates. Use the ID with --device flag.
func newDevicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "List available audio devices",
		RunE:  runDevices,
	}
	cmd.Flags().Bool("input", false, "Show only input (capture) devices")
	cmd.Flags().Bool("output", false, "Show only output (playback) devices")
	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}

type jsonDevice struct {
	ID         uint32 `json:"id"`
	Name       string `json:"name"`
	Channels   uint32 `json:"channels"`
	SampleRate uint32 `json:"sample_rate"`
	Type       string `json:"type"`
}

type jsonDeviceList struct {
	Input  []jsonDevice `json:"input"`
	Output []jsonDevice `json:"output"`
}

func runDevices(cmd *cobra.Command, args []string) error {
	onlyInput, _ := cmd.Flags().GetBool("input")
	onlyOutput, _ := cmd.Flags().GetBool("output")
	jsonOut, _ := cmd.Flags().GetBool("json")
	// If neither flag is set, show both
	showInput := !onlyOutput
	showOutput := !onlyInput

	dm, err := audio.NewDeviceManager()
	if err != nil {
		return fmt.Errorf("failed to initialize audio: %w", err)
	}
	defer func() { _ = dm.Close() }()

	var inputDevices, outputDevices []audio.AudioDevice

	if showInput {
		inputDevices, err = dm.ListInputDevices()
		if err != nil {
			return fmt.Errorf("failed to list input devices: %w", err)
		}
	}

	if showOutput {
		outputDevices, err = dm.ListOutputDevices()
		if err != nil {
			return fmt.Errorf("failed to list output devices: %w", err)
		}
	}

	if jsonOut {
		return printDevicesJSON(inputDevices, outputDevices, showInput, showOutput)
	}

	if showInput {
		_, _ = fmt.Fprintln(os.Stdout, "Input devices (capture):")
		if len(inputDevices) == 0 {
			_, _ = fmt.Fprintln(os.Stdout, "  (none found)")
		}
		for _, d := range inputDevices {
			_, _ = fmt.Fprintf(os.Stdout, "  [%d] %s (channels=%d, sampleRate=%d)\n",
				d.ID, d.Name, d.Channels, d.SampleRate)
		}
	}

	if showOutput {
		if showInput {
			_, _ = fmt.Fprintln(os.Stdout, "")
		}
		_, _ = fmt.Fprintln(os.Stdout, "Output devices (playback):")
		if len(outputDevices) == 0 {
			_, _ = fmt.Fprintln(os.Stdout, "  (none found)")
		}
		for _, d := range outputDevices {
			_, _ = fmt.Fprintf(os.Stdout, "  [%d] %s (channels=%d, sampleRate=%d)\n",
				d.ID, d.Name, d.Channels, d.SampleRate)
		}
	}

	return nil
}

func printDevicesJSON(inputDevices, outputDevices []audio.AudioDevice, showInput, showOutput bool) error {
	result := jsonDeviceList{
		Input:  []jsonDevice{},
		Output: []jsonDevice{},
	}
	if showInput {
		for _, d := range inputDevices {
			result.Input = append(result.Input, jsonDevice{d.ID, d.Name, d.Channels, d.SampleRate, "input"})
		}
	}
	if showOutput {
		for _, d := range outputDevices {
			result.Output = append(result.Output, jsonDevice{d.ID, d.Name, d.Channels, d.SampleRate, "output"})
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
