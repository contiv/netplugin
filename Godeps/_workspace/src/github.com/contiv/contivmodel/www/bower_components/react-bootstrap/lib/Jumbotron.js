define(['exports', 'module', 'react', 'classnames'], function (exports, module, _react, _classnames) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var Jumbotron = _React.createClass({
    displayName: 'Jumbotron',

    render: function render() {
      return _React.createElement(
        'div',
        _extends({}, this.props, { className: _classNames(this.props.className, 'jumbotron') }),
        this.props.children
      );
    }
  });

  module.exports = Jumbotron;
});