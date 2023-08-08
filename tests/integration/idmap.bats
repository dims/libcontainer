#!/usr/bin/env bats

load helpers

OVERFLOW_UID="$(cat /proc/sys/kernel/overflowuid)"
OVERFLOW_GID="$(cat /proc/sys/kernel/overflowgid)"

function setup() {
	requires root
	requires_kernel 5.12
	requires_idmap_fs /tmp

	setup_debian

	# Prepare source folders for mounts.
	mkdir -p source-{1,2,multi{1,2,3}}/
	touch source-{1,2,multi{1,2,3}}/foo.txt
	touch source-multi{1,2,3}/{bar,baz}.txt

	# Change the owners for everything other than source-1.
	chown 1:1 source-2/foo.txt

	chown 100:211 source-multi1/foo.txt
	chown 101:222 source-multi1/bar.txt
	chown 102:233 source-multi1/baz.txt

	# Same gids as multi1, different uids.
	chown 200:211 source-multi2/foo.txt
	chown 201:222 source-multi2/bar.txt
	chown 202:233 source-multi2/baz.txt

	# 1000 uids, 500 gids
	chown 5000528:6000491 source-multi3/foo.txt
	chown 5000133:6000337 source-multi3/bar.txt
	chown 5000999:6000444 source-multi3/baz.txt
}

function teardown() {
	teardown_bundle
}

function setup_idmap_userns() {
	update_config '.linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]
		| .linux.gidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]'
}

function setup_bind_mount() {
	mountname="${1:-1}"
	update_config '.mounts += [
			{
				"source": "source-'"$mountname"'/",
				"destination": "/tmp/bind-mount-'"$mountname"'",
				"options": ["bind"]
			}
		]'
}

function setup_idmap_single_mount() {
	uidmap="$1"       # ctr:host:size
	gidmap="${2:-$1}" # ctr:host:size
	mountname="${3:-1}"
	destname="${4:-$mountname}"

	read -r uid_containerID uid_hostID uid_size <<<"$(tr : ' ' <<<"$uidmap")"
	read -r gid_containerID gid_hostID gid_size <<<"$(tr : ' ' <<<"$gidmap")"

	update_config '.mounts += [
			{
				"source": "source-'"$mountname"'/",
				"destination": "/tmp/mount-'"$destname"'",
				"options": ["bind"],
				"uidMappings": [{"containerID": '"$uid_containerID"', "hostID": '"$uid_hostID"', "size": '"$uid_size"'}],
				"gidMappings": [{"containerID": '"$gid_containerID"', "hostID": '"$gid_hostID"', "size": '"$gid_size"'}]
			}
		]'
}

function setup_idmap_basic_mount() {
	mountname="${1:-1}"
	setup_idmap_single_mount 0:100000:65536 0:100000:65536 "$mountname"
}

@test "simple idmap mount [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "stat -c =%u=%g= /tmp/mount-1/foo.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=0=0="* ]]
}

@test "simple idmap mount [no userns]" {
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "stat -c =%u=%g= /tmp/mount-1/foo.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=100000=100000="* ]]
}

@test "write to an idmap mount [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "touch /tmp/mount-1/bar && stat -c =%u=%g= /tmp/mount-1/bar"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=0=0="* ]]
}

@test "write to an idmap mount [no userns]" {
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "touch /tmp/mount-1/bar && stat -c =%u=%g= /tmp/mount-1/bar"]'

	runc run test_debian
	# The write must fail because the user is unmapped.
	[ "$status" -ne 0 ]
	[[ "$output" == *"Value too large for defined data type"* ]] # ERANGE
}

@test "idmap mount with propagation flag [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "findmnt -o PROPAGATION /tmp/mount-1"]'
	# Add the shared option to the idmap mount
	update_config '.mounts |= map((select(.source == "source-1/") | .options += ["shared"]) // .)'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"shared"* ]]
}

@test "idmap mount with bind mount [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount
	setup_bind_mount

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/{,bind-}mount-1/foo.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-1/foo.txt:0=0="* ]]
	[[ "$output" == *"=/tmp/bind-mount-1/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
}

@test "idmap mount with bind mount [no userns]" {
	setup_idmap_basic_mount
	setup_bind_mount

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/{,bind-}mount-1/foo.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-1/foo.txt:100000=100000="* ]]
	[[ "$output" == *"=/tmp/bind-mount-1/foo.txt:0=0="* ]]
}

@test "two idmap mounts (same mapping) with two bind mounts [userns]" {
	setup_idmap_userns

	setup_idmap_basic_mount 1
	setup_bind_mount 1
	setup_bind_mount 2
	setup_idmap_basic_mount 2

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-[12]/foo.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-1/foo.txt:0=0="* ]]
	[[ "$output" == *"=/tmp/mount-2/foo.txt:1=1="* ]]
}

@test "same idmap mount (different mappings) [userns]" {
	setup_idmap_userns

	# Mount the same directory with different mappings. Make sure we also use
	# different mappings for uids and gids.
	setup_idmap_single_mount 100:100000:100 200:100000:100 multi1
	setup_idmap_single_mount 100:101000:100 200:102000:100 multi1 multi1-alt

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1{,-alt}/{foo,bar,baz}.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:0=11="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:1=22="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:2=33="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/foo.txt:1000=2011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/bar.txt:1001=2022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/baz.txt:1002=2033="* ]]
}

@test "same idmap mount (different mappings) [no userns]" {
	# Mount the same directory with different mappings. Make sure we also use
	# different mappings for uids and gids.
	setup_idmap_single_mount 100:100000:100 200:100000:100 multi1
	setup_idmap_single_mount 100:101000:100 200:102000:100 multi1 multi1-alt

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1{,-alt}/{foo,bar,baz}.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:100000=100011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:100001=100022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:100002=100033="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/foo.txt:101000=102011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/bar.txt:101001=102022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/baz.txt:101002=102033="* ]]
}

@test "multiple idmap mounts (different mappings) [userns]" {
	setup_idmap_userns

	# Make sure we use different mappings for uids and gids.
	setup_idmap_single_mount 100:101100:3 200:101900:50 multi1
	setup_idmap_single_mount 200:102200:3 200:102900:100 multi2
	setup_idmap_single_mount 5000000:103000:1000 6000000:103000:500 multi3

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi[123]/{foo,bar,baz}.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1100=1911="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:1101=1922="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:1102=1933="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/foo.txt:2200=2911="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/bar.txt:2201=2922="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/baz.txt:2202=2933="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/foo.txt:3528=3491="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/bar.txt:3133=3337="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/baz.txt:3999=3444="* ]]
}

@test "multiple idmap mounts (different mappings) [no userns]" {
	# Make sure we use different mappings for uids and gids.
	setup_idmap_single_mount 100:1100:3 200:1900:50 multi1
	setup_idmap_single_mount 200:2200:3 200:2900:100 multi2
	setup_idmap_single_mount 5000000:3000:1000 6000000:3000:500 multi3

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi[123]/{foo,bar,baz}.txt"]'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1100=1911="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:1101=1922="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:1102=1933="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/foo.txt:2200=2911="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/bar.txt:2201=2922="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/baz.txt:2202=2933="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/foo.txt:3528=3491="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/bar.txt:3133=3337="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/baz.txt:3999=3444="* ]]
}

@test "idmap mount (complicated mapping) [userns]" {
	setup_idmap_userns

	update_config '.mounts += [
			{
				"source": "source-multi1/",
				"destination": "/tmp/mount-multi1",
				"options": ["bind"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 1},
					{"containerID": 101, "hostID": 102000, "size": 1},
					{"containerID": 102, "hostID": 103000, "size": 1}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1/{foo,bar,baz}.txt"]'
	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:2000=2202="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:3000=3303="* ]]
}

@test "idmap mount (complicated mapping) [no userns]" {
	update_config '.mounts += [
			{
				"source": "source-multi1/",
				"destination": "/tmp/mount-multi1",
				"options": ["bind"],
				"uidMappings": [
					{"containerID": 100, "hostID": 1000, "size": 1},
					{"containerID": 101, "hostID": 2000, "size": 1},
					{"containerID": 102, "hostID": 3000, "size": 1}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 1100, "size": 10},
					{"containerID": 220, "hostID": 2200, "size": 10},
					{"containerID": 230, "hostID": 3300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1/{foo,bar,baz}.txt"]'
	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:2000=2202="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:3000=3303="* ]]
}
