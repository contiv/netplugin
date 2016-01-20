define(['exports', 'module', 'react', './utils/domUtils', './utils/EventListener'], function (exports, module, _react, _utilsDomUtils, _utilsEventListener) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _React = _interopRequire(_react);

  var _domUtils = _interopRequire(_utilsDomUtils);

  var _EventListener = _interopRequire(_utilsEventListener);

  var AffixMixin = {
    propTypes: {
      offset: _React.PropTypes.number,
      offsetTop: _React.PropTypes.number,
      offsetBottom: _React.PropTypes.number
    },

    getInitialState: function getInitialState() {
      return {
        affixClass: 'affix-top'
      };
    },

    getPinnedOffset: function getPinnedOffset(DOMNode) {
      if (this.pinnedOffset) {
        return this.pinnedOffset;
      }

      DOMNode.className = DOMNode.className.replace(/affix-top|affix-bottom|affix/, '');
      DOMNode.className += DOMNode.className.length ? ' affix' : 'affix';

      this.pinnedOffset = _domUtils.getOffset(DOMNode).top - window.pageYOffset;

      return this.pinnedOffset;
    },

    checkPosition: function checkPosition() {
      var DOMNode = undefined,
          scrollHeight = undefined,
          scrollTop = undefined,
          position = undefined,
          offsetTop = undefined,
          offsetBottom = undefined,
          affix = undefined,
          affixType = undefined,
          affixPositionTop = undefined;

      // TODO: or not visible
      if (!this.isMounted()) {
        return;
      }

      DOMNode = _React.findDOMNode(this);
      scrollHeight = document.documentElement.offsetHeight;
      scrollTop = window.pageYOffset;
      position = _domUtils.getOffset(DOMNode);

      if (this.affixed === 'top') {
        position.top += scrollTop;
      }

      offsetTop = this.props.offsetTop != null ? this.props.offsetTop : this.props.offset;
      offsetBottom = this.props.offsetBottom != null ? this.props.offsetBottom : this.props.offset;

      if (offsetTop == null && offsetBottom == null) {
        return;
      }
      if (offsetTop == null) {
        offsetTop = 0;
      }
      if (offsetBottom == null) {
        offsetBottom = 0;
      }

      if (this.unpin != null && scrollTop + this.unpin <= position.top) {
        affix = false;
      } else if (offsetBottom != null && position.top + DOMNode.offsetHeight >= scrollHeight - offsetBottom) {
        affix = 'bottom';
      } else if (offsetTop != null && scrollTop <= offsetTop) {
        affix = 'top';
      } else {
        affix = false;
      }

      if (this.affixed === affix) {
        return;
      }

      if (this.unpin != null) {
        DOMNode.style.top = '';
      }

      affixType = 'affix' + (affix ? '-' + affix : '');

      this.affixed = affix;
      this.unpin = affix === 'bottom' ? this.getPinnedOffset(DOMNode) : null;

      if (affix === 'bottom') {
        DOMNode.className = DOMNode.className.replace(/affix-top|affix-bottom|affix/, 'affix-bottom');
        affixPositionTop = scrollHeight - offsetBottom - DOMNode.offsetHeight - _domUtils.getOffset(DOMNode).top;
      }

      this.setState({
        affixClass: affixType,
        affixPositionTop: affixPositionTop
      });
    },

    checkPositionWithEventLoop: function checkPositionWithEventLoop() {
      setTimeout(this.checkPosition, 0);
    },

    componentDidMount: function componentDidMount() {
      this._onWindowScrollListener = _EventListener.listen(window, 'scroll', this.checkPosition);
      this._onDocumentClickListener = _EventListener.listen(_domUtils.ownerDocument(this), 'click', this.checkPositionWithEventLoop);
    },

    componentWillUnmount: function componentWillUnmount() {
      if (this._onWindowScrollListener) {
        this._onWindowScrollListener.remove();
      }

      if (this._onDocumentClickListener) {
        this._onDocumentClickListener.remove();
      }
    },

    componentDidUpdate: function componentDidUpdate(prevProps, prevState) {
      if (prevState.affixClass === this.state.affixClass) {
        this.checkPositionWithEventLoop();
      }
    }
  };

  module.exports = AffixMixin;
});