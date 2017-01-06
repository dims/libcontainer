#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
  setup_fifos
}

function teardown() {
  teardown_busybox
  teardown_fifos
}

@test "runc exec" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec test_busybox echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]
}

@test "runc exec --pid-file" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec --pid-file pid.txt test_busybox echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
  [[ ${lines[0]} != $(__runc state test_busybox | jq '.pid') ]]
}

@test "runc exec --pid-file with new CWD" {
  # create pid_file directory as the CWD
  run mkdir pid_file
  [ "$status" -eq 0 ]
  run cd pid_file
  [ "$status" -eq 0 ]

  # run busybox detached
  runc run -d -b $BUSYBOX_BUNDLE --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec --pid-file pid.txt test_busybox echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
  [[ ${lines[0]} != $(__runc state test_busybox | jq '.pid') ]]
}

@test "runc exec ls -la" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec test_busybox ls -la
  [ "$status" -eq 0 ]
  [[ ${lines[0]} == *"total"* ]]
  [[ ${lines[1]} == *"."* ]]
  [[ ${lines[2]} == *".."* ]]
}

@test "runc exec ls -la with --cwd" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec --cwd /bin test_busybox pwd
  [ "$status" -eq 0 ]
  [[ ${output} == "/bin" ]]
}

@test "runc exec --env" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec --env RUNC_EXEC_TEST=true test_busybox env
  [ "$status" -eq 0 ]

  [[ ${output} == *"RUNC_EXEC_TEST=true"* ]]
}

@test "runc exec --user" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec --user 1000:1000 test_busybox id
  [ "$status" -eq 0 ]

  [[ ${output} == "uid=1000 gid=1000" ]]
}

@test "runc exec --stdin" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  echo thisisstdin > ./stdinfifo &

  runc exec --stdin=./stdinfifo test_busybox sh -c "cat <&0"

  [ "$status" -eq 0 ]
  [[ "${output}" == "thisisstdin" ]]
}

@test "runc exec --stdout" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  cat ./stdoutfifo > stdoutoutput &

  runc exec --stdout=./stdoutfifo test_busybox echo thisisstdout

  [ "$status" -eq 0 ]
  [[ "$(cat stdoutoutput)" == "thisisstdout" ]]
}

@test "runc exec --stderr" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  cat ./stderrfifo > stderroutput &

  runc exec --stderr=./stderrfifo test_busybox sh -c "1>&2 echo thisisstderr"

  [ "$status" -eq 0 ]
  [[ "$(cat stderroutput)" == "thisisstderr" ]]
}
