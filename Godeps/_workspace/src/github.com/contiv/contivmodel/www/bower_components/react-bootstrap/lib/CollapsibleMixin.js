define(['exports', 'module', 'react', 'react/lib/ReactTransitionEvents', './utils/deprecationWarning'], function (exports, module, _react, _reactLibReactTransitionEvents, _utilsDeprecationWarning) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _React = _interopRequire(_react);

  var _TransitionEvents = _interopRequire(_reactLibReactTransitionEvents);

  var _deprecationWarning = _interopRequire(_utilsDeprecationWarning);

  var CollapsibleMixin = {

    propTypes: {
      defaultExpanded: _React.PropTypes.bool,
      expanded: _React.PropTypes.bool
    },

    getInitialState: function getInitialState() {
      var defaultExpanded = this.props.defaultExpanded != null ? this.props.defaultExpanded : this.props.expanded != null ? this.props.expanded : false;

      return {
        expanded: defaultExpanded,
        collapsing: false
      };
    },

    componentWillUpdate: function componentWillUpdate(nextProps, nextState) {
      var willExpanded = nextProps.expanded != null ? nextProps.expanded : nextState.expanded;
      if (willExpanded === this.isExpanded()) {
        return;
      }

      // if the expanded state is being toggled, ensure node has a dimension value
      // this is needed for the animation to work and needs to be set before
      // the collapsing class is applied (after collapsing is applied the in class
      // is removed and the node's dimension will be wrong)

      var node = this.getCollapsibleDOMNode();
      var dimension = this.dimension();
      var value = '0';

      if (!willExpanded) {
        value = this.getCollapsibleDimensionValue();
      }

      node.style[dimension] = value + 'px';

      this._afterWillUpdate();
    },

    componentDidUpdate: function componentDidUpdate(prevProps, prevState) {
      // check if expanded is being toggled; if so, set collapsing
      this._checkToggleCollapsing(prevProps, prevState);

      // check if collapsing was turned on; if so, start animation
      this._checkStartAnimation();
    },

    // helps enable test stubs
    _afterWillUpdate: function _afterWillUpdate() {},

    _checkStartAnimation: function _checkStartAnimation() {
      if (!this.state.collapsing) {
        return;
      }

      var node = this.getCollapsibleDOMNode();
      var dimension = this.dimension();
      var value = this.getCollapsibleDimensionValue();

      // setting the dimension here starts the transition animation
      var result = undefined;
      if (this.isExpanded()) {
        result = value + 'px';
      } else {
        result = '0px';
      }
      node.style[dimension] = result;
    },

    _checkToggleCollapsing: function _checkToggleCollapsing(prevProps, prevState) {
      var wasExpanded = prevProps.expanded != null ? prevProps.expanded : prevState.expanded;
      var isExpanded = this.isExpanded();
      if (wasExpanded !== isExpanded) {
        if (wasExpanded) {
          this._handleCollapse();
        } else {
          this._handleExpand();
        }
      }
    },

    _handleExpand: function _handleExpand() {
      var _this = this;

      var node = this.getCollapsibleDOMNode();
      var dimension = this.dimension();

      var complete = function complete() {
        _this._removeEndEventListener(node, complete);
        // remove dimension value - this ensures the collapsible item can grow
        // in dimension after initial display (such as an image loading)
        node.style[dimension] = '';
        _this.setState({
          collapsing: false
        });
      };

      this._addEndEventListener(node, complete);

      this.setState({
        collapsing: true
      });
    },

    _handleCollapse: function _handleCollapse() {
      var _this2 = this;

      var node = this.getCollapsibleDOMNode();

      var complete = function complete() {
        _this2._removeEndEventListener(node, complete);
        _this2.setState({
          collapsing: false
        });
      };

      this._addEndEventListener(node, complete);

      this.setState({
        collapsing: true
      });
    },

    // helps enable test stubs
    _addEndEventListener: function _addEndEventListener(node, complete) {
      _TransitionEvents.addEndEventListener(node, complete);
    },

    // helps enable test stubs
    _removeEndEventListener: function _removeEndEventListener(node, complete) {
      _TransitionEvents.removeEndEventListener(node, complete);
    },

    dimension: function dimension() {
      if (typeof this.getCollapsableDimension === 'function') {
        _deprecationWarning('CollapsableMixin.getCollapsableDimension()', 'CollapsibleMixin.getCollapsibleDimension()', 'https://github.com/react-bootstrap/react-bootstrap/issues/425#issuecomment-97110963');
        return this.getCollapsableDimension();
      }

      return typeof this.getCollapsibleDimension === 'function' ? this.getCollapsibleDimension() : 'height';
    },

    isExpanded: function isExpanded() {
      return this.props.expanded != null ? this.props.expanded : this.state.expanded;
    },

    getCollapsibleClassSet: function getCollapsibleClassSet(className) {
      var classes = {};

      if (typeof className === 'string') {
        className.split(' ').forEach(function (subClasses) {
          if (subClasses) {
            classes[subClasses] = true;
          }
        });
      }

      classes.collapsing = this.state.collapsing;
      classes.collapse = !this.state.collapsing;
      classes['in'] = this.isExpanded() && !this.state.collapsing;

      return classes;
    }
  };

  module.exports = CollapsibleMixin;
});