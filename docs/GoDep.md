# Dependency management in netplugin
This document explains the steps for vendoring update packages in netplugin

## Table of Contents
- [Audience](#audience)
- [Restoring dependencies](#restoring-dependencies)
- [Adding new dependencies](#adding-new-dependencies)
- [Updating existing dependencies](#updating-existing-dependencies)

## Audience
This document is targeted towards the developers looking into or working on
adding/updating vendored packages in netplugin/Godeps.

## Restoring dependencies
- netplugin/Godeps/Godeps.json has information about the vendored packages.
- `godep restore` can be used to copy these packages to $GOPATH/src
- It restores the packages in $GOPATH to a state expected by netplugin
- note: changes(if any) in $GOPATH/src will be over-written
```
cd netplugin
godep restore
```

## Adding new dependencies

- Add the new package to your $GOPATH/src
```
go get pkg_url 
```

- From within the netplugin directory run `godep save`, which will copy relevant 
packages from `$GOPATH/src` to `netplugin/Godeps/_workspace`. It will also update 
`netplugin/Godeps/Godeps.json`

```
cd netplugin
godep save ./...
```

- Verify the changes by running `git status` in netplugin directory. 
Verify that package is added to `Godeps/_workspace/src` and that 
`Godeps/Godeps.json` also reflects it

```
git status
```

## Updating existing dependencies
- Go to the `$GOPATH/directory` which hosts the package and update the package
```
cd $GOPATH/src/github.com/samalba/dockerclient

# update the package to the master
git checkout master
```

- From within the netplugin directory execute a `godep update pkg` to update the
Godeps
```
godep udpate github.com/samalba/dockerclient
```

In case above does not work try update as follows:
```
godep udpate github.com/samalba/dockerclient/...
```

- Verify the changes by running `git status` in netplugin directory. 
Verify that package is added to `Godeps/_workspace/src` and that 
`Godeps/Godeps.json` also reflects the version change 

```
git status
```


