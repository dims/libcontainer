package configs_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestUnmarshalHooks(t *testing.T) {
	timeout := time.Second

	hookCmd := configs.NewCommandHook(configs.Command{
		Path:    "/var/vcap/hooks/hook",
		Args:    []string{"--pid=123"},
		Env:     []string{"FOO=BAR"},
		Dir:     "/var/vcap",
		Timeout: &timeout,
	})

	hookJson, err := json.Marshal(hookCmd)
	if err != nil {
		t.Fatal(err)
	}

	for _, hookName := range configs.HookNameList {
		hooks := configs.Hooks{}
		err = hooks.UnmarshalJSON([]byte(fmt.Sprintf(`{"%s" :[%s]}`, hookName, hookJson)))
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(hooks[hookName], configs.HookList{hookCmd}) {
			t.Errorf("Expected %s to equal %+v but it was %+v", hookName, hookCmd, hooks[hookName])
		}
	}
}

func TestUnmarshalHooksWithInvalidData(t *testing.T) {
	hook := configs.Hooks{}
	err := hook.UnmarshalJSON([]byte(`{invalid-json}`))
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestMarshalHooks(t *testing.T) {
	timeout := time.Second

	hookCmd := configs.NewCommandHook(configs.Command{
		Path:    "/var/vcap/hooks/hook",
		Args:    []string{"--pid=123"},
		Env:     []string{"FOO=BAR"},
		Dir:     "/var/vcap",
		Timeout: &timeout,
	})

	hook := configs.Hooks{
		configs.Prestart:        configs.HookList{hookCmd},
		configs.CreateRuntime:   configs.HookList{hookCmd},
		configs.CreateContainer: configs.HookList{hookCmd},
		configs.StartContainer:  configs.HookList{hookCmd},
		configs.Poststart:       configs.HookList{hookCmd},
		configs.Poststop:        configs.HookList{hookCmd},
	}
	hooks, err := hook.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Note Marshal seems to output fields in alphabetical order
	hookCmdJson := `[{"path":"/var/vcap/hooks/hook","args":["--pid=123"],"env":["FOO=BAR"],"dir":"/var/vcap","timeout":1000000000}]`
	h := fmt.Sprintf(`{"createContainer":%[1]s,"createRuntime":%[1]s,"poststart":%[1]s,"poststop":%[1]s,"prestart":%[1]s,"startContainer":%[1]s}`, hookCmdJson)
	if string(hooks) != h {
		t.Errorf("Expected hooks %s to equal %s", string(hooks), h)
	}
}

func TestMarshalUnmarshalHooks(t *testing.T) {
	timeout := time.Second

	hookCmd := configs.NewCommandHook(configs.Command{
		Path:    "/var/vcap/hooks/hook",
		Args:    []string{"--pid=123"},
		Env:     []string{"FOO=BAR"},
		Dir:     "/var/vcap",
		Timeout: &timeout,
	})

	hook := configs.Hooks{
		configs.Prestart:        configs.HookList{hookCmd},
		configs.CreateRuntime:   configs.HookList{hookCmd},
		configs.CreateContainer: configs.HookList{hookCmd},
		configs.StartContainer:  configs.HookList{hookCmd},
		configs.Poststart:       configs.HookList{hookCmd},
		configs.Poststop:        configs.HookList{hookCmd},
	}
	hooks, err := hook.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	umMhook := configs.Hooks{}
	err = umMhook.UnmarshalJSON(hooks)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(umMhook, hook) {
		t.Errorf("Expected hooks to be equal after mashaling -> unmarshaling them: %+v, %+v", umMhook, hook)
	}
}

func TestMarshalHooksWithUnexpectedType(t *testing.T) {
	fHook := configs.NewFunctionHook(func(*specs.State) error {
		return nil
	})
	hook := configs.Hooks{
		configs.CreateRuntime: configs.HookList{fHook},
	}
	hooks, err := hook.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	h := `{"createContainer":null,"createRuntime":null,"poststart":null,"poststop":null,"prestart":null,"startContainer":null}`
	if string(hooks) != h {
		t.Errorf("Expected hooks %s to equal %s", string(hooks), h)
	}
}

func TestFuncHookRun(t *testing.T) {
	state := &specs.State{
		Version: "1",
		ID:      "1",
		Status:  "created",
		Pid:     1,
		Bundle:  "/bundle",
	}

	fHook := configs.NewFunctionHook(func(s *specs.State) error {
		if !reflect.DeepEqual(state, s) {
			t.Errorf("Expected state %+v to equal %+v", state, s)
		}
		return nil
	})

	err := fHook.Run(state)
	if err != nil {
		t.Errorf("Unexpected error running hook: %+v", err)
	}
}

func TestCommandHookRun(t *testing.T) {
	state := &specs.State{
		Version: "1",
		ID:      "1",
		Status:  "created",
		Pid:     1,
		Bundle:  "/bundle",
	}
	timeout := time.Second

	cmdHook := configs.NewCommandHook(configs.Command{
		Path:    os.Args[0],
		Args:    []string{os.Args[0], "-test.run=TestHelperProcess"},
		Env:     []string{"FOO=BAR"},
		Dir:     "/",
		Timeout: &timeout,
	})

	err := cmdHook.Run(state)
	if err != nil {
		t.Errorf(fmt.Sprintf("Expected error to not occur but it was %+v", err))
	}
}

func TestCommandHookRunTimeout(t *testing.T) {
	state := &specs.State{
		Version: "1",
		ID:      "1",
		Status:  "created",
		Pid:     1,
		Bundle:  "/bundle",
	}
	timeout := 10 * time.Millisecond

	cmdHook := configs.NewCommandHook(configs.Command{
		Path:    os.Args[0],
		Args:    []string{os.Args[0], "-test.run=TestHelperProcessWithTimeout"},
		Env:     []string{"FOO=BAR"},
		Dir:     "/",
		Timeout: &timeout,
	})

	err := cmdHook.Run(state)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestHelperProcess(*testing.T) {
	fmt.Println("Helper Process")
	os.Exit(0)
}
func TestHelperProcessWithTimeout(*testing.T) {
	time.Sleep(time.Second)
}
