## Contributing to Netplugin

There are many ways to contribute - report issues, suggest doc changes, 
submit bug fixes, propose design changes, discuss use cases, propose 
interop with other ecosystem software, or become a maintainer.

All contributions, big or small are welcome!

### Reporting Issues
Usage or non code related issues can be reported by clicking `New Issue` on
[netplugin's issues on github](https://github.com/contiv/netplugin/issues).
A feature request can also be submitted as an issue.

However if there is an issue with running the netplugin binary, providing following 
information would shorten the debug time:
- Version of container runtime (e.g. docker), state driver (e.g. etcd), 
netplugin version, driver (e.g. ovs), Operating System version, and other
applicable information.
- Steps to reproduce, if any
- Backtrace, if applicable

### Suggesting a doc change, or submitting a bug fix
Just go ahead and submit a PR, one of the maintainers would review the diffs
and provide feedback. After discussions, as the changes look good, they will
be merged into the master. Please make sure you run the unit and system tests
before submitting the PR.

### Proposing a design change
If you would like to refactor the code, propose a new design component, or
introduce a significant change, please discuss it as an issue titled as
`Proposal: ...` in order for people to provide feedback early enough. 
Making significant code changes before design discussion may waste
time, therefore it is discouraged.

After design discussions:
- Fork a private repo
- Make the changes in the private repo
- Add unit and system test cases for your code
- Make sure existing tests and newly added tests pass
- Merge your changes with the latest in the master branch
- Re-run the unit and system tests
- Submit a PR with the code changes
- This would involve discussions and few adjustments may need to be made.
It is encouraged to engage into discussions during the coding phase as well.
- After `LGTM` from maintainers, re-run unit and system tests
- One of the maintainers would merge your changes into the appropriate release candidate


### Discussing use cases and requesting new features
Bring up your use cases by submitting an issue. Describe the use case that
is not handled in the latest version. Issues that seek new features should
be titled as `Feature Request: ...`. It is encouraged to include diagrams 
(pics of hand drawn diagrams is just fine), or other details that best
describes a use case.

### Becoming a maintainer
Of course, play with the code, know is inside out - and you will know if you
are ripe to become a maintainer. If you think you are ready, drop an eamil
to any of the maintainers.

### Legal Stuff: Sign your work
You must sign-off your work by adding your signature at the end of 
patch description. Your signature certifies that you wrote the patch or 
otherwise have the right to pass it on as an open-source patch. 
By signing off your work you ascertain following (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Use your real name (sorry, no pseudonyms or anonymous contributions.)

If you set your `user.name` and `user.email` git configs, you can sign your
commit automatically with `git commit -s`.

