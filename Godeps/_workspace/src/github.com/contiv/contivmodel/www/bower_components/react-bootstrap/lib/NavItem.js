define(['exports', 'module', 'react', 'classnames', './BootstrapMixin'], function (exports, module, _react, _classnames, _BootstrapMixin) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _objectWithoutProperties = function (obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var NavItem = _React.createClass({
    displayName: 'NavItem',

    mixins: [_BootstrapMixin2],

    propTypes: {
      onSelect: _React.PropTypes.func,
      active: _React.PropTypes.bool,
      disabled: _React.PropTypes.bool,
      href: _React.PropTypes.string,
      title: _React.PropTypes.node,
      eventKey: _React.PropTypes.any,
      target: _React.PropTypes.string
    },

    getDefaultProps: function getDefaultProps() {
      return {
        href: '#'
      };
    },

    render: function render() {
      var _props = this.props;
      var disabled = _props.disabled;
      var active = _props.active;
      var href = _props.href;
      var title = _props.title;
      var target = _props.target;
      var children = _props.children;

      var props = _objectWithoutProperties(_props, ['disabled', 'active', 'href', 'title', 'target', 'children']);

      var classes = {
        active: active,
        disabled: disabled
      };
      var linkProps = {
        href: href,
        title: title,
        target: target,
        onClick: this.handleClick,
        ref: 'anchor'
      };

      if (href === '#') {
        linkProps.role = 'button';
      }

      return _React.createElement(
        'li',
        _extends({}, props, { className: _classNames(props.className, classes) }),
        _React.createElement(
          'a',
          linkProps,
          children
        )
      );
    },

    handleClick: function handleClick(e) {
      if (this.props.onSelect) {
        e.preventDefault();

        if (!this.props.disabled) {
          this.props.onSelect(this.props.eventKey, this.props.href, this.props.target);
        }
      }
    }
  });

  module.exports = NavItem;
});