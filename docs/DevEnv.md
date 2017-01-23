## Netplugin Developer's Guide
This document describes the development environment and typical steps to hack on netplugin.
While there are alternative ways, the steps listed here are used by a lot of developers while
making incremental changes and doing frequent compilation for a quicker development cycle.

Happy hacking!

### Pre-requisites
Vagarant 1.8.7
VirtualBox 5.1.x

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
Note: This is done outside the VMs. System tests would start the vagrant VMs if
they are not already running and run the tests on three VMs
```
$ make system-test
```

### 6. Commit changes to your fork; submit PR
Note: This is best done outside the VM using your git credentials setup on the host.

## Frequently asked questions

### How to use Godep?
Please see [here](./GoDep.md) for working with godep

### How to trigger a Jenkins CI run on my fork?

1. Go go http://contiv-ci.ngrok.io/view/Netplugin/
2. Click on `log in` button on top right corner and log in as user: `guest`, password: `guest`
3. Click on `Netplugin_OnDemand_Build` job
4. Click on `Build with Parameters` link on the left side
5. In `GIT_URL` field, enter the URL to your fork of netplugin
6. In `GIT_BRANCH` field, enter the branch name in your fork
7. Click `Build` button

This will checkout a tree from your fork and run full Jenkins CI on the branch.

### How to work across multiple repos like netplugin, contivmodel and ofnet

Easiest way to work across multiple repos is by doing a `godep restore` first and then make changes.

1. SSH to vagrant VM, goto `/opt/gopath/src/github.com/contiv/netplugin` directory.
2. issue `godep restore` command to restore all dependent packages into `$GOPATH`
3. go to individual repo directory, for example `github.com/contiv/ofnet` and checkout your fork of the repository.
4. Make changes to netplugin and ofnet and rebuild netplugin/netmaster using `cd /opt/gopath/src/github.com/contiv/netplugin; go install ./netplugin ./netmaster`
5. restart netplugin/netmaster using `make host-restart`
6. Test your code
7. Repeat steps 4-6 multiple times as you need

Once you are ready to commit:

1. Commit changes in ofnet/contivmodel first. Push the changes to your fork and submit pull request.
2. Once the pull request is merged, checkout the latest version of ofnet/contivmodel from contiv repo.
3. update netplugin's godeps using `godep update github.com/contiv/ofnet` or `godep update github.com/contiv/contivmodel`. Note that If you are godep updating ofnet, you need to update all ofnet packages in one command. E.g. `godep update github.com/contiv/ofnet github.com/contiv/ofnet/ofctrl github.com/contiv/ofnet/ovsdbDriver github.com/contiv/ofnet/pqueue github.com/contiv/ofnet/rpcHub`
4. Build netplugin and run unit/system tests as mentioned above
5. Commit netplugin changes and submit a pull request

### What is the development workflow with Git: Fork, Branching, Commits, and Pull Request

We follow [this](https://github.com/sevntu-checkstyle/sevntu.checkstyle/wiki/Development-workflow-with-Git:-Fork,-Branching,-Commits,-and-Pull-Request) Workflow for submitting pull requests

### How to squash all commits before submitting pull request

It is recommended that all commits in a pull requests is squashed before they can be merged. Please see [here](http://makandracards.com/makandra/527-squash-several-git-commits-into-a-single-commit) for instruction on squashing your commits

Github recently added a feature to [automatically squash all merges](https://github.com/blog/2141-squash-your-commits). We are evaluating using this. Meanwhile please continue to squash all commits in a pull request.
