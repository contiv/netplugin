define(['exports', 'module', 'react', 'classnames', './utils/createChainedFunction', './BootstrapMixin', './DropdownStateMixin', './Button', './ButtonGroup', './DropdownMenu', './utils/ValidComponentChildren'], function (exports, module, _react, _classnames, _utilsCreateChainedFunction, _BootstrapMixin, _DropdownStateMixin, _Button, _ButtonGroup, _DropdownMenu, _utilsValidComponentChildren) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _DropdownStateMixin2 = _interopRequire(_DropdownStateMixin);

  var _Button2 = _interopRequire(_Button);

  var _ButtonGroup2 = _interopRequire(_ButtonGroup);

  var _DropdownMenu2 = _interopRequire(_DropdownMenu);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var DropdownButton = _React.createClass({
    displayName: 'DropdownButton',

    mixins: [_BootstrapMixin2, _DropdownStateMixin2],

    propTypes: {
      pullRight: _React.PropTypes.bool,
      dropup: _React.PropTypes.bool,
      title: _React.PropTypes.node,
      href: _React.PropTypes.string,
      onClick: _React.PropTypes.func,
      onSelect: _React.PropTypes.func,
      navItem: _React.PropTypes.bool,
      noCaret: _React.PropTypes.bool,
      buttonClassName: _React.PropTypes.string
    },

    render: function render() {
      var renderMethod = this.props.navItem ? 'renderNavItem' : 'renderButtonGroup';

      var caret = this.props.noCaret ? null : _React.createElement('span', { className: 'caret' });

      return this[renderMethod]([_React.createElement(
        _Button2,
        _extends({}, this.props, {
          ref: 'dropdownButton',
          className: _classNames('dropdown-toggle', this.props.buttonClassName),
          onClick: this.handleDropdownClick,
          key: 0,
          navDropdown: this.props.navItem,
          navItem: null,
          title: null,
          pullRight: null,
          dropup: null }),
        this.props.title,
        ' ',
        caret
      ), _React.createElement(
        _DropdownMenu2,
        {
          ref: 'menu',
          'aria-labelledby': this.props.id,
          pullRight: this.props.pullRight,
          key: 1 },
        _ValidComponentChildren.map(this.props.children, this.renderMenuItem)
      )]);
    },

    renderButtonGroup: function renderButtonGroup(children) {
      var groupClasses = {
        open: this.state.open,
        dropup: this.props.dropup
      };

      return _React.createElement(
        _ButtonGroup2,
        {
          bsSize: this.props.bsSize,
          className: _classNames(this.props.className, groupClasses) },
        children
      );
    },

    renderNavItem: function renderNavItem(children) {
      var classes = {
        dropdown: true,
        open: this.state.open,
        dropup: this.props.dropup
      };

      return _React.createElement(
        'li',
        { className: _classNames(this.props.className, classes) },
        children
      );
    },

    renderMenuItem: function renderMenuItem(child, index) {
      // Only handle the option selection if an onSelect prop has been set on the
      // component or it's child, this allows a user not to pass an onSelect
      // handler and have the browser preform the default action.
      var handleOptionSelect = this.props.onSelect || child.props.onSelect ? this.handleOptionSelect : null;

      return _react.cloneElement(child, {
        // Capture onSelect events
        onSelect: _createChainedFunction(child.props.onSelect, handleOptionSelect),
        key: child.key ? child.key : index
      });
    },

    handleDropdownClick: function handleDropdownClick(e) {
      e.preventDefault();

      this.setDropdownState(!this.state.open);
    },

    handleOptionSelect: function handleOptionSelect(key) {
      if (this.props.onSelect) {
        this.props.onSelect(key);
      }

      this.setDropdownState(false);
    }
  });

  module.exports = DropdownButton;
});