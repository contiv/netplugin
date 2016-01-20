define(['exports', 'module', 'react', './utils/domUtils'], function (exports, module, _react, _utilsDomUtils) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _React = _interopRequire(_react);

  var _domUtils = _interopRequire(_utilsDomUtils);

  // TODO: listen for onTransitionEnd to remove el
  function getElementsAndSelf(root, classes) {
    var els = root.querySelectorAll('.' + classes.join('.'));

    els = [].map.call(els, function (e) {
      return e;
    });

    for (var i = 0; i < classes.length; i++) {
      if (!root.className.match(new RegExp('\\b' + classes[i] + '\\b'))) {
        return els;
      }
    }
    els.unshift(root);
    return els;
  }

  module.exports = {
    _fadeIn: function _fadeIn() {
      var els = undefined;

      if (this.isMounted()) {
        els = getElementsAndSelf(_React.findDOMNode(this), ['fade']);

        if (els.length) {
          els.forEach(function (el) {
            el.className += ' in';
          });
        }
      }
    },

    _fadeOut: function _fadeOut() {
      var els = getElementsAndSelf(this._fadeOutEl, ['fade', 'in']);

      if (els.length) {
        els.forEach(function (el) {
          el.className = el.className.replace(/\bin\b/, '');
        });
      }

      setTimeout(this._handleFadeOutEnd, 300);
    },

    _handleFadeOutEnd: function _handleFadeOutEnd() {
      if (this._fadeOutEl && this._fadeOutEl.parentNode) {
        this._fadeOutEl.parentNode.removeChild(this._fadeOutEl);
      }
    },

    componentDidMount: function componentDidMount() {
      if (document.querySelectorAll) {
        // Firefox needs delay for transition to be triggered
        setTimeout(this._fadeIn, 20);
      }
    },

    componentWillUnmount: function componentWillUnmount() {
      var els = getElementsAndSelf(_React.findDOMNode(this), ['fade']),
          container = this.props.container && _React.findDOMNode(this.props.container) || _domUtils.ownerDocument(this).body;

      if (els.length) {
        this._fadeOutEl = document.createElement('div');
        container.appendChild(this._fadeOutEl);
        this._fadeOutEl.appendChild(_React.findDOMNode(this).cloneNode(true));
        // Firefox needs delay for transition to be triggered
        setTimeout(this._fadeOut, 20);
      }
    }
  };
});