# Dependency management in netplugin
This document explains the steps for vendoring update packages in netplugin.

## Obtaining Godep

```bash
$ go get -d -u github.com/tools/godep
```

## Make Tasks

Please review the make tasks in the
[Makefile](https://github.com/contiv/netplugin/blob/master/Makefile) to see how
these tasks are implemented, they will assist with debugging godep issues in
your `$GOPATH`, etc.

* `make godep-save` saves the godeps from your `$GOPATH` to the repository,
  overwiting all Godeps as necessary.
* `make godep-restore` restores the godeps to your `$GOPATH`, populating or
  changing the revisions as necessary of the repositories within.

## Workflow

1. `make godep-restore` to update your repositories with the latest godeps we use.
1. Enter your `$GOPATH` and make the revisions you need or check out the versions you want.
  * all changes must be committed, godep does not work with uncommitted data in any repository.
1. `make godep-save` to commit your changes from `$GOPATH` to the repository.
1. `git add vendor Godeps` to add your changes and `git commit -s` to commit them to the repository.
  * **it is strongly advised you do this in a commit with just these changes to avoid problems with rebasing later**
