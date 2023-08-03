#!/bin/bash -x
ALL=$(cat /sys/fs/cgroup/cpuset/cpuset.cpus)

echo $$ >/sys/fs/cgroup/pids/cgroup.procs

for p in $(cat /sys/fs/cgroup/pids/crio.slice/*/cgroup.procs); do
  cpuset=$(cat /proc/$p/cpuset)
  echo $p >/sys/fs/cgroup/cpuset/cgroup.procs
  kill -TSTP $p
  taskset -a -pc $ALL $p
  kill -CONT $p
  echo $p >/sys/fs/cgroup/cpuset${cpuset}/cgroup.procs
  cat /proc/$p/cpuset
done

exit 0
