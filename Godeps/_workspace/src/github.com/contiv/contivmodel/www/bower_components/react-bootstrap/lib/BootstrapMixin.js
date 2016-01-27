define(['exports', 'module', './styleMaps', './utils/CustomPropTypes'], function (exports, module, _styleMaps, _utilsCustomPropTypes) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _styleMaps2 = _interopRequire(_styleMaps);

  var _CustomPropTypes = _interopRequire(_utilsCustomPropTypes);

  var BootstrapMixin = {
    propTypes: {
      bsClass: _CustomPropTypes.keyOf(_styleMaps2.CLASSES),
      bsStyle: _CustomPropTypes.keyOf(_styleMaps2.STYLES),
      bsSize: _CustomPropTypes.keyOf(_styleMaps2.SIZES)
    },

    getBsClassSet: function getBsClassSet() {
      var classes = {};

      var bsClass = this.props.bsClass && _styleMaps2.CLASSES[this.props.bsClass];
      if (bsClass) {
        classes[bsClass] = true;

        var prefix = bsClass + '-';

        var bsSize = this.props.bsSize && _styleMaps2.SIZES[this.props.bsSize];
        if (bsSize) {
          classes[prefix + bsSize] = true;
        }

        var bsStyle = this.props.bsStyle && _styleMaps2.STYLES[this.props.bsStyle];
        if (this.props.bsStyle) {
          classes[prefix + bsStyle] = true;
        }
      }

      return classes;
    },

    prefixClass: function prefixClass(subClass) {
      return _styleMaps2.CLASSES[this.props.bsClass] + '-' + subClass;
    }
  };

  module.exports = BootstrapMixin;
});