define(['exports', 'module', 'react', './OverlayMixin', './utils/createChainedFunction'], function (exports, module, _react, _OverlayMixin, _utilsCreateChainedFunction) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _React = _interopRequire(_react);

  var _OverlayMixin2 = _interopRequire(_OverlayMixin);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var ModalTrigger = _React.createClass({
    displayName: 'ModalTrigger',

    mixins: [_OverlayMixin2],

    propTypes: {
      modal: _React.PropTypes.node.isRequired
    },

    getInitialState: function getInitialState() {
      return {
        isOverlayShown: false
      };
    },

    show: function show() {
      this.setState({
        isOverlayShown: true
      });
    },

    hide: function hide() {
      this.setState({
        isOverlayShown: false
      });
    },

    toggle: function toggle() {
      this.setState({
        isOverlayShown: !this.state.isOverlayShown
      });
    },

    renderOverlay: function renderOverlay() {
      if (!this.state.isOverlayShown) {
        return _React.createElement('span', null);
      }

      return _react.cloneElement(this.props.modal, {
        onRequestHide: this.hide
      });
    },

    render: function render() {
      var child = _React.Children.only(this.props.children);
      var props = {};

      props.onClick = _createChainedFunction(child.props.onClick, this.toggle);
      props.onMouseOver = _createChainedFunction(child.props.onMouseOver, this.props.onMouseOver);
      props.onMouseOut = _createChainedFunction(child.props.onMouseOut, this.props.onMouseOut);
      props.onFocus = _createChainedFunction(child.props.onFocus, this.props.onFocus);
      props.onBlur = _createChainedFunction(child.props.onBlur, this.props.onBlur);

      return _react.cloneElement(child, props);
    }
  });

  module.exports = ModalTrigger;
});