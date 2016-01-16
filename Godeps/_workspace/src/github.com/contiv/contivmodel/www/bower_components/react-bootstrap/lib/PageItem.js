define(['exports', 'module', 'react', 'classnames'], function (exports, module, _react, _classnames) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var PageItem = _React.createClass({
    displayName: 'PageItem',

    propTypes: {
      href: _React.PropTypes.string,
      target: _React.PropTypes.string,
      title: _React.PropTypes.string,
      disabled: _React.PropTypes.bool,
      previous: _React.PropTypes.bool,
      next: _React.PropTypes.bool,
      onSelect: _React.PropTypes.func,
      eventKey: _React.PropTypes.any
    },

    getDefaultProps: function getDefaultProps() {
      return {
        href: '#'
      };
    },

    render: function render() {
      var classes = {
        disabled: this.props.disabled,
        previous: this.props.previous,
        next: this.props.next
      };

      return _React.createElement(
        'li',
        _extends({}, this.props, {
          className: _classNames(this.props.className, classes) }),
        _React.createElement(
          'a',
          {
            href: this.props.href,
            title: this.props.title,
            target: this.props.target,
            onClick: this.handleSelect,
            ref: 'anchor' },
          this.props.children
        )
      );
    },

    handleSelect: function handleSelect(e) {
      if (this.props.onSelect) {
        e.preventDefault();

        if (!this.props.disabled) {
          this.props.onSelect(this.props.eventKey, this.props.href, this.props.target);
        }
      }
    }
  });

  module.exports = PageItem;
});