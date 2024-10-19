module github.com/dims/libcontainer

go 1.23.2

require (
	github.com/cilium/ebpf v0.16.0
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/cyphar/filepath-securejoin v0.3.4
	github.com/godbus/dbus/v5 v5.1.0
	github.com/moby/sys/mountinfo v0.7.2
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/opencontainers/selinux v1.11.1
	github.com/seccomp/libseccomp-golang v0.10.0
	github.com/sirupsen/logrus v1.9.3
	golang.org/x/net v0.30.0
	golang.org/x/sys v0.26.0
)

require golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2 // indirect
