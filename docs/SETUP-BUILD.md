# Netplugin Setup / Build Instructions
This document describes how to setup a build environment for [Netplugin](https://github.com/contiv/netplugin)). It starts with a base Ubuntu installation and shows which additional packages need to be installed to get to a functional working environment using a fork of the Netplugin from GitHub, including the required steps to get to and  pass the build, test and demo steps.

# Base Virtual Machine
Start with installing a base Ubuntu image. I've used the `ubuntu-14.04.2-server-amd64.iso` 64-bit server flavor of Ubuntu. The parameters for my VM are

- 8 GB memory
- 64 GB disk space
- 2 vCPUs

To enable the use of [Vagrant](https://www.vagrantup.com/) and [VirtualBox](https://www.virtualbox.org/) within the VM on ESXi, I had to add the following line to the VMs configuration (via SSH / CLI on the ESXi server, editing the `.vmx` file of the VM):

`vhv.enable = "TRUE"`

Then reload the changed VM configuration into ESXi by typing

`vim-cmd vmsvc/getallvms`, make a note of the Netplugin VM ID

`vim-cmd vmsvc/reload <VMID>`

If this is on VMware Workstation or Fusion then the `.vmx` file modification works as well. However, on Fusion this is actually a GUI option called 'Enable hypervisor applications in this virtual machine'. It can be found in the VM settings, Processors and Memory, Advanced options.

Install the Ubuntu server image, pretty much with defaults, the only option I've chosen in the software selection was 'OpenSSH server'. After installation has completed and the box reloaded, install updates and a few additional packages to compile code and the Git SCM:

	sudo apt-get update
	sudo apt-get dist-upgrade -y
	sudo shutdown -r now 
	
When the machine is back up again:

	sudo apt-get install build-essential git -y
	sudo apt-get autoremove
	sudo apt-get autoclean

# Additional Software
On top of the base install we need additional software before we can install and compile Netplugin. The following packages are required except for Docker on the *build* machine which is optional, as far as I can tell. All of them need a newer version than included with Ubuntu 14.04:

- Vagrant 

		$ vagrant --version
		Vagrant 1.7.3

- VirtualBox, latest 4.3 should do, Vagrant does not support 5.0 (yet)

		$ VBoxManage --version
		4.3.30r101610
				
- Docker, installed latest 1.7

		$ docker --version
		Docker version 1.7.1, build 786b29d

- Go, needs to be at least at version 1.4

		$ go version
		go version go1.4.2 linux/amd64

### Vagrant
Download the current Vagrant package from [here](http://www.vagrantup.com/downloads), it's the 64 bit package for Debian / Ubuntu. Install via

	$ sudo dpkg --install vagrant_1.7.3_x86_64.deb

### VirtualBox
Download the latest 4.3 version of VirtualBox from [here](https://www.virtualbox.org/wiki/Download_Old_Builds_4_3), choose the 64bit version for Ubuntu 13.04 (which also works with subsequent Ubuntu versions). Install via

	$ sudo dpkg --install virtualbox-4.3_4.3.30-101610~Ubuntu~raring_amd64.deb
	
This will result in an error because of missing dependencies. Then tell the system to install (a ton) of missing packages, mostly b/c VirtualBox is a GUI application and therefore needs all the X11 etc. libraries and files. Despite the fact that Vagrant is using it in a headless way. The driver setup is for good measure.

	$ sudo apt-get install -fy
	$ sudo service vboxdrv setup
	
The result should look like 

	$ sudo service vboxdrv setup
	Stopping VirtualBox kernel modules ...done.
	Recompiling VirtualBox kernel modules ...done.
	Starting VirtualBox kernel modules ...done.
	$ 

Finally, we need to add our user id to the `vboxusers` group:

	sudo usermod -a -G vboxusers netplugin
	
And logout, login to apply that change. We should now be all set from a Virtualbox point of view (we expect to see our user as a member of `vboxusers`):

	$ id
	uid=1000(netplugin) gid=1000(netplugin) groups=1000(netplugin),4(adm),24(cdrom),27(sudo),	30(dip),46(plugdev),110(lpadmin),111(sambashare),112(vboxusers)

If we ever need the GUI of Virtualbox then it can be fairly easily forwarded over SSH using the -X switch to a locally running X11 server (on a Mac, it's called [Xquartz](https://support.apple.com/en-us/HT201341)).

Verify the Vagrant and Virtualbox setup by going through the following commands:

	$ mkdir TEST; cd TEST
	$ vagrant init ubuntu/trusty64
	$ vagrant up
	
If all goes well (download and machine starting etc.) we will see something like 

	==> default: Machine booted and ready!

showing up. If it does, we can declare success for the Vagrant and VirtualBox combo. If so, bring down and destroy that temporary box again using

	$ vagrant halt 
	==> default: Attempting graceful shutdown of VM...
	$ vagrant destroy
    	default: Are you sure you want to destroy the 'default' VM? [y/N] y
	==> default: Destroying VM and associated drives...
	$ cd ..
	$ rm -rf TEST/
	
### Docker
This step is optional. We install the latest Docker version direct from their site. At the end of the installation we need to add our user to the `docker` group.

	$ wget -qO- https://get.docker.com/ | sh
	$ sudo usermod -aG docker netplugin
	
As the Docker install script says, *Remember that you will have to log out and back in for this to take effect!*. After doing so let's check what we have:

	$ docker version
	Client version: 1.7.1
	Client API version: 1.19
	Go version (client): go1.4.2
	Git commit (client): 786b29d
	OS/Arch (client): linux/amd64
	Server version: 1.7.1
	Server API version: 1.19
	Go version (server): go1.4.2
	Git commit (server): 786b29d
	OS/Arch (server): linux/amd64
	$ 

And if it actually works:

	$ docker run -it ubuntu:trusty bash
	Unable to find image 'ubuntu:trusty' locally
	trusty: Pulling from ubuntu
	83e4dde6b9cf: Pull complete 
	b670fb0c7ecd: Pull complete 
	29460ac93442: Pull complete 
	d2a0ecffe6fa: Already exists 
	ubuntu:trusty: The image you are pulling has been verified. Important: image verification is a tech preview feature and should not be relied on to provide security.
	Digest: sha256:6cd0b1a4370ce7fbd27fddb70a5673a50c4719bd66a110b6cc8dd22a30ccb374
	Status: Downloaded newer image for ubuntu:trusty
	root@9164788dda01:/# 

So, we are now in a container, it did work! Ctrl-P, Ctrl-Q to bail out back to the host machine. Then clean up our stuff again (change the container id so that it matches your system):

	$ docker ps
	CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
	9164788dda01        ubuntu:trusty       "bash"              2 minutes ago       Up 2 minutes                            cranky_leakey       
	$ docker stop 9164788dda01
	9164788dda01
	$ docker rm 9164788dda01
	9164788dda01
	$ 

### Go
Go can be downloaded [here](https://golang.org/dl/). Choose `go1.4.2.linux-amd64.tar.gz` for Ubuntu. Then go to [this page](https://golang.org/doc/install) and follow the instructions there. I've installed in `/usr/local`, as recommended. Create a local Go workspace in the home directory: 

	$ mkdir -p ~/go

Add the following lines to `~/.bashrc`:

	export GOPATH=$HOME/go
	export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

Need to logout / login to make the changes effective (or `source .bashrc`)

# Project Setup
Created the Netplugin source tree using:

	$ mkdir -p $GOPATH/src/github.com
	
Lot of good info regarding Go directory layout [here](http://golang.org/doc/code.html). Then installed the Netplugin source tree. Note that I forked the project into my own account (johndoe) on Github.

	$ go get github.com/johndoe/netplugin
	package github.com/johndoe/netplugin
        imports github.com/johndoe/netplugin
        imports github.com/johndoe/netplugin: no buildable Go source files in /home/netplugin/go/src/github.com/johndoe/netplugin
	$ 

### Syncing the Netplugin Fork
In my case, my fork of Netplugin was out of date... Looking [here](https://help.github.com/articles/syncing-a-fork/) and [here](http://www.jonathanmedd.net/2013/06/git-remote-add-upstream-fatal-remote-upstream-already-exists.html) definitely helped to get to the latest master. These were my steps (and I was several commits behind which is not showing below b/c it already fetched and merged those in the meantime):

	$ cd $GOPATH/src/github.com/johndoe/netplugin
	$ git remote add upstream https://github.com/contiv/netplugin
	$ git fetch upstream
	From https://github.com/contiv/netplugin
	 * [new branch]      mapuri/safeStateAPIs -> upstream/mapuri/safeStateAPIs
	 * [new branch]      mapuri/stateTranslation -> upstream/mapuri/stateTranslation
	 * [new branch]      master     -> upstream/master
	$ git checkout master
	Already on 'master'
	Your branch is up-to-date with 'origin/master'.
	$ git merge upstream/master
	Already up-to-date.
	$ 
	
### Git Setup
To setup the local fork and track `contiv/netlugin` as the master the following commands are required:

	$ go get -d github.com/contiv/netplugin
	$ cd $GOPATH/src/github.com/contiv/netplugin
	$ git remote add johndoe git@github.com:johndoe/netplugin
	
This allows to work with remote `johndoe` when pushing to your fork.

### Alternative Setup with Sym Links
An alternative way to build the environment is to add a symbolic link in the Go directory tree so that the 'contiv' (original) path points to your fork... This helped in my case:

	$ cd $GOPATH/go/src/github.com/
	$ ln -s johndoe contiv 
	$ ls -l
	total 4
	lrwxrwxrwx 1 netplugin netplugin    8 Jul 16 16:35 contiv -> johndoe
	drwxrwxr-x 3 netplugin netplugin 4096 Jul 16 16:38 johndoe
	$ 
	

# Build
We should be ready now to build the project.

	$ cd $GOPATH/src/github.com/johndoe/netplugin
	$ make build
	$ make unit-test
	
When everything goes well, we will see a lot of output ending in:

	[...]
	--- PASS: TestNewNetworkDriverInvalidDriverName (0.01s)
	PASS
	ok      github.com/contiv/netplugin/utils       0.124s
	Sandbox: Tests succeeded!
	Connection to 127.0.0.1 closed.
	==> netplugin-node1: Forcing shutdown of VM...
	==> netplugin-node1: Destroying VM and associated drives...
	==> netplugin-node1: Running cleanup tasks for 'shell' provisioner...
	==> netplugin-node1: Running cleanup tasks for 'shell' provisioner...
	Host: Tests succeeded!
	$ 

And we then can go on with the system-test and the other demo and test targets as outlined in the top level README.
