define(['exports', 'module', 'react', 'classnames'], function (exports, module, _react, _classnames) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var Table = _React.createClass({
    displayName: 'Table',

    propTypes: {
      striped: _React.PropTypes.bool,
      bordered: _React.PropTypes.bool,
      condensed: _React.PropTypes.bool,
      hover: _React.PropTypes.bool,
      responsive: _React.PropTypes.bool
    },

    render: function render() {
      var classes = {
        table: true,
        'table-striped': this.props.striped,
        'table-bordered': this.props.bordered,
        'table-condensed': this.props.condensed,
        'table-hover': this.props.hover
      };
      var table = _React.createElement(
        'table',
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        this.props.children
      );

      return this.props.responsive ? _React.createElement(
        'div',
        { className: 'table-responsive' },
        table
      ) : table;
    }
  });

  module.exports = Table;
});