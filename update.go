// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
)

func u64Ptr(i uint64) *uint64 { return &i }
func u16Ptr(i uint16) *uint16 { return &i }

var updateCmd = &cobra.Command{
	Short: "update container resource constraints",
	Use:   `update [command options] <container-id>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()

		container, err := getContainer(flags, args)
		if err != nil {
			return err
		}

		r := specs.Resources{
			Memory: &specs.Memory{
				Limit:       u64Ptr(0),
				Reservation: u64Ptr(0),
				Swap:        u64Ptr(0),
				Kernel:      u64Ptr(0),
				KernelTCP:   u64Ptr(0),
			},
			CPU: &specs.CPU{
				Shares: u64Ptr(0),
				Quota:  u64Ptr(0),
				Period: u64Ptr(0),
				Cpus:   sPtr(""),
				Mems:   sPtr(""),
			},
			BlockIO: &specs.BlockIO{
				Weight: u16Ptr(0),
			},
		}

		config := container.Config()

		if in, _ := flags.GetString("resources"); in != "" {
			var (
				f   *os.File
				err error
			)
			switch in {
			case "-":
				f = os.Stdin
			default:
				f, err = os.Open(in)
				if err != nil {
					return err
				}
			}
			err = json.NewDecoder(f).Decode(&r)
			if err != nil {
				return err
			}
		} else {
			if val, _ := flags.GetInt("blkio-weight"); val != 0 {
				r.BlockIO.Weight = u16Ptr(uint16(val))
			}
			if val, _ := flags.GetString("cpuset-cpus"); val != "" {
				r.CPU.Cpus = &val
			}
			if val, _ := flags.GetString("cpuset-mems"); val != "" {
				r.CPU.Mems = &val
			}

			for _, pair := range []struct {
				opt  string
				dest *uint64
			}{

				{"cpu-period", r.CPU.Period},
				{"cpu-quota", r.CPU.Quota},
				{"cpu-share", r.CPU.Shares},
			} {
				if val, _ := flags.GetString(pair.opt); val != "" {
					var err error
					*pair.dest, err = strconv.ParseUint(val, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid value for %s: %s", pair.opt, err)
					}
				}
			}
			for _, pair := range []struct {
				opt  string
				dest *uint64
			}{
				{"kernel-memory", r.Memory.Kernel},
				{"kernel-memory-tcp", r.Memory.KernelTCP},
				{"memory", r.Memory.Limit},
				{"memory-reservation", r.Memory.Reservation},
				{"memory-swap", r.Memory.Swap},
			} {
				if val, _ := flags.GetString(pair.opt); val != "" {
					v, err := units.RAMInBytes(val)
					if err != nil {
						return fmt.Errorf("invalid value for %s: %s", pair.opt, err)
					}
					*pair.dest = uint64(v)
				}
			}
		}

		// Update the value
		config.Cgroups.Resources.BlkioWeight = *r.BlockIO.Weight
		config.Cgroups.Resources.CpuPeriod = int64(*r.CPU.Period)
		config.Cgroups.Resources.CpuQuota = int64(*r.CPU.Quota)
		config.Cgroups.Resources.CpuShares = int64(*r.CPU.Shares)
		config.Cgroups.Resources.CpusetCpus = *r.CPU.Cpus
		config.Cgroups.Resources.CpusetMems = *r.CPU.Mems
		config.Cgroups.Resources.KernelMemory = int64(*r.Memory.Kernel)
		config.Cgroups.Resources.KernelMemoryTCP = int64(*r.Memory.KernelTCP)
		config.Cgroups.Resources.Memory = int64(*r.Memory.Limit)
		config.Cgroups.Resources.MemoryReservation = int64(*r.Memory.Reservation)
		config.Cgroups.Resources.MemorySwap = int64(*r.Memory.Swap)

		if err := container.Set(config); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	flags := updateCmd.Flags()

	flags.StringP("resources", "r", "",
		`path to the file containing the resources to update or '-' to read from the standard input

The accepted format is as follow (unchanged values can be omitted):

{
  "memory": {
    "limit": 0,
    "reservation": 0,
    "swap": 0,
    "kernel": 0,
    "kernelTCP": 0
  },
  "cpu": {
    "shares": 0,
    "quota": 0,
    "period": 0,
    "cpus": "",
    "mems": ""
  },
  "blockIO": {
    "blkioWeight": 0
  },
}

Note: if data is to be read from a file or the standard input, all
other options are ignored.
`)

	flags.Int("blkio-weight", 0, "Specifies per cgroup weight, range is from 10 to 1000")
	flags.String("cpu-period", "", "CPU period to be used for hardcapping (in usecs). 0 to use system default")
	flags.String("cpu-quota", "", "CPU hardcap limit (in usecs). Allowed cpu time in a given period")
	flags.String("cpu-share", "", "CPU shares (relative weight vs. other containers)")
	flags.String("cpuset-cpus", "", "CPU(s) to use")
	flags.String("cpuset-mems", "", "Memory node(s) to use")
	flags.String("kernel-memory", "", "Kernel memory limit (in bytes)")
	flags.String("kernel-memory-tcp", "", "Kernel memory limit (in bytes) for tcp buffer")
	flags.String("memory", "", "Memory limit (in bytes)")
	flags.String("memory-reservation", "", "Memory reservation or soft_limit (in bytes)")
	flags.String("memory-swap", "", "Total memory usage (memory + swap); set '-1' to enable unlimited swap")
}
