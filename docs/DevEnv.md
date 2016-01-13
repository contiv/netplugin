## Netplugin Developer's Guide
This document describes the development environment and typical steps to hack on netplugin.
While there are alternative ways, the steps listed here are used by a lot of developers while
making incremental changes and doing frequent compilation for a quicker development cycle.

Happy hacking!

### 1. Check-out a tree. `Estimated time: 2 minutes`
Notes: Make sure GOPATH is set. This is a one time activity.
```
$ cd $GOPATH
$ mkdir -p src/github.com/contiv
$ cd src/github.com/contiv
# it is recommended that you fork the repo if you want to make contributions
$ git clone https://github.com/<your-github-id>/netplugin.git
```

### 2. Create development VMs. `Estimated time: 3-4 minutes`
Note: This is a one time activity
```
$ cd $GOPATH/src/github.com/contiv/netplugin
$ make start
```

### 3. Make code changes
Notes: This must be done inside a VM. Note that netplugin repo is mounted from the host,
therefore any changes changes are saved outside VM and are not lost if VM crashes or dies or any reason
```
# ssh into one of the VMs
$ make ssh
# this command will change the directory to $GOPATH/src/github.com/contiv/netplugin
# make code changes here and add unit/system tests for your changes
. . .
# this command might not be needed if the directory wasn't changed
$ cd $GOPATH/src/github.com/contiv/netplugin
# compile the recently made changes. `Estimated time: 1m 20s`
$ make host-build
```

### 4. Run Unit tests. `Estimated time: 2 minutes`
Note: All this is done from inside the VM. Technically the VM is 
the development environment including unit testing
```
$ cd $GOPATH/src/github.com/contiv/netplugin
# make sure to clean up any remnants from prior runs; note that cleanup may 
# throw some harmless errors if things are already clean (so feel free to ignore them)
$ make host-cleanup
$ make host-unit-test

# iterate back to step 3 if tests fails or you need to make more code changes
```

### 5. Run system tests `Estimated Time: 90 mins`
Note: Again, this is also done from inside the VM. System tests would run across multiple 
hosts (vm1 and vm2). Therefore it is important to keep both VMs (spun up from make-demo) running 
otherwise it may not run multi-host networking tests well enough. The time taken to run
system tests will be higher first time because the tests will download some containers for testing
```
$ make host-sanity-test
```

### 6. Commit changes to your fork; submit PR
Note: This is best done outside the VM using your git credentials setup on the host.
