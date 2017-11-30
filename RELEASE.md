## Make a release

There are two types of releases: automated and manual. The releases made
manually are meant to be done as part of a release process by a developer.

The automated releases are meant to be periodic snapshot releases. These
releases are meant to produce builds periodically.

The GITHUB_USER and GITHUB_TOKEN environment variables need to be set up.

### Automated releases

Automated releases are done using the scripts/REL.sh script. This script
will prepare and release binaries to GitHub.

The timestamp is appended to the release version if NIGHTLY_RELEASE=1 is
passed as an environment variable on build.

An example of a such release tag is "1.0.0-beta-01-31-2017.21-11-13.UTC".

Releases are marked as pre-releases by default. The pre-release needs to
be marked by hand as a release.

### Manual releases

These releases are releases made by hand. Some configuration options
need to be provided manually.

You'll find a few examples below:

	# version/CURRENT_VERSION is 1.0.1
	$ OLD_VERSION=1.0 make release
	# will release version 1.0.1 on GitHub

	# version/CURRENT_VERSION is 1.0
	$ OLD_VERSION=none make release
	# will release version 1.0 when no previous stable release exists

Please keep in mind that the release notes can be updated on GitHub manually.

BUILD_VERSION will not override the version for actual
releases (1.0, 1.0.1, 1.1.0 and so on).

The release process can be found below.

1. Check out the right branch and the right commit. This is necessary
when not releasing from the HEAD of master.

2. Tag the right commit and push it to GitHub. This is mandatory if the
release isn't made from the HEAD of master.
	```
	git tag 1.0.1 3aba546aea1235
	git push origin 1.0.1
	```

3. Update version/CURRENT_VERSION. This will be needed for the next steps.

4. Make sure GITHUB_USER and GITHUB_TOKEN variables are exported in your environment.

5. Make the release to GitHub.
	```
	OLD_VERSION=1.1.0-beta.1 make release
	```

	```
	# version/CURRENT_VERSION is 1.1.1
	OLD_VERSION=1.1.0 make release
	```

## Build and upload container image (manual only)

	1. cd scripts/netContain
	2. ./release_image.sh -v <version> \[-i <image_name>] \[-t <image_tag>]

Here <version> is a version that has been released using steps above and exists on github.
