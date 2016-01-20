## Contiv Web UI

This directory has the web UI for the contiv object model

### Building the web UI
Requirements:
	Node.js v0.12 and npm v2.5.1
	Bower 1.3.9
	Webpack 1.8.11

Currently build is tested on Mac OS only.

Just Run
```
./make.sh
```
This will install all required Npm packages, Bower packages and create a Javascript bundle at ./dist/bundle.js.

For incremental builds, run
```
webpack
```

### Architecture
This is a classic single page web application based on React framework, Jquery and Bootstrap front end framework. It uses React-Bootstrap framework(http://react-bootstrap.github.io/) to render web views.
