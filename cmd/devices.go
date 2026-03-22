package cmd

import (
	"cmp"
	"slices"

	"github.com/kacyfortner/ios-build-cli/internal/device"
	"github.com/kacyfortner/ios-build-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	devicesCmd.Flags().Bool("booted", false, "show only booted simulators")
	devicesCmd.Flags().Bool("physical", false, "show physical devices")
	rootCmd.AddCommand(devicesCmd)
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "list available simulators and devices",
	Long:  "lists ios simulators available for deployment. use --booted to show only running simulators.",
	Run: func(cmd *cobra.Command, args []string) {
		physical, _ := cmd.Flags().GetBool("physical")
		if physical {
			out.Warn("physical device listing is not yet supported")
			return
		}

		booted, _ := cmd.Flags().GetBool("booted")

		sims, err := device.ListSimulators(cmd.Context())
		if err != nil {
			out.Error(err.Error())
			return
		}

		if booted {
			var filtered []device.SimulatorInfo
			for _, s := range sims {
				if s.State == device.StateBooted {
					filtered = append(filtered, s)
				}
			}
			sims = filtered
		}

		if len(sims) == 0 {
			out.Info("no simulators found")
			return
		}

		// group by runtime for human output
		groups := groupByRuntime(sims)
		for _, g := range groups {
			out.Info(g.runtime, "count", len(g.sims))
			for _, s := range g.sims {
				out.Data("simulator", output.OrderedMap{
					{Key: "name", Value: s.Name},
					{Key: "udid", Value: s.UDID},
					{Key: "state", Value: s.State},
					{Key: "runtime", Value: s.Runtime},
				})
			}
		}
	},
}

type runtimeGroup struct {
	runtime string
	sims    []device.SimulatorInfo
}

// groupByRuntime groups simulators by runtime, preserving encounter order.
func groupByRuntime(sims []device.SimulatorInfo) []runtimeGroup {
	order := []string{}
	groups := map[string][]device.SimulatorInfo{}

	for _, s := range sims {
		if _, seen := groups[s.Runtime]; !seen {
			order = append(order, s.Runtime)
		}
		groups[s.Runtime] = append(groups[s.Runtime], s)
	}

	// newest runtime first
	slices.SortFunc(order, func(a, b string) int {
		return cmp.Compare(b, a)
	})

	result := make([]runtimeGroup, len(order))
	for i, rt := range order {
		result[i] = runtimeGroup{runtime: rt, sims: groups[rt]}
	}
	return result
}
