package cmd

import (
	"cmp"
	"slices"
	"sync"

	"github.com/kacy/xless/internal/device"
	"github.com/kacy/xless/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	devicesCmd.Flags().Bool("booted", false, "show only booted simulators")
	devicesCmd.Flags().Bool("physical", false, "show only physical devices")
	devicesCmd.Flags().Bool("simulators", false, "show only simulators")
	rootCmd.AddCommand(devicesCmd)
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "list available simulators and devices",
	Long:  "lists ios simulators and physical devices available for deployment.",
	Run: func(cmd *cobra.Command, args []string) {
		physical, _ := cmd.Flags().GetBool("physical")
		simulators, _ := cmd.Flags().GetBool("simulators")
		booted, _ := cmd.Flags().GetBool("booted")

		// when neither flag set, show both
		showSimulators := !physical || simulators
		showPhysical := !simulators || physical

		// if both flags explicitly set, show both
		if physical && simulators {
			showSimulators = true
			showPhysical = true
		}

		if showSimulators && showPhysical {
			// fetch both concurrently, print sequentially
			var wg sync.WaitGroup
			var sims []device.SimulatorInfo
			var simsErr error
			var devs []device.PhysicalDeviceInfo
			var devsErr error

			wg.Add(2)
			go func() {
				defer wg.Done()
				sims, simsErr = device.ListSimulators(cmd.Context())
			}()
			go func() {
				defer wg.Done()
				devs, devsErr = device.ListPhysicalDevices(cmd.Context())
			}()
			wg.Wait()

			printSimulators(sims, simsErr, booted)
			printPhysicalDevices(devs, devsErr)
			return
		}

		if showSimulators {
			sims, err := device.ListSimulators(cmd.Context())
			printSimulators(sims, err, booted)
		}

		if showPhysical {
			devs, err := device.ListPhysicalDevices(cmd.Context())
			printPhysicalDevices(devs, err)
		}
	},
}

func printSimulators(sims []device.SimulatorInfo, err error, booted bool) {
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
}

func printPhysicalDevices(devices []device.PhysicalDeviceInfo, err error) {
	if err != nil {
		out.Warn("could not list physical devices: " + err.Error())
		return
	}

	if len(devices) == 0 {
		out.Info("no physical devices found")
		return
	}

	out.Info("physical devices", "count", len(devices))
	for _, d := range devices {
		state := "disconnected"
		if d.Connected {
			state = "connected"
		}
		out.Data("device", output.OrderedMap{
			{Key: "name", Value: d.Name},
			{Key: "udid", Value: d.UDID},
			{Key: "type", Value: d.DeviceType},
			{Key: "transport", Value: d.TransportType},
			{Key: "state", Value: state},
		})
	}
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
