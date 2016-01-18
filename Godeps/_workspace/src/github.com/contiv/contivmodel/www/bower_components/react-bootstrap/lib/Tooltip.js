define(['exports', 'module', 'react', 'classnames', './BootstrapMixin'], function (exports, module, _react, _classnames, _BootstrapMixin) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _defineProperty = function (obj, key, value) { return Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var Tooltip = _React.createClass({
    displayName: 'Tooltip',

    mixins: [_BootstrapMixin2],

    propTypes: {
      placement: _React.PropTypes.oneOf(['top', 'right', 'bottom', 'left']),
      positionLeft: _React.PropTypes.number,
      positionTop: _React.PropTypes.number,
      arrowOffsetLeft: _React.PropTypes.number,
      arrowOffsetTop: _React.PropTypes.number
    },

    getDefaultProps: function getDefaultProps() {
      return {
        placement: 'right'
      };
    },

    render: function render() {
      var _classes;

      var classes = (_classes = {
        tooltip: true }, _defineProperty(_classes, this.props.placement, true), _defineProperty(_classes, 'in', this.props.positionLeft != null || this.props.positionTop != null), _classes);

      var style = {
        left: this.props.positionLeft,
        top: this.props.positionTop
      };

      var arrowStyle = {
        left: this.props.arrowOffsetLeft,
        top: this.props.arrowOffsetTop
      };

      return _React.createElement(
        'div',
        _extends({}, this.props, { className: _classNames(this.props.className, classes), style: style }),
        _React.createElement('div', { className: 'tooltip-arrow', style: arrowStyle }),
        _React.createElement(
          'div',
          { className: 'tooltip-inner' },
          this.props.children
        )
      );
    }
  });

  module.exports = Tooltip;
});