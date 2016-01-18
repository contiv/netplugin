define(['exports', 'module', 'react', './BootstrapMixin', 'classnames', './utils/ValidComponentChildren', './utils/createChainedFunction'], function (exports, module, _react, _BootstrapMixin, _classnames, _utilsValidComponentChildren, _utilsCreateChainedFunction) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _classNames = _interopRequire(_classnames);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var Navbar = _React.createClass({
    displayName: 'Navbar',

    mixins: [_BootstrapMixin2],

    propTypes: {
      fixedTop: _React.PropTypes.bool,
      fixedBottom: _React.PropTypes.bool,
      staticTop: _React.PropTypes.bool,
      inverse: _React.PropTypes.bool,
      fluid: _React.PropTypes.bool,
      role: _React.PropTypes.string,
      componentClass: _React.PropTypes.node.isRequired,
      brand: _React.PropTypes.node,
      toggleButton: _React.PropTypes.node,
      toggleNavKey: _React.PropTypes.oneOfType([_React.PropTypes.string, _React.PropTypes.number]),
      onToggle: _React.PropTypes.func,
      navExpanded: _React.PropTypes.bool,
      defaultNavExpanded: _React.PropTypes.bool
    },

    getDefaultProps: function getDefaultProps() {
      return {
        bsClass: 'navbar',
        bsStyle: 'default',
        role: 'navigation',
        componentClass: 'nav'
      };
    },

    getInitialState: function getInitialState() {
      return {
        navExpanded: this.props.defaultNavExpanded
      };
    },

    shouldComponentUpdate: function shouldComponentUpdate() {
      // Defer any updates to this component during the `onSelect` handler.
      return !this._isChanging;
    },

    handleToggle: function handleToggle() {
      if (this.props.onToggle) {
        this._isChanging = true;
        this.props.onToggle();
        this._isChanging = false;
      }

      this.setState({
        navExpanded: !this.state.navExpanded
      });
    },

    isNavExpanded: function isNavExpanded() {
      return this.props.navExpanded != null ? this.props.navExpanded : this.state.navExpanded;
    },

    render: function render() {
      var classes = this.getBsClassSet();
      var ComponentClass = this.props.componentClass;

      classes['navbar-fixed-top'] = this.props.fixedTop;
      classes['navbar-fixed-bottom'] = this.props.fixedBottom;
      classes['navbar-static-top'] = this.props.staticTop;
      classes['navbar-inverse'] = this.props.inverse;

      return _React.createElement(
        ComponentClass,
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        _React.createElement(
          'div',
          { className: this.props.fluid ? 'container-fluid' : 'container' },
          this.props.brand || this.props.toggleButton || this.props.toggleNavKey != null ? this.renderHeader() : null,
          _ValidComponentChildren.map(this.props.children, this.renderChild)
        )
      );
    },

    renderChild: function renderChild(child, index) {
      return _react.cloneElement(child, {
        navbar: true,
        collapsable: this.props.toggleNavKey != null && this.props.toggleNavKey === child.props.eventKey,
        expanded: this.props.toggleNavKey != null && this.props.toggleNavKey === child.props.eventKey && this.isNavExpanded(),
        key: child.key ? child.key : index
      });
    },

    renderHeader: function renderHeader() {
      var brand = undefined;

      if (this.props.brand) {
        if (_React.isValidElement(this.props.brand)) {
          brand = _react.cloneElement(this.props.brand, {
            className: _classNames(this.props.brand.props.className, 'navbar-brand')
          });
        } else {
          brand = _React.createElement(
            'span',
            { className: 'navbar-brand' },
            this.props.brand
          );
        }
      }

      return _React.createElement(
        'div',
        { className: 'navbar-header' },
        brand,
        this.props.toggleButton || this.props.toggleNavKey != null ? this.renderToggleButton() : null
      );
    },

    renderToggleButton: function renderToggleButton() {
      var children = undefined;

      if (_React.isValidElement(this.props.toggleButton)) {

        return _react.cloneElement(this.props.toggleButton, {
          className: _classNames(this.props.toggleButton.props.className, 'navbar-toggle'),
          onClick: _createChainedFunction(this.handleToggle, this.props.toggleButton.props.onClick)
        });
      }

      children = this.props.toggleButton != null ? this.props.toggleButton : [_React.createElement(
        'span',
        { className: 'sr-only', key: 0 },
        'Toggle navigation'
      ), _React.createElement('span', { className: 'icon-bar', key: 1 }), _React.createElement('span', { className: 'icon-bar', key: 2 }), _React.createElement('span', { className: 'icon-bar', key: 3 })];

      return _React.createElement(
        'button',
        { className: 'navbar-toggle', type: 'button', onClick: this.handleToggle },
        children
      );
    }
  });

  module.exports = Navbar;
});