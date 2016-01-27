define(['exports', 'module', 'react', 'classnames', './utils/ValidComponentChildren', './utils/createChainedFunction'], function (exports, module, _react, _classnames, _utilsValidComponentChildren, _utilsCreateChainedFunction) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var Pager = _React.createClass({
    displayName: 'Pager',

    propTypes: {
      onSelect: _React.PropTypes.func
    },

    render: function render() {
      return _React.createElement(
        'ul',
        _extends({}, this.props, {
          className: _classNames(this.props.className, 'pager') }),
        _ValidComponentChildren.map(this.props.children, this.renderPageItem)
      );
    },

    renderPageItem: function renderPageItem(child, index) {
      return _react.cloneElement(child, {
        onSelect: _createChainedFunction(child.props.onSelect, this.props.onSelect),
        key: child.key ? child.key : index
      });
    }
  });

  module.exports = Pager;
});