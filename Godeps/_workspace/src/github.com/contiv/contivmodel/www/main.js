/** @jsx React.DOM */

// Little hack to make ReactBootstrap components visible globally
Object.keys(ReactBootstrap).forEach(function (name) {
    window[name] = ReactBootstrap[name];
});

// Navigation tab
var ControlledTabArea = require("./navTab")

// Render the main tabs
React.render(<ControlledTabArea />, document.getElementById('mainViewContainer'));
