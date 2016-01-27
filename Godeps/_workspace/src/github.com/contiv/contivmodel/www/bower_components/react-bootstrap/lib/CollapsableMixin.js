define(['exports', 'module', './utils/Object.assign', './utils/deprecationWarning', './CollapsibleMixin'], function (exports, module, _utilsObjectAssign, _utilsDeprecationWarning, _CollapsibleMixin) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _assign = _interopRequire(_utilsObjectAssign);

  var _deprecationWarning = _interopRequire(_utilsDeprecationWarning);

  var _CollapsibleMixin2 = _interopRequire(_CollapsibleMixin);

  var link = 'https://github.com/react-bootstrap/react-bootstrap/issues/425#issuecomment-97110963';

  var CollapsableMixin = _assign({}, _CollapsibleMixin2, {
    getCollapsableClassSet: function getCollapsableClassSet(className) {
      _deprecationWarning('CollapsableMixin.getCollapsableClassSet()', 'CollapsibleMixin.getCollapsibleClassSet()', link);
      return _CollapsibleMixin2.getCollapsibleClassSet.call(this, className);
    },

    getCollapsibleDOMNode: function getCollapsibleDOMNode() {
      _deprecationWarning('CollapsableMixin.getCollapsableDOMNode()', 'CollapsibleMixin.getCollapsibleDOMNode()', link);
      return this.getCollapsableDOMNode();
    },

    getCollapsibleDimensionValue: function getCollapsibleDimensionValue() {
      _deprecationWarning('CollapsableMixin.getCollapsableDimensionValue()', 'CollapsibleMixin.getCollapsibleDimensionValue()', link);
      return this.getCollapsableDimensionValue();
    },

    componentDidMount: function componentDidMount() {
      _deprecationWarning('CollapsableMixin', 'CollapsibleMixin', link);
    }
  });

  module.exports = CollapsableMixin;
});