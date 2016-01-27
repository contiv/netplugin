define(['exports', 'module', 'react', './BootstrapMixin', './utils/ValidComponentChildren', './Nav', './NavItem'], function (exports, module, _react, _BootstrapMixin, _utilsValidComponentChildren, _Nav, _NavItem) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var _Nav2 = _interopRequire(_Nav);

  var _NavItem2 = _interopRequire(_NavItem);

  function getDefaultActiveKeyFromChildren(children) {
    var defaultActiveKey = undefined;

    _ValidComponentChildren.forEach(children, function (child) {
      if (defaultActiveKey == null) {
        defaultActiveKey = child.props.eventKey;
      }
    });

    return defaultActiveKey;
  }

  var TabbedArea = _React.createClass({
    displayName: 'TabbedArea',

    mixins: [_BootstrapMixin2],

    propTypes: {
      activeKey: _React.PropTypes.any,
      defaultActiveKey: _React.PropTypes.any,
      bsStyle: _React.PropTypes.oneOf(['tabs', 'pills']),
      animation: _React.PropTypes.bool,
      id: _React.PropTypes.string,
      onSelect: _React.PropTypes.func
    },

    getDefaultProps: function getDefaultProps() {
      return {
        bsStyle: 'tabs',
        animation: true
      };
    },

    getInitialState: function getInitialState() {
      var defaultActiveKey = this.props.defaultActiveKey != null ? this.props.defaultActiveKey : getDefaultActiveKeyFromChildren(this.props.children);

      return {
        activeKey: defaultActiveKey,
        previousActiveKey: null
      };
    },

    componentWillReceiveProps: function componentWillReceiveProps(nextProps) {
      if (nextProps.activeKey != null && nextProps.activeKey !== this.props.activeKey) {
        this.setState({
          previousActiveKey: this.props.activeKey
        });
      }
    },

    handlePaneAnimateOutEnd: function handlePaneAnimateOutEnd() {
      this.setState({
        previousActiveKey: null
      });
    },

    render: function render() {
      var activeKey = this.props.activeKey != null ? this.props.activeKey : this.state.activeKey;

      function renderTabIfSet(child) {
        return child.props.tab != null ? this.renderTab(child) : null;
      }

      var nav = _React.createElement(
        _Nav2,
        _extends({}, this.props, { activeKey: activeKey, onSelect: this.handleSelect, ref: 'tabs' }),
        _ValidComponentChildren.map(this.props.children, renderTabIfSet, this)
      );

      return _React.createElement(
        'div',
        null,
        nav,
        _React.createElement(
          'div',
          { id: this.props.id, className: 'tab-content', ref: 'panes' },
          _ValidComponentChildren.map(this.props.children, this.renderPane)
        )
      );
    },

    getActiveKey: function getActiveKey() {
      return this.props.activeKey != null ? this.props.activeKey : this.state.activeKey;
    },

    renderPane: function renderPane(child, index) {
      var activeKey = this.getActiveKey();

      return _react.cloneElement(child, {
        active: child.props.eventKey === activeKey && (this.state.previousActiveKey == null || !this.props.animation),
        key: child.key ? child.key : index,
        animation: this.props.animation,
        onAnimateOutEnd: this.state.previousActiveKey != null && child.props.eventKey === this.state.previousActiveKey ? this.handlePaneAnimateOutEnd : null
      });
    },

    renderTab: function renderTab(child) {
      var _child$props = child.props;
      var eventKey = _child$props.eventKey;
      var className = _child$props.className;
      var tab = _child$props.tab;

      return _React.createElement(
        _NavItem2,
        {
          ref: 'tab' + eventKey,
          eventKey: eventKey,
          className: className },
        tab
      );
    },

    shouldComponentUpdate: function shouldComponentUpdate() {
      // Defer any updates to this component during the `onSelect` handler.
      return !this._isChanging;
    },

    handleSelect: function handleSelect(key) {
      if (this.props.onSelect) {
        this._isChanging = true;
        this.props.onSelect(key);
        this._isChanging = false;
      } else if (key !== this.getActiveKey()) {
        this.setState({
          activeKey: key,
          previousActiveKey: this.getActiveKey()
        });
      }
    }
  });

  module.exports = TabbedArea;
});