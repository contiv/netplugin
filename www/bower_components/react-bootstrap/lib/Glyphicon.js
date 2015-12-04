define(['exports', 'module', 'react', 'classnames', './BootstrapMixin', './styleMaps'], function (exports, module, _react, _classnames, _BootstrapMixin, _styleMaps) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _styleMaps2 = _interopRequire(_styleMaps);

  var Glyphicon = _React.createClass({
    displayName: 'Glyphicon',

    mixins: [_BootstrapMixin2],

    propTypes: {
      glyph: _React.PropTypes.oneOf(_styleMaps2.GLYPHS).isRequired
    },

    getDefaultProps: function getDefaultProps() {
      return {
        bsClass: 'glyphicon'
      };
    },

    render: function render() {
      var classes = this.getBsClassSet();

      classes['glyphicon-' + this.props.glyph] = true;

      return _React.createElement(
        'span',
        _extends({}, this.props, { className: _classNames(this.props.className, classes) }),
        this.props.children
      );
    }
  });

  module.exports = Glyphicon;
});