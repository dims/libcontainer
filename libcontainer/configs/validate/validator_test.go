package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

func TestValidate(t *testing.T) {
	config := &configs.Config{
		Rootfs: "/var",
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateWithInvalidRootfs(t *testing.T) {
	dir := "rootfs"
	if err := os.Symlink("/var", dir); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir)

	config := &configs.Config{
		Rootfs: dir,
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateNetworkWithoutNETNamespace(t *testing.T) {
	network := &configs.Network{Type: "loopback"}
	config := &configs.Config{
		Rootfs:     "/var",
		Namespaces: []configs.Namespace{},
		Networks:   []*configs.Network{network},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateNetworkRoutesWithoutNETNamespace(t *testing.T) {
	route := &configs.Route{Gateway: "255.255.255.0"}
	config := &configs.Config{
		Rootfs:     "/var",
		Namespaces: []configs.Namespace{},
		Routes:     []*configs.Route{route},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateHostname(t *testing.T) {
	config := &configs.Config{
		Rootfs:   "/var",
		Hostname: "runc",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUTS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateUTS(t *testing.T) {
	config := &configs.Config{
		Rootfs:     "/var",
		Domainname: "runc",
		Hostname:   "runc",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUTS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateUTSWithoutUTSNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:   "/var",
		Hostname: "runc",
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}

	config = &configs.Config{
		Rootfs:     "/var",
		Domainname: "runc",
	}

	err = Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateSecurityWithMaskPaths(t *testing.T) {
	config := &configs.Config{
		Rootfs:    "/var",
		MaskPaths: []string{"/proc/kcore"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWNS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateSecurityWithROPaths(t *testing.T) {
	config := &configs.Config{
		Rootfs:        "/var",
		ReadonlyPaths: []string{"/proc/sys"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWNS},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("Expected error to not occur: %+v", err)
	}
}

func TestValidateSecurityWithoutNEWNS(t *testing.T) {
	config := &configs.Config{
		Rootfs:        "/var",
		MaskPaths:     []string{"/proc/kcore"},
		ReadonlyPaths: []string{"/proc/sys"},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateUsernamespace(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}
	config := &configs.Config{
		Rootfs: "/var",
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{Type: configs.NEWUSER},
			},
		),
	}

	err := Validate(config)
	if err != nil {
		t.Errorf("expected error to not occur %+v", err)
	}
}

func TestValidateUsernamespaceWithoutUserNS(t *testing.T) {
	uidMap := configs.IDMap{ContainerID: 123}
	config := &configs.Config{
		Rootfs:      "/var",
		UIDMappings: []configs.IDMap{uidMap},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

// TestConvertSysctlVariableToDotsSeparator tests whether the sysctl variable
// can be correctly converted to a dot as a separator.
func TestConvertSysctlVariableToDotsSeparator(t *testing.T) {
	type testCase struct {
		in  string
		out string
	}
	valid := []testCase{
		{in: "kernel.shm_rmid_forced", out: "kernel.shm_rmid_forced"},
		{in: "kernel/shm_rmid_forced", out: "kernel.shm_rmid_forced"},
		{in: "net.ipv4.conf.eno2/100.rp_filter", out: "net.ipv4.conf.eno2/100.rp_filter"},
		{in: "net/ipv4/conf/eno2.100/rp_filter", out: "net.ipv4.conf.eno2/100.rp_filter"},
		{in: "net/ipv4/ip_local_port_range", out: "net.ipv4.ip_local_port_range"},
		{in: "kernel/msgmax", out: "kernel.msgmax"},
		{in: "kernel/sem", out: "kernel.sem"},
	}

	for _, test := range valid {
		convertSysctlVal := convertSysctlVariableToDotsSeparator(test.in)
		if convertSysctlVal != test.out {
			t.Errorf("The sysctl variable was not converted correctly. got: %s, want: %s", convertSysctlVal, test.out)
		}
	}
}

func TestValidateSysctl(t *testing.T) {
	sysctl := map[string]string{
		"fs.mqueue.ctl":                    "ctl",
		"fs/mqueue/ctl":                    "ctl",
		"net.ctl":                          "ctl",
		"net/ctl":                          "ctl",
		"net.ipv4.conf.eno2/100.rp_filter": "ctl",
		"kernel.ctl":                       "ctl",
		"kernel/ctl":                       "ctl",
	}

	for k, v := range sysctl {
		config := &configs.Config{
			Rootfs: "/var",
			Sysctl: map[string]string{k: v},
		}

		err := Validate(config)
		if err == nil {
			t.Error("Expected error to occur but it was nil")
		}
	}
}

func TestValidateValidSysctl(t *testing.T) {
	sysctl := map[string]string{
		"fs.mqueue.ctl":                    "ctl",
		"fs/mqueue/ctl":                    "ctl",
		"net.ctl":                          "ctl",
		"net/ctl":                          "ctl",
		"net.ipv4.conf.eno2/100.rp_filter": "ctl",
		"kernel.msgmax":                    "ctl",
		"kernel/msgmax":                    "ctl",
	}

	for k, v := range sysctl {
		config := &configs.Config{
			Rootfs: "/var",
			Sysctl: map[string]string{k: v},
			Namespaces: []configs.Namespace{
				{
					Type: configs.NEWNET,
				},
				{
					Type: configs.NEWIPC,
				},
			},
		}

		err := Validate(config)
		if err != nil {
			t.Errorf("Expected error to not occur with {%s=%s} but got: %q", k, v, err)
		}
	}
}

func TestValidateSysctlWithSameNs(t *testing.T) {
	config := &configs.Config{
		Rootfs: "/var",
		Sysctl: map[string]string{"net.ctl": "ctl"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{
					Type: configs.NEWNET,
					Path: "/proc/self/ns/net",
				},
			},
		),
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateSysctlWithBindHostNetNS(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}

	const selfnet = "/proc/self/ns/net"

	file := filepath.Join(t.TempDir(), "default")
	fd, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file)
	fd.Close()

	if err := unix.Mount(selfnet, file, "bind", unix.MS_BIND, ""); err != nil {
		t.Fatalf("can't bind-mount %s to %s: %s", selfnet, file, err)
	}
	defer func() {
		_ = unix.Unmount(file, unix.MNT_DETACH)
	}()

	config := &configs.Config{
		Rootfs: "/var",
		Sysctl: map[string]string{"net.ctl": "ctl", "net.foo": "bar"},
		Namespaces: configs.Namespaces(
			[]configs.Namespace{
				{
					Type: configs.NEWNET,
					Path: file,
				},
			},
		),
	}

	if err := Validate(config); err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateSysctlWithoutNETNamespace(t *testing.T) {
	config := &configs.Config{
		Rootfs:     "/var",
		Sysctl:     map[string]string{"net.ctl": "ctl"},
		Namespaces: []configs.Namespace{},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateMounts(t *testing.T) {
	testCases := []struct {
		isErr bool
		dest  string
	}{
		{isErr: true, dest: "not/an/abs/path"},
		{isErr: true, dest: "./rel/path"},
		{isErr: true, dest: "./rel/path"},
		{isErr: true, dest: "../../path"},

		{isErr: false, dest: "/abs/path"},
		{isErr: false, dest: "/abs/but/../unclean"},
	}

	for _, tc := range testCases {
		config := &configs.Config{
			Rootfs: "/var",
			Mounts: []*configs.Mount{
				{Destination: tc.dest},
			},
		}

		err := Validate(config)
		if tc.isErr && err == nil {
			t.Errorf("mount dest: %s, expected error, got nil", tc.dest)
		}
		if !tc.isErr && err != nil {
			t.Errorf("mount dest: %s, expected nil, got error %v", tc.dest, err)
		}
	}
}

func TestValidateIDMapMounts(t *testing.T) {
	mapping := []configs.IDMap{
		{
			ContainerID: 0,
			HostID:      10000,
			Size:        1,
		},
	}

	testCases := []struct {
		name   string
		isErr  bool
		config *configs.Config
	}{
		{
			name:  "idmap mount without bind opt specified",
			isErr: true,
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						UIDMappings: mapping,
						GIDMappings: mapping,
					},
				},
			},
		},
		{
			name:  "rootless idmap mount",
			isErr: true,
			config: &configs.Config{
				RootlessEUID: true,
				UIDMappings:  mapping,
				GIDMappings:  mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						UIDMappings: mapping,
						GIDMappings: mapping,
					},
				},
			},
		},
		{
			name:  "idmap mounts without abs dest path",
			isErr: true,
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "./rel/path/",
						Flags:       unix.MS_BIND,
						UIDMappings: mapping,
						GIDMappings: mapping,
					},
				},
			},
		},
		{
			name: "simple idmap mount",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/another-abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						UIDMappings: mapping,
						GIDMappings: mapping,
					},
				},
			},
		},
		{
			name: "idmap mount with more flags",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/another-abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND | unix.MS_RDONLY,
						UIDMappings: mapping,
						GIDMappings: mapping,
					},
				},
			},
		},
		{
			name: "idmap mount without userns mappings",
			config: &configs.Config{
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						UIDMappings: mapping,
						GIDMappings: mapping,
					},
				},
			},
		},
		{
			name: "idmap mounts with different userns and mount mappings",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						UIDMappings: []configs.IDMap{
							{
								ContainerID: 10,
								HostID:      10,
								Size:        1,
							},
						},
						GIDMappings: mapping,
					},
				},
			},
		},
		{
			name: "idmap mounts with different userns and mount mappings",
			config: &configs.Config{
				UIDMappings: mapping,
				GIDMappings: mapping,
				Mounts: []*configs.Mount{
					{
						Source:      "/abs/path/",
						Destination: "/abs/path/",
						Flags:       unix.MS_BIND,
						UIDMappings: mapping,
						GIDMappings: []configs.IDMap{
							{
								ContainerID: 10,
								HostID:      10,
								Size:        1,
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			config := tc.config
			config.Rootfs = "/var"

			err := mounts(config)
			if tc.isErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tc.isErr && err != nil {
				t.Error(err)
			}
		})
	}
}
