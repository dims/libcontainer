#!/usr/bin/env bats

load helpers

# get the cgroup paths
for g in DEVICES MEMORY CPUSET CPU BLKIO; do
	base_path=$(grep "cgroup"  /proc/self/mountinfo | gawk 'toupper($NF) ~ /\<'${g}'\>/ { print $5; exit }')
	eval CGROUP_${g}="${base_path}/runc-update-integration-test"
done

DEFAULT_CGROUP_DATA=$(cat <<EOF
    "memory": {
        "limit": 33554432,
        "reservation": 25165824,
        "kernel": 16777216,
        "kernelTCP": 11534336
    },
    "cpu": {
        "shares": 100,
        "quota": 500000,
        "period": 1000000,
        "cpus": "0"
    },
    "blockio": {
        "blkioWeight": 1000
    }
EOF
    )


function teardown() {
    rm -f $BATS_TMPDIR/runc-update-integration-test.json
    teardown_running_container test_update
    teardown_busybox
}

function setup() {
    teardown
    setup_busybox

    # Add cgroup path
    sed -i 's/\("linux": {\)/\1\n    "cgroupsPath": "\/runc-update-integration-test",/'  ${BUSYBOX_BUNDLE}/config.json

    # Set some initial known values
    DATA=$(echo ${DEFAULT_CGROUP_DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA},/" ${BUSYBOX_BUNDLE}/config.json
}

function check_cgroup_value() {
    cgroup=$1
    source=$2
    expected=$3

    current=$(cat $cgroup/$source)
    [ "$current" -eq "$expected" ]
}

function contains_cgroup_value() {
    cgroup=$1
    source=$2
    expected=$3

    grep "$expected" $cgroup/$source
    [[ ${line[0]} =~ "$expected" ]]
}

# TODO: test rt cgroup updating
@test "update" {
    requires cgroups_kmem
    # run a few busyboxes detached
    runc run -d --console /dev/pts/ptmx test_update
    [ "$status" -eq 0 ]
    wait_for_container 15 1 test_update

    # check that initial values were properly set
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 1000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 1000000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 500000
    check_cgroup_value $CGROUP_CPU "cpu.shares" 100
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 0
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 16777216
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 11534336
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 33554432
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 25165824

    # update blkio-weight
    runc update test_update --blkio-weight 500
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 500

    # update cpu-period
    runc update test_update --cpu-period 900000
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 900000

    # update cpu-quota
    runc update test_update --cpu-quota 600000
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 600000

    # update cpu-shares
    runc update test_update --cpu-share 200
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.shares" 200

    # update cpuset if supported (i.e. we're running on a multicore cpu)
    cpu_count=$(grep '^processor' /proc/cpuinfo | wc -l)
    if [ $cpu_count -gt 1 ]; then
        runc update test_update --cpuset-cpus "1"
        [ "$status" -eq 0 ]
        check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 1
    fi

    # update memory limit
    runc update test_update --memory 67108864
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 67108864

    runc update test_update --memory 50M
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 52428800


    # update memory soft limit
    runc update test_update --memory-reservation 33554432
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 33554432

    # update memory swap (if available)
    if [ -f "$CGROUP_MEMORY/memory.memsw.limit_in_bytes" ]; then
        runc update test_update --memory-swap 96468992
        [ "$status" -eq 0 ]
        check_cgroup_value $CGROUP_MEMORY "memory.memsw.limit_in_bytes" 96468992
    fi

    # update kernel memory limit
    runc update test_update --kernel-memory 50331648
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 50331648

    # update kernel memory tcp limit
    runc update test_update --kernel-memory-tcp 41943040
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 41943040

    # redo all the changes at once
    runc update test_update --blkio-weight 500 \
        --cpu-period 900000 --cpu-quota 600000 --cpu-share 200 --memory 67108864 \
        --memory-reservation 33554432 --kernel-memory 50331648 --kernel-memory-tcp 41943040
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 500
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 900000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 600000
    check_cgroup_value $CGROUP_CPU "cpu.shares" 200
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 50331648
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 41943040
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 67108864
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 33554432

}

@test "update from json file or stdin" {
    requires cgroups_kmem
    # run a few busyboxes detached
    runc run -d --console /dev/pts/ptmx test_update
    [ "$status" -eq 0 ]
    wait_for_container 15 1 test_update

    # Revert to the test initial value via json on stding
    runc update  -r - test_update <<EOF
{
  "devices": [
    {   
      "major": 1,
      "minor": 3, 
      "type": "c",
	  "allow": false,
	  "access": "rwm"
    }   
  ],  
  "memory": {
    "limit": 29999104,
    "reservation": 24158208,
    "kernel": 12996608,
    "kernelTCP": 12996608
  },
  "cpu": {
    "shares": 101,
    "quota": 500001,
    "period": 999999,
    "cpus": "0",
    "mems": "0"
  },
  "blockIO": {
    "blkioWeight": 999,
    "blkioLeafWeight": 801
  },
  "pids": {},
  "hugepageLimits": [],
  "network": {}
}
EOF
    [ "$status" -eq 0 ]
    ! contains_cgroup_value $CGROUP_DEVICES "devices.list" "c 1:3 rwm"
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 999
    check_cgroup_value $CGROUP_BLKIO "blkio.leaf_weight" 801
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 999999
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 500001
    check_cgroup_value $CGROUP_CPU "cpu.shares" 101
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 0
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 12996608
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 12996608
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 29999104
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 24158208

    # reset to initial test value via json file
    echo -e "{\n${DEFAULT_CGROUP_DATA}\n}" > $BATS_TMPDIR/runc-update-integration-test.json

    runc update  -r $BATS_TMPDIR/runc-update-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 1000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 1000000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 500000
    check_cgroup_value $CGROUP_CPU "cpu.shares" 100
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 0
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 16777216
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 11534336
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 33554432
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 25165824
}
