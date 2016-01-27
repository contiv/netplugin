define(['exports', 'module', 'react', './utils/CustomPropTypes', './utils/domUtils'], function (exports, module, _react, _utilsCustomPropTypes, _utilsDomUtils) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _React = _interopRequire(_react);

  var _CustomPropTypes = _interopRequire(_utilsCustomPropTypes);

  var _domUtils = _interopRequire(_utilsDomUtils);

  module.exports = {
    propTypes: {
      container: _CustomPropTypes.mountable
    },

    componentWillUnmount: function componentWillUnmount() {
      this._unrenderOverlay();
      if (this._overlayTarget) {
        this.getContainerDOMNode().removeChild(this._overlayTarget);
        this._overlayTarget = null;
      }
    },

    componentDidUpdate: function componentDidUpdate() {
      this._renderOverlay();
    },

    componentDidMount: function componentDidMount() {
      this._renderOverlay();
    },

    _mountOverlayTarget: function _mountOverlayTarget() {
      this._overlayTarget = document.createElement('div');
      this.getContainerDOMNode().appendChild(this._overlayTarget);
    },

    _renderOverlay: function _renderOverlay() {
      if (!this._overlayTarget) {
        this._mountOverlayTarget();
      }

      var overlay = this.renderOverlay();

      // Save reference to help testing
      if (overlay !== null) {
        this._overlayInstance = _React.render(overlay, this._overlayTarget);
      } else {
        // Unrender if the component is null for transitions to null
        this._unrenderOverlay();
      }
    },

    _unrenderOverlay: function _unrenderOverlay() {
      _React.unmountComponentAtNode(this._overlayTarget);
      this._overlayInstance = null;
    },

    getOverlayDOMNode: function getOverlayDOMNode() {
      if (!this.isMounted()) {
        throw new Error('getOverlayDOMNode(): A component must be mounted to have a DOM node.');
      }

      if (this._overlayInstance) {
        return _React.findDOMNode(this._overlayInstance);
      }

      return null;
    },

    getContainerDOMNode: function getContainerDOMNode() {
      return _React.findDOMNode(this.props.container) || _domUtils.ownerDocument(this).body;
    }
  };
});