## Make a release

There are two types of releases: automated and manual. The releases made
manually are meant to be done as part of a release process by a developer.

The automated releases are meant to be periodic snapshot releases. These
releases are meant to produce builds periodically.

The GITHUB_USER and GITHUB_TOKEN environment variables need to be set up.

### Automated releases

Automated releases are done using the scripts/REL.sh script. This script
will prepare and release binaries to GitHub.

The timestamp is appended to the release version.

An example of a such release tag is "1.0.0-beta-01-31-2017.21-11-13.UTC".


### Manual releases

These releases are releases made by hand. Some configuration options
need to be provided manually.

You'll find a few examples below:
```
# version/CURRENT_VERSION is 1.0.1
$ USE_RELEASE=1 OLD_VERSION=1.0 make release
# will release version 1.0.1 on GitHub
```

```
# version/CURRENT_VERSION is 1.0
$ USE_RELEASE=1 OLD_VERSION=none make release
# will release version 1.0 when no previous stable release exists
```

Please keep in mind that the release notes can be updated further manually.
