define(['exports', 'module', 'react', './OverlayMixin', './utils/domUtils', './utils/createChainedFunction', './utils/Object.assign'], function (exports, module, _react, _OverlayMixin, _utilsDomUtils, _utilsCreateChainedFunction, _utilsObjectAssign) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _React = _interopRequire(_react);

  var _OverlayMixin2 = _interopRequire(_OverlayMixin);

  var _domUtils = _interopRequire(_utilsDomUtils);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var _assign = _interopRequire(_utilsObjectAssign);

  /**
   * Check if value one is inside or equal to the of value
   *
   * @param {string} one
   * @param {string|array} of
   * @returns {boolean}
   */
  function isOneOf(one, of) {
    if (Array.isArray(of)) {
      return of.indexOf(one) >= 0;
    }
    return one === of;
  }

  var OverlayTrigger = _React.createClass({
    displayName: 'OverlayTrigger',

    mixins: [_OverlayMixin2],

    propTypes: {
      trigger: _React.PropTypes.oneOfType([_React.PropTypes.oneOf(['manual', 'click', 'hover', 'focus']), _React.PropTypes.arrayOf(_React.PropTypes.oneOf(['click', 'hover', 'focus']))]),
      placement: _React.PropTypes.oneOf(['top', 'right', 'bottom', 'left']),
      delay: _React.PropTypes.number,
      delayShow: _React.PropTypes.number,
      delayHide: _React.PropTypes.number,
      defaultOverlayShown: _React.PropTypes.bool,
      overlay: _React.PropTypes.node.isRequired
    },

    getDefaultProps: function getDefaultProps() {
      return {
        placement: 'right',
        trigger: ['hover', 'focus']
      };
    },

    getInitialState: function getInitialState() {
      return {
        isOverlayShown: this.props.defaultOverlayShown == null ? false : this.props.defaultOverlayShown,
        overlayLeft: null,
        overlayTop: null
      };
    },

    show: function show() {
      this.setState({
        isOverlayShown: true
      }, function () {
        this.updateOverlayPosition();
      });
    },

    hide: function hide() {
      this.setState({
        isOverlayShown: false
      });
    },

    toggle: function toggle() {
      if (this.state.isOverlayShown) {
        this.hide();
      } else {
        this.show();
      }
    },

    renderOverlay: function renderOverlay() {
      if (!this.state.isOverlayShown) {
        return _React.createElement('span', null);
      }

      return _react.cloneElement(this.props.overlay, {
        onRequestHide: this.hide,
        placement: this.props.placement,
        positionLeft: this.state.overlayLeft,
        positionTop: this.state.overlayTop
      });
    },

    render: function render() {
      var child = _React.Children.only(this.props.children);
      if (this.props.trigger === 'manual') {
        return child;
      }

      var props = {};

      props.onClick = _createChainedFunction(child.props.onClick, this.props.onClick);
      if (isOneOf('click', this.props.trigger)) {
        props.onClick = _createChainedFunction(this.toggle, props.onClick);
      }

      if (isOneOf('hover', this.props.trigger)) {
        props.onMouseOver = _createChainedFunction(this.handleDelayedShow, this.props.onMouseOver);
        props.onMouseOut = _createChainedFunction(this.handleDelayedHide, this.props.onMouseOut);
      }

      if (isOneOf('focus', this.props.trigger)) {
        props.onFocus = _createChainedFunction(this.handleDelayedShow, this.props.onFocus);
        props.onBlur = _createChainedFunction(this.handleDelayedHide, this.props.onBlur);
      }

      return _react.cloneElement(child, props);
    },

    componentWillUnmount: function componentWillUnmount() {
      clearTimeout(this._hoverDelay);
    },

    componentDidMount: function componentDidMount() {
      if (this.props.defaultOverlayShown) {
        this.updateOverlayPosition();
      }
    },

    handleDelayedShow: function handleDelayedShow() {
      if (this._hoverDelay != null) {
        clearTimeout(this._hoverDelay);
        this._hoverDelay = null;
        return;
      }

      var delay = this.props.delayShow != null ? this.props.delayShow : this.props.delay;

      if (!delay) {
        this.show();
        return;
      }

      this._hoverDelay = setTimeout((function () {
        this._hoverDelay = null;
        this.show();
      }).bind(this), delay);
    },

    handleDelayedHide: function handleDelayedHide() {
      if (this._hoverDelay != null) {
        clearTimeout(this._hoverDelay);
        this._hoverDelay = null;
        return;
      }

      var delay = this.props.delayHide != null ? this.props.delayHide : this.props.delay;

      if (!delay) {
        this.hide();
        return;
      }

      this._hoverDelay = setTimeout((function () {
        this._hoverDelay = null;
        this.hide();
      }).bind(this), delay);
    },

    updateOverlayPosition: function updateOverlayPosition() {
      if (!this.isMounted()) {
        return;
      }

      var pos = this.calcOverlayPosition();

      this.setState({
        overlayLeft: pos.left,
        overlayTop: pos.top
      });
    },

    calcOverlayPosition: function calcOverlayPosition() {
      var childOffset = this.getPosition();

      var overlayNode = this.getOverlayDOMNode();
      var overlayHeight = overlayNode.offsetHeight;
      var overlayWidth = overlayNode.offsetWidth;

      switch (this.props.placement) {
        case 'right':
          return {
            top: childOffset.top + childOffset.height / 2 - overlayHeight / 2,
            left: childOffset.left + childOffset.width
          };
        case 'left':
          return {
            top: childOffset.top + childOffset.height / 2 - overlayHeight / 2,
            left: childOffset.left - overlayWidth
          };
        case 'top':
          return {
            top: childOffset.top - overlayHeight,
            left: childOffset.left + childOffset.width / 2 - overlayWidth / 2
          };
        case 'bottom':
          return {
            top: childOffset.top + childOffset.height,
            left: childOffset.left + childOffset.width / 2 - overlayWidth / 2
          };
        default:
          throw new Error('calcOverlayPosition(): No such placement of "' + this.props.placement + '" found.');
      }
    },

    getPosition: function getPosition() {
      var node = _React.findDOMNode(this);
      var container = this.getContainerDOMNode();

      var offset = container.tagName === 'BODY' ? _domUtils.getOffset(node) : _domUtils.getPosition(node, container);

      return _assign({}, offset, {
        height: node.offsetHeight,
        width: node.offsetWidth
      });
    }
  });

  module.exports = OverlayTrigger;
});