define(['exports', 'module', 'react', 'classnames', './utils/createChainedFunction', './utils/ValidComponentChildren'], function (exports, module, _react, _classnames, _utilsCreateChainedFunction, _utilsValidComponentChildren) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var DropdownMenu = _React.createClass({
    displayName: 'DropdownMenu',

    propTypes: {
      pullRight: _React.PropTypes.bool,
      onSelect: _React.PropTypes.func
    },

    render: function render() {
      var classes = {
        'dropdown-menu': true,
        'dropdown-menu-right': this.props.pullRight
      };

      return _React.createElement(
        'ul',
        _extends({}, this.props, {
          className: _classNames(this.props.className, classes),
          role: 'menu' }),
        _ValidComponentChildren.map(this.props.children, this.renderMenuItem)
      );
    },

    renderMenuItem: function renderMenuItem(child, index) {
      return _react.cloneElement(child, {
        // Capture onSelect events
        onSelect: _createChainedFunction(child.props.onSelect, this.props.onSelect),

        // Force special props to be transferred
        key: child.key ? child.key : index
      });
    }
  });

  module.exports = DropdownMenu;
});