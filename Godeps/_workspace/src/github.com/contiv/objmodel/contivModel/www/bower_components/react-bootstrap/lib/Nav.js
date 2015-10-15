define(['exports', 'module', 'react', './BootstrapMixin', './CollapsibleMixin', 'classnames', './utils/domUtils', './utils/ValidComponentChildren', './utils/createChainedFunction'], function (exports, module, _react, _BootstrapMixin, _CollapsibleMixin, _classnames, _utilsDomUtils, _utilsValidComponentChildren, _utilsCreateChainedFunction) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _CollapsibleMixin2 = _interopRequire(_CollapsibleMixin);

  var _classNames = _interopRequire(_classnames);

  var _domUtils = _interopRequire(_utilsDomUtils);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var Nav = _React.createClass({
    displayName: 'Nav',

    mixins: [_BootstrapMixin2, _CollapsibleMixin2],

    propTypes: {
      activeHref: _React.PropTypes.string,
      activeKey: _React.PropTypes.any,
      bsStyle: _React.PropTypes.oneOf(['tabs', 'pills']),
      stacked: _React.PropTypes.bool,
      justified: _React.PropTypes.bool,
      onSelect: _React.PropTypes.func,
      collapsable: _React.PropTypes.bool,
      expanded: _React.PropTypes.bool,
      navbar: _React.PropTypes.bool,
      eventKey: _React.PropTypes.any,
      pullRight: _React.PropTypes.bool,
      right: _React.PropTypes.bool
    },

    getDefaultProps: function getDefaultProps() {
      return {
        bsClass: 'nav'
      };
    },

    getCollapsibleDOMNode: function getCollapsibleDOMNode() {
      return _React.findDOMNode(this);
    },

    getCollapsibleDimensionValue: function getCollapsibleDimensionValue() {
      var node = _React.findDOMNode(this.refs.ul),
          height = node.offsetHeight,
          computedStyles = _domUtils.getComputedStyles(node);

      return height + parseInt(computedStyles.marginTop, 10) + parseInt(computedStyles.marginBottom, 10);
    },

    render: function render() {
      var classes = this.props.collapsable ? this.getCollapsibleClassSet() : {};

      classes['navbar-collapse'] = this.props.collapsable;

      if (this.props.navbar && !this.props.collapsable) {
        return this.renderUl();
      }

      return _React.createElement(
        'nav',
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        this.renderUl()
      );
    },

    renderUl: function renderUl() {
      var classes = this.getBsClassSet();

      classes['nav-stacked'] = this.props.stacked;
      classes['nav-justified'] = this.props.justified;
      classes['navbar-nav'] = this.props.navbar;
      classes['pull-right'] = this.props.pullRight;
      classes['navbar-right'] = this.props.right;

      return _React.createElement(
        'ul',
        _extends({}, this.props, { className: _classNames(this.props.className, classes), ref: 'ul' }),
        _ValidComponentChildren.map(this.props.children, this.renderNavItem)
      );
    },

    getChildActiveProp: function getChildActiveProp(child) {
      if (child.props.active) {
        return true;
      }
      if (this.props.activeKey != null) {
        if (child.props.eventKey === this.props.activeKey) {
          return true;
        }
      }
      if (this.props.activeHref != null) {
        if (child.props.href === this.props.activeHref) {
          return true;
        }
      }

      return child.props.active;
    },

    renderNavItem: function renderNavItem(child, index) {
      return _react.cloneElement(child, {
        active: this.getChildActiveProp(child),
        activeKey: this.props.activeKey,
        activeHref: this.props.activeHref,
        onSelect: _createChainedFunction(child.props.onSelect, this.props.onSelect),
        key: child.key ? child.key : index,
        navItem: true
      });
    }
  });

  module.exports = Nav;
});