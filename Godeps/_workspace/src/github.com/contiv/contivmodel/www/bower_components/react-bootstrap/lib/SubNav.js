define(['exports', 'module', 'react', 'classnames', './utils/ValidComponentChildren', './utils/createChainedFunction', './BootstrapMixin'], function (exports, module, _react, _classnames, _utilsValidComponentChildren, _utilsCreateChainedFunction, _BootstrapMixin) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var SubNav = _React.createClass({
    displayName: 'SubNav',

    mixins: [_BootstrapMixin2],

    propTypes: {
      onSelect: _React.PropTypes.func,
      active: _React.PropTypes.bool,
      activeHref: _React.PropTypes.string,
      activeKey: _React.PropTypes.any,
      disabled: _React.PropTypes.bool,
      eventKey: _React.PropTypes.any,
      href: _React.PropTypes.string,
      title: _React.PropTypes.string,
      text: _React.PropTypes.node,
      target: _React.PropTypes.string
    },

    getDefaultProps: function getDefaultProps() {
      return {
        bsClass: 'nav'
      };
    },

    handleClick: function handleClick(e) {
      if (this.props.onSelect) {
        e.preventDefault();

        if (!this.props.disabled) {
          this.props.onSelect(this.props.eventKey, this.props.href, this.props.target);
        }
      }
    },

    isActive: function isActive() {
      return this.isChildActive(this);
    },

    isChildActive: function isChildActive(child) {
      var _this = this;

      if (child.props.active) {
        return true;
      }

      if (this.props.activeKey != null && this.props.activeKey === child.props.eventKey) {
        return true;
      }

      if (this.props.activeHref != null && this.props.activeHref === child.props.href) {
        return true;
      }

      if (child.props.children) {
        var _ret = (function () {
          var isActive = false;

          _ValidComponentChildren.forEach(child.props.children, function (grandchild) {
            if (this.isChildActive(grandchild)) {
              isActive = true;
            }
          }, _this);

          return {
            v: isActive
          };
        })();

        if (typeof _ret === 'object') {
          return _ret.v;
        }
      }

      return false;
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

    render: function render() {
      var classes = {
        active: this.isActive(),
        disabled: this.props.disabled
      };

      return _React.createElement(
        'li',
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        _React.createElement(
          'a',
          {
            href: this.props.href,
            title: this.props.title,
            target: this.props.target,
            onClick: this.handleClick,
            ref: 'anchor' },
          this.props.text
        ),
        _React.createElement(
          'ul',
          { className: 'nav' },
          _ValidComponentChildren.map(this.props.children, this.renderNavItem)
        )
      );
    },

    renderNavItem: function renderNavItem(child, index) {
      return _react.cloneElement(child, {
        active: this.getChildActiveProp(child),
        onSelect: _createChainedFunction(child.props.onSelect, this.props.onSelect),
        key: child.key ? child.key : index
      });
    }
  });

  module.exports = SubNav;
});