define(['exports', 'module', 'react', 'classnames', './BootstrapMixin', './FadeMixin', './utils/domUtils', './utils/EventListener'], function (exports, module, _react, _classnames, _BootstrapMixin, _FadeMixin, _utilsDomUtils, _utilsEventListener) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _FadeMixin2 = _interopRequire(_FadeMixin);

  var _domUtils = _interopRequire(_utilsDomUtils);

  var _EventListener = _interopRequire(_utilsEventListener);

  // TODO:
  // - aria-labelledby
  // - Add `modal-body` div if only one child passed in that doesn't already have it
  // - Tests

  var Modal = _React.createClass({
    displayName: 'Modal',

    mixins: [_BootstrapMixin2, _FadeMixin2],

    propTypes: {
      title: _React.PropTypes.node,
      backdrop: _React.PropTypes.oneOf(['static', true, false]),
      keyboard: _React.PropTypes.bool,
      closeButton: _React.PropTypes.bool,
      animation: _React.PropTypes.bool,
      onRequestHide: _React.PropTypes.func.isRequired
    },

    getDefaultProps: function getDefaultProps() {
      return {
        bsClass: 'modal',
        backdrop: true,
        keyboard: true,
        animation: true,
        closeButton: true
      };
    },

    render: function render() {
      var modalStyle = { display: 'block' };
      var dialogClasses = this.getBsClassSet();
      delete dialogClasses.modal;
      dialogClasses['modal-dialog'] = true;

      var classes = {
        modal: true,
        fade: this.props.animation,
        'in': !this.props.animation || !document.querySelectorAll
      };

      var modal = _React.createElement(
        'div',
        _extends({}, this.props, {
          title: null,
          tabIndex: '-1',
          role: 'dialog',
          style: modalStyle,
          className: _classNames(this.props.className, classes),
          onClick: this.props.backdrop === true ? this.handleBackdropClick : null,
          ref: 'modal' }),
        _React.createElement(
          'div',
          { className: _classNames(dialogClasses) },
          _React.createElement(
            'div',
            { className: 'modal-content', style: { overflow: 'hidden' } },
            this.props.title ? this.renderHeader() : null,
            this.props.children
          )
        )
      );

      return this.props.backdrop ? this.renderBackdrop(modal) : modal;
    },

    renderBackdrop: function renderBackdrop(modal) {
      var classes = {
        'modal-backdrop': true,
        fade: this.props.animation
      };

      classes['in'] = !this.props.animation || !document.querySelectorAll;

      var onClick = this.props.backdrop === true ? this.handleBackdropClick : null;

      return _React.createElement(
        'div',
        null,
        _React.createElement('div', { className: _classNames(classes), ref: 'backdrop', onClick: onClick }),
        modal
      );
    },

    renderHeader: function renderHeader() {
      var closeButton = undefined;
      if (this.props.closeButton) {
        closeButton = _React.createElement(
          'button',
          { type: 'button', className: 'close', 'aria-hidden': 'true', onClick: this.props.onRequestHide },
          '×'
        );
      }

      var style = this.props.bsStyle;
      var classes = {
        'modal-header': true
      };
      classes['bg-' + style] = style;
      classes['text-' + style] = style;

      var className = _classNames(classes);

      return _React.createElement(
        'div',
        { className: className },
        closeButton,
        this.renderTitle()
      );
    },

    renderTitle: function renderTitle() {
      return _React.isValidElement(this.props.title) ? this.props.title : _React.createElement(
        'h4',
        { className: 'modal-title' },
        this.props.title
      );
    },

    iosClickHack: function iosClickHack() {
      // IOS only allows click events to be delegated to the document on elements
      // it considers 'clickable' - anchors, buttons, etc. We fake a click handler on the
      // DOM nodes themselves. Remove if handled by React: https://github.com/facebook/react/issues/1169
      _React.findDOMNode(this.refs.modal).onclick = function () {};
      _React.findDOMNode(this.refs.backdrop).onclick = function () {};
    },

    componentDidMount: function componentDidMount() {
      this._onDocumentKeyupListener = _EventListener.listen(_domUtils.ownerDocument(this), 'keyup', this.handleDocumentKeyUp);

      var container = this.props.container && _React.findDOMNode(this.props.container) || _domUtils.ownerDocument(this).body;
      container.className += container.className.length ? ' modal-open' : 'modal-open';

      if (this.props.backdrop) {
        this.iosClickHack();
      }
    },

    componentDidUpdate: function componentDidUpdate(prevProps) {
      if (this.props.backdrop && this.props.backdrop !== prevProps.backdrop) {
        this.iosClickHack();
      }
    },

    componentWillUnmount: function componentWillUnmount() {
      this._onDocumentKeyupListener.remove();
      var container = this.props.container && _React.findDOMNode(this.props.container) || _domUtils.ownerDocument(this).body;
      container.className = container.className.replace(/ ?modal-open/, '');
    },

    handleBackdropClick: function handleBackdropClick(e) {
      if (e.target !== e.currentTarget) {
        return;
      }

      this.props.onRequestHide();
    },

    handleDocumentKeyUp: function handleDocumentKeyUp(e) {
      if (this.props.keyboard && e.keyCode === 27) {
        this.props.onRequestHide();
      }
    }
  });

  module.exports = Modal;
});