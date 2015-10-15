define(['exports', 'module', 'react', 'classnames', './BootstrapMixin', './DropdownStateMixin', './Button', './ButtonGroup', './DropdownMenu'], function (exports, module, _react, _classnames, _BootstrapMixin, _DropdownStateMixin, _Button, _ButtonGroup, _DropdownMenu) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  /* eslint react/prop-types: [1, {ignore: ["children", "className", "bsSize"]}]*/
  /* BootstrapMixin contains `bsSize` type validation */

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _DropdownStateMixin2 = _interopRequire(_DropdownStateMixin);

  var _Button2 = _interopRequire(_Button);

  var _ButtonGroup2 = _interopRequire(_ButtonGroup);

  var _DropdownMenu2 = _interopRequire(_DropdownMenu);

  var SplitButton = _React.createClass({
    displayName: 'SplitButton',

    mixins: [_BootstrapMixin2, _DropdownStateMixin2],

    propTypes: {
      pullRight: _React.PropTypes.bool,
      title: _React.PropTypes.node,
      href: _React.PropTypes.string,
      id: _React.PropTypes.string,
      target: _React.PropTypes.string,
      dropdownTitle: _React.PropTypes.node,
      dropup: _React.PropTypes.bool,
      onClick: _React.PropTypes.func,
      onSelect: _React.PropTypes.func,
      disabled: _React.PropTypes.bool
    },

    getDefaultProps: function getDefaultProps() {
      return {
        dropdownTitle: 'Toggle dropdown'
      };
    },

    render: function render() {
      var groupClasses = {
        open: this.state.open,
        dropup: this.props.dropup
      };

      var button = _React.createElement(
        _Button2,
        _extends({}, this.props, {
          ref: 'button',
          onClick: this.handleButtonClick,
          title: null,
          id: null }),
        this.props.title
      );

      var dropdownButton = _React.createElement(
        _Button2,
        _extends({}, this.props, {
          ref: 'dropdownButton',
          className: _classNames(this.props.className, 'dropdown-toggle'),
          onClick: this.handleDropdownClick,
          title: null,
          href: null,
          target: null,
          id: null }),
        _React.createElement(
          'span',
          { className: 'sr-only' },
          this.props.dropdownTitle
        ),
        _React.createElement('span', { className: 'caret' }),
        _React.createElement(
          'span',
          { style: { letterSpacing: '-.3em' } },
          ' '
        )
      );

      return _React.createElement(
        _ButtonGroup2,
        {
          bsSize: this.props.bsSize,
          className: _classNames(groupClasses),
          id: this.props.id },
        button,
        dropdownButton,
        _React.createElement(
          _DropdownMenu2,
          {
            ref: 'menu',
            onSelect: this.handleOptionSelect,
            'aria-labelledby': this.props.id,
            pullRight: this.props.pullRight },
          this.props.children
        )
      );
    },

    handleButtonClick: function handleButtonClick(e) {
      if (this.state.open) {
        this.setDropdownState(false);
      }

      if (this.props.onClick) {
        this.props.onClick(e, this.props.href, this.props.target);
      }
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

  module.exports = SplitButton;
});