// +build linux

package fs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type IOGroupV2 struct {
}

func (s *IOGroupV2) Name() string {
	return "io"
}

func (s *IOGroupV2) Apply(d *cgroupData) error {
	_, err := d.join("io")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *IOGroupV2) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.BlkioWeight != 0 {
		filename := "io.bfq.weight"
		if err := writeFile(path, filename, strconv.FormatUint(uint64(cgroup.Resources.BlkioWeight), 10)); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleReadBpsDevice {
		if err := writeFile(path, "io.max", td.StringName("rbps")); err != nil {
			return err
		}
	}
	for _, td := range cgroup.Resources.BlkioThrottleWriteBpsDevice {
		if err := writeFile(path, "io.max", td.StringName("wbps")); err != nil {
			return err
		}
	}
	for _, td := range cgroup.Resources.BlkioThrottleReadIOPSDevice {
		if err := writeFile(path, "io.max", td.StringName("riops")); err != nil {
			return err
		}
	}
	for _, td := range cgroup.Resources.BlkioThrottleWriteIOPSDevice {
		if err := writeFile(path, "io.max", td.StringName("wiops")); err != nil {
			return err
		}
	}

	return nil
}

func (s *IOGroupV2) Remove(d *cgroupData) error {
	return removePath(d.path("io"))
}

func readCgroup2MapFile(path string, name string) (map[string][]string, error) {
	ret := map[string][]string{}
	p := filepath.Join("/sys/fs/cgroup", path, name)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ret, nil
		}
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ret[parts[0]] = parts[1:]
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *IOGroupV2) getCgroupV2Stats(path string, stats *cgroups.Stats) error {
	// more details on the io.stat file format: https://www.kernel.org/doc/Documentation/cgroup-v2.txt
	var ioServiceBytesRecursive []cgroups.BlkioStatEntry
	values, err := readCgroup2MapFile(path, "io.stat")
	if err != nil {
		return err
	}
	for k, v := range values {
		d := strings.Split(k, ":")
		if len(d) != 2 {
			continue
		}
		minor, err := strconv.ParseUint(d[0], 10, 0)
		if err != nil {
			return err
		}
		major, err := strconv.ParseUint(d[1], 10, 0)
		if err != nil {
			return err
		}

		for _, item := range v {
			d := strings.Split(item, "=")
			if len(d) != 2 {
				continue
			}
			op := d[0]

			// Accommodate the cgroup v1 naming
			switch op {
			case "rbytes":
				op = "read"
			case "wbytes":
				op = "write"
			}

			value, err := strconv.ParseUint(d[1], 10, 0)
			if err != nil {
				return err
			}

			entry := cgroups.BlkioStatEntry{
				Op:    op,
				Major: major,
				Minor: minor,
				Value: value,
			}
			ioServiceBytesRecursive = append(ioServiceBytesRecursive, entry)
		}
	}
	stats.BlkioStats = cgroups.BlkioStats{IoServiceBytesRecursive: ioServiceBytesRecursive}
	return nil
}

func (s *IOGroupV2) GetStats(path string, stats *cgroups.Stats) error {
	return s.getCgroupV2Stats(path, stats)
}
