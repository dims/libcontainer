module github.com/dims/libcontainer

go 1.23.2

require (
	github.com/cilium/ebpf v0.7.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/cyphar/filepath-securejoin v0.2.4
	github.com/godbus/dbus/v5 v5.0.6
	github.com/moby/sys/mountinfo v0.5.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/selinux v1.10.0
	github.com/seccomp/libseccomp-golang v0.9.2-0.20220502022130-f33da4d89646
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/net v0.24.0
	golang.org/x/sys v0.19.0
)

require github.com/google/go-cmp v0.5.8 // indirect
