define(['exports', 'module', 'react', 'classnames', './styleMaps'], function (exports, module, _react, _classnames, _styleMaps) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _styleMaps2 = _interopRequire(_styleMaps);

  var Col = _React.createClass({
    displayName: 'Col',

    propTypes: {
      xs: _React.PropTypes.number,
      sm: _React.PropTypes.number,
      md: _React.PropTypes.number,
      lg: _React.PropTypes.number,
      xsOffset: _React.PropTypes.number,
      smOffset: _React.PropTypes.number,
      mdOffset: _React.PropTypes.number,
      lgOffset: _React.PropTypes.number,
      xsPush: _React.PropTypes.number,
      smPush: _React.PropTypes.number,
      mdPush: _React.PropTypes.number,
      lgPush: _React.PropTypes.number,
      xsPull: _React.PropTypes.number,
      smPull: _React.PropTypes.number,
      mdPull: _React.PropTypes.number,
      lgPull: _React.PropTypes.number,
      componentClass: _React.PropTypes.node.isRequired
    },

    getDefaultProps: function getDefaultProps() {
      return {
        componentClass: 'div'
      };
    },

    render: function render() {
      var ComponentClass = this.props.componentClass;
      var classes = {};

      Object.keys(_styleMaps2.SIZES).forEach(function (key) {
        var size = _styleMaps2.SIZES[key];
        var prop = size;
        var classPart = size + '-';

        if (this.props[prop]) {
          classes['col-' + classPart + this.props[prop]] = true;
        }

        prop = size + 'Offset';
        classPart = size + '-offset-';
        if (this.props[prop] >= 0) {
          classes['col-' + classPart + this.props[prop]] = true;
        }

        prop = size + 'Push';
        classPart = size + '-push-';
        if (this.props[prop] >= 0) {
          classes['col-' + classPart + this.props[prop]] = true;
        }

        prop = size + 'Pull';
        classPart = size + '-pull-';
        if (this.props[prop] >= 0) {
          classes['col-' + classPart + this.props[prop]] = true;
        }
      }, this);

      return _React.createElement(
        ComponentClass,
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        this.props.children
      );
    }
  });

  module.exports = Col;
});