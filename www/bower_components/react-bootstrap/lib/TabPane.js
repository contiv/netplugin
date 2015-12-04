define(['exports', 'module', 'react', 'classnames', './utils/TransitionEvents'], function (exports, module, _react, _classnames, _utilsTransitionEvents) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _TransitionEvents = _interopRequire(_utilsTransitionEvents);

  var TabPane = _React.createClass({
    displayName: 'TabPane',

    propTypes: {
      active: _React.PropTypes.bool,
      animation: _React.PropTypes.bool,
      onAnimateOutEnd: _React.PropTypes.func
    },

    getDefaultProps: function getDefaultProps() {
      return {
        animation: true
      };
    },

    getInitialState: function getInitialState() {
      return {
        animateIn: false,
        animateOut: false
      };
    },

    componentWillReceiveProps: function componentWillReceiveProps(nextProps) {
      if (this.props.animation) {
        if (!this.state.animateIn && nextProps.active && !this.props.active) {
          this.setState({
            animateIn: true
          });
        } else if (!this.state.animateOut && !nextProps.active && this.props.active) {
          this.setState({
            animateOut: true
          });
        }
      }
    },

    componentDidUpdate: function componentDidUpdate() {
      if (this.state.animateIn) {
        setTimeout(this.startAnimateIn, 0);
      }
      if (this.state.animateOut) {
        _TransitionEvents.addEndEventListener(_React.findDOMNode(this), this.stopAnimateOut);
      }
    },

    startAnimateIn: function startAnimateIn() {
      if (this.isMounted()) {
        this.setState({
          animateIn: false
        });
      }
    },

    stopAnimateOut: function stopAnimateOut() {
      if (this.isMounted()) {
        this.setState({
          animateOut: false
        });

        if (this.props.onAnimateOutEnd) {
          this.props.onAnimateOutEnd();
        }
      }
    },

    render: function render() {
      var classes = {
        'tab-pane': true,
        fade: true,
        active: this.props.active || this.state.animateOut,
        'in': this.props.active && !this.state.animateIn
      };

      return _React.createElement(
        'div',
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        this.props.children
      );
    }
  });

  module.exports = TabPane;
});