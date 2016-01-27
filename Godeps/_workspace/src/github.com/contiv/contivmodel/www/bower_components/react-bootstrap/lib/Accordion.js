define(['exports', 'module', 'react', './PanelGroup'], function (exports, module, _react, _PanelGroup) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _PanelGroup2 = _interopRequire(_PanelGroup);

  var Accordion = _React.createClass({
    displayName: 'Accordion',

    render: function render() {
      return _React.createElement(
        _PanelGroup2,
        _extends({}, this.props, { accordion: true }),
        this.props.children
      );
    }
  });

  module.exports = Accordion;
});