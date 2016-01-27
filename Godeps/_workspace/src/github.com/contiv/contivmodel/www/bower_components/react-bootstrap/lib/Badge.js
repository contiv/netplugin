define(['exports', 'module', 'react', './utils/ValidComponentChildren', 'classnames'], function (exports, module, _react, _utilsValidComponentChildren, _classnames) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var _classNames = _interopRequire(_classnames);

  var Badge = _React.createClass({
    displayName: 'Badge',

    propTypes: {
      pullRight: _React.PropTypes.bool
    },

    hasContent: function hasContent() {
      return _ValidComponentChildren.hasValidComponent(this.props.children) || _React.Children.count(this.props.children) > 1 || typeof this.props.children === 'string' || typeof this.props.children === 'number';
    },

    render: function render() {
      var classes = {
        'pull-right': this.props.pullRight,
        badge: this.hasContent()
      };
      return _React.createElement(
        'span',
        _extends({}, this.props, {
          className: _classNames(this.props.className, classes) }),
        this.props.children
      );
    }
  });

  module.exports = Badge;
});