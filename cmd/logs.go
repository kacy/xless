package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/kacy/xless/internal/device"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	logsCmd.Flags().String("filter", "", "filter log messages by keyword")
	logsCmd.Flags().String("bundle-id", "", "bundle identifier (default: from project config)")
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "stream app logs from a simulator",
	Long:  "streams app logs from a booted simulator. filters by bundle identifier and optional keyword.",
	Run: func(cmd *cobra.Command, args []string) {
		filter, _ := cmd.Flags().GetString("filter")
		bundleID, _ := cmd.Flags().GetString("bundle-id")
		flags := cliFlags()
		defaultSimulator := ""
		processName := ""

		_, cfg, _, err := loadProject(flags)
		if err == nil {
			defaultSimulator = cfg.Defaults.Simulator

			target, targetErr := selectedTarget(cfg, flags)
			if targetErr != nil {
				printTargetSelectionError(targetErr)
				return
			}

			if bundleID == "" {
				bundleID = target.BundleID
			}
			processName = target.Name
		} else if bundleID == "" {
			out.Error("cannot load config for bundle id", "error", err.Error(),
				"hint", "use --bundle-id to specify the bundle identifier directly")
			return
		}

		if bundleID == "" {
			out.Error("no bundle id available",
				"hint", "use --bundle-id or run from a project directory")
			return
		}

		// resolve simulator
		dev, err := device.ResolveSimulator(cmd.Context(), flags.Device, defaultSimulator)
		if err != nil {
			out.Error(err.Error())
			return
		}

		// boot if needed
		if err := dev.Prepare(cmd.Context()); err != nil {
			out.Error(err.Error())
			return
		}

		out.Info("streaming logs", "device", dev.Name(), "bundle_id", bundleID)

		streamLogs(cmd, dev.UDID(), bundleID, processName, filter)
	},
}

// streamLogs streams logs from a simulator using simctl spawn log stream.
// this is shared between cmd/logs.go and cmd/run.go (--logs flag).
func streamLogs(cmd *cobra.Command, udid, bundleID, processName, filter string) {
	predicate := buildLogPredicate(bundleID, processName, filter)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	logCmd := exec.CommandContext(ctx, "xcrun", "simctl", "spawn", udid,
		"log", "stream", "--style", "compact", "--predicate", predicate)

	stdout, err := logCmd.StdoutPipe()
	if err != nil {
		out.Error("cannot create log pipe", "error", err.Error())
		return
	}

	if err := logCmd.Start(); err != nil {
		out.Error("cannot start log stream", "error", err.Error(),
			"hint", "ensure the simulator is booted")
		return
	}

	jsonMode := viper.GetBool("json")
	scanner := bufio.NewScanner(stdout)
	// increase buffer for long log lines
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if jsonMode {
			data, _ := json.Marshal(map[string]string{
				"type":    "log",
				"message": line,
			})
			fmt.Fprintln(os.Stdout, string(data))
		} else {
			fmt.Fprintln(os.Stdout, line)
		}
	}

	if err := scanner.Err(); err != nil {
		out.Warn("log stream interrupted: " + err.Error())
	}

	// wait for process to finish (will return error on signal, which is expected)
	_ = logCmd.Wait()
}

// buildLogPredicate creates an NSPredicate string for log filtering.
func buildLogPredicate(bundleID, processName, filter string) string {
	terms := []string{fmt.Sprintf("subsystem == %q", bundleID)}
	if processName != "" {
		terms = append(terms,
			fmt.Sprintf("senderImagePath ENDSWITH[c] %q", "/"+processName),
			fmt.Sprintf("senderImagePath CONTAINS[c] %q", "/"+processName+".app/"),
		)
	}

	predicate := "(" + strings.Join(terms, " OR ") + ")"
	if filter != "" {
		predicate += fmt.Sprintf(" AND eventMessage CONTAINS[c] %q", filter)
	}
	return predicate
}
