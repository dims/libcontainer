package libcontainer

import (
	"fmt"
	"io"
	"math"
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
)

type processOperations interface {
	wait() (*os.ProcessState, error)
	signal(sig os.Signal) error
	pid() int
}

// Process specifies the configuration and IO for a process inside
// a container.
type Process struct {
	// The command to be run followed by any arguments.
	Args []string `json:"args"`

	// Env specifies the environment variables for the process.
	Env []string `json:"env,omitempty"`

	// User will set the uid and gid of the executing process running inside the container
	// local to the container's user and group configuration.
	User string `json:"user"`

	// AdditionalGroups specifies the gids that should be added to supplementary groups
	// in addition to those that the user belongs to.
	AdditionalGroups []string `json:"additional_groups,omitempty"`

	// Cwd will change the processes current working directory inside the container's rootfs.
	Cwd string `json:"cwd"`

	// Stdin is a pointer to a reader which provides the standard input stream.
	Stdin io.Reader `json:"-"`

	// Stdout is a pointer to a writer which receives the standard output stream.
	Stdout io.Writer `json:"-"`

	// Stderr is a pointer to a writer which receives the standard error stream.
	Stderr io.Writer `json:"-"`

	// ExtraFiles specifies additional open files to be inherited by the container
	ExtraFiles []*os.File `json:"-"`

	// consoleChan provides the masterfd console.
	// TODO: Make this persistent in Process.
	consoleChan chan *os.File

	// Capabilities specify the capabilities to keep when executing the process inside the container
	// All capabilities not specified will be dropped from the processes capability mask
	Capabilities []string `json:"capabilities,omitempty"`

	// AppArmorProfile specifies the profile to apply to the process and is
	// changed at the time the process is execed
	AppArmorProfile string `json:"apparmor_profile,omitempty"`

	// Label specifies the label to apply to the process.  It is commonly used by selinux
	Label string `json:"label,omitempty"`

	// NoNewPrivileges controls whether processes can gain additional privileges.
	NoNewPrivileges *bool `json:"no_new_privileges,omitempty"`

	// Rlimits specifies the resource limits, such as max open files, to set in the container
	// If Rlimits are not set, the container will inherit rlimits from the parent process
	Rlimits []configs.Rlimit `json:"rlimits,omitempty"`

	ops processOperations
}

// Wait waits for the process to exit.
// Wait releases any resources associated with the Process
func (p Process) Wait() (*os.ProcessState, error) {
	if p.ops == nil {
		return nil, newGenericError(fmt.Errorf("invalid process"), NoProcessOps)
	}
	return p.ops.wait()
}

// Pid returns the process ID
func (p Process) Pid() (int, error) {
	// math.MinInt32 is returned here, because it's invalid value
	// for the kill() system call.
	if p.ops == nil {
		return math.MinInt32, newGenericError(fmt.Errorf("invalid process"), NoProcessOps)
	}
	return p.ops.pid(), nil
}

// Signal sends a signal to the Process.
func (p Process) Signal(sig os.Signal) error {
	if p.ops == nil {
		return newGenericError(fmt.Errorf("invalid process"), NoProcessOps)
	}
	return p.ops.signal(sig)
}

// IO holds the process's STDIO
type IO struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

func (p *Process) GetConsole() (Console, error) {
	consoleFd, ok := <-p.consoleChan
	if !ok {
		return nil, fmt.Errorf("failed to get console from process")
	}

	// TODO: Fix this so that it used the console API.
	return &linuxConsole{
		master: consoleFd,
	}, nil
}
