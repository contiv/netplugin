define(['exports', 'module', 'react', './BootstrapMixin', 'classnames'], function (exports, module, _react, _BootstrapMixin, _classnames) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _classNames = _interopRequire(_classnames);

  var ListGroupItem = _React.createClass({
    displayName: 'ListGroupItem',

    mixins: [_BootstrapMixin2],

    propTypes: {
      bsStyle: _React.PropTypes.oneOf(['danger', 'info', 'success', 'warning']),
      className: _React.PropTypes.string,
      active: _React.PropTypes.any,
      disabled: _React.PropTypes.any,
      header: _React.PropTypes.node,
      listItem: _React.PropTypes.bool,
      onClick: _React.PropTypes.func,
      eventKey: _React.PropTypes.any,
      href: _React.PropTypes.string,
      target: _React.PropTypes.string
    },

    getDefaultProps: function getDefaultProps() {
      return {
        bsClass: 'list-group-item'
      };
    },

    render: function render() {
      var classes = this.getBsClassSet();

      classes.active = this.props.active;
      classes.disabled = this.props.disabled;

      if (this.props.href || this.props.onClick) {
        return this.renderAnchor(classes);
      } else if (this.props.listItem) {
        return this.renderLi(classes);
      } else {
        return this.renderSpan(classes);
      }
    },

    renderLi: function renderLi(classes) {
      return _React.createElement(
        'li',
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        this.props.header ? this.renderStructuredContent() : this.props.children
      );
    },

    renderAnchor: function renderAnchor(classes) {
      return _React.createElement(
        'a',
        _extends({}, this.props, {
          className: _classNames(this.props.className, classes)
        }),
        this.props.header ? this.renderStructuredContent() : this.props.children
      );
    },

    renderSpan: function renderSpan(classes) {
      return _React.createElement(
        'span',
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        this.props.header ? this.renderStructuredContent() : this.props.children
      );
    },

    renderStructuredContent: function renderStructuredContent() {
      var header = undefined;
      if (_React.isValidElement(this.props.header)) {
        header = _react.cloneElement(this.props.header, {
          key: 'header',
          className: _classNames(this.props.header.props.className, 'list-group-item-heading')
        });
      } else {
        header = _React.createElement(
          'h4',
          { key: 'header', className: 'list-group-item-heading' },
          this.props.header
        );
      }

      var content = _React.createElement(
        'p',
        { key: 'content', className: 'list-group-item-text' },
        this.props.children
      );

      return [header, content];
    }
  });

  module.exports = ListGroupItem;
});