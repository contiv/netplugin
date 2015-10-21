define(['exports', 'module', 'react', './BootstrapMixin', './CollapsibleMixin', 'classnames', './utils/domUtils', './utils/deprecationWarning', './utils/ValidComponentChildren', './utils/createChainedFunction'], function (exports, module, _react, _BootstrapMixin, _CollapsibleMixin, _classnames, _utilsDomUtils, _utilsDeprecationWarning, _utilsValidComponentChildren, _utilsCreateChainedFunction) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _React = _interopRequire(_react);

  var _BootstrapMixin2 = _interopRequire(_BootstrapMixin);

  var _CollapsibleMixin2 = _interopRequire(_CollapsibleMixin);

  var _classNames = _interopRequire(_classnames);

  var _domUtils = _interopRequire(_utilsDomUtils);

  var _deprecationWarning = _interopRequire(_utilsDeprecationWarning);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var _createChainedFunction = _interopRequire(_utilsCreateChainedFunction);

  var CollapsibleNav = _React.createClass({
    displayName: 'CollapsibleNav',

    mixins: [_BootstrapMixin2, _CollapsibleMixin2],

    propTypes: {
      onSelect: _React.PropTypes.func,
      activeHref: _React.PropTypes.string,
      activeKey: _React.PropTypes.any,
      collapsable: _React.PropTypes.bool,
      expanded: _React.PropTypes.bool,
      eventKey: _React.PropTypes.any
    },

    getCollapsibleDOMNode: function getCollapsibleDOMNode() {
      return this.getDOMNode();
    },

    getCollapsibleDimensionValue: function getCollapsibleDimensionValue() {
      var height = 0;
      var nodes = this.refs;
      for (var key in nodes) {
        if (nodes.hasOwnProperty(key)) {

          var n = nodes[key].getDOMNode(),
              h = n.offsetHeight,
              computedStyles = _domUtils.getComputedStyles(n);

          height += h + parseInt(computedStyles.marginTop, 10) + parseInt(computedStyles.marginBottom, 10);
        }
      }
      return height;
    },

    componentDidMount: function componentDidMount() {
      if (this.constructor.__deprecated__) {
        _deprecationWarning('CollapsableNav', 'CollapsibleNav', 'https://github.com/react-bootstrap/react-bootstrap/issues/425#issuecomment-97110963');
      }
    },

    render: function render() {
      /*
       * this.props.collapsable is set in NavBar when a eventKey is supplied.
       */
      var classes = this.props.collapsable ? this.getCollapsibleClassSet() : {};
      /*
       * prevent duplicating navbar-collapse call if passed as prop.
       * kind of overkill...
       * good cadidate to have check implemented as an util that can
       * also be used elsewhere.
       */
      if (this.props.className === undefined || this.props.className.split(' ').indexOf('navbar-collapse') === -2) {
        classes['navbar-collapse'] = this.props.collapsable;
      }

      return _React.createElement(
        'div',
        { eventKey: this.props.eventKey, className: _classNames(this.props.className, classes) },
        _ValidComponentChildren.map(this.props.children, this.props.collapsable ? this.renderCollapsibleNavChildren : this.renderChildren)
      );
    },

    getChildActiveProp: function getChildActiveProp(child) {
      if (child.props.active) {
        return true;
      }
      if (this.props.activeKey != null) {
        if (child.props.eventKey === this.props.activeKey) {
          return true;
        }
      }
      if (this.props.activeHref != null) {
        if (child.props.href === this.props.activeHref) {
          return true;
        }
      }

      return child.props.active;
    },

    renderChildren: function renderChildren(child, index) {
      var key = child.key ? child.key : index;
      return _react.cloneElement(child, {
        activeKey: this.props.activeKey,
        activeHref: this.props.activeHref,
        ref: 'nocollapse_' + key,
        key: key,
        navItem: true
      });
    },

    renderCollapsibleNavChildren: function renderCollapsibleNavChildren(child, index) {
      var key = child.key ? child.key : index;
      return _react.cloneElement(child, {
        active: this.getChildActiveProp(child),
        activeKey: this.props.activeKey,
        activeHref: this.props.activeHref,
        onSelect: _createChainedFunction(child.props.onSelect, this.props.onSelect),
        ref: 'collapsible_' + key,
        key: key,
        navItem: true
      });
    }
  });

  module.exports = CollapsibleNav;
});