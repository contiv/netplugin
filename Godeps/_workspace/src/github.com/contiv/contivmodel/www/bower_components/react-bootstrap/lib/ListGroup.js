define(['exports', 'module', 'react', 'classnames', './utils/ValidComponentChildren'], function (exports, module, _react, _classnames, _utilsValidComponentChildren) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _classCallCheck = function (instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } };

  var _createClass = (function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ('value' in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; })();

  var _inherits = function (subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) subClass.__proto__ = superClass; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _ValidComponentChildren = _interopRequire(_utilsValidComponentChildren);

  var ListGroup = (function (_React$Component) {
    function ListGroup() {
      _classCallCheck(this, ListGroup);

      if (_React$Component != null) {
        _React$Component.apply(this, arguments);
      }
    }

    _inherits(ListGroup, _React$Component);

    _createClass(ListGroup, [{
      key: 'render',
      value: function render() {
        var _this = this;

        var items = _ValidComponentChildren.map(this.props.children, function (item, index) {
          return _react.cloneElement(item, { key: item.key ? item.key : index });
        });

        var childrenAnchors = false;

        if (!this.props.children) {
          return this.renderDiv(items);
        } else if (_React.Children.count(this.props.children) === 1 && !Array.isArray(this.props.children)) {
          var child = this.props.children;

          childrenAnchors = this.isAnchor(child.props);
        } else {

          childrenAnchors = Array.prototype.some.call(this.props.children, function (child) {
            return !Array.isArray(child) ? _this.isAnchor(child.props) : Array.prototype.some.call(child, function (subChild) {
              return _this.isAnchor(subChild.props);
            });
          });
        }

        if (childrenAnchors) {
          return this.renderDiv(items);
        } else {
          return this.renderUL(items);
        }
      }
    }, {
      key: 'isAnchor',
      value: function isAnchor(props) {
        return props.href || props.onClick;
      }
    }, {
      key: 'renderUL',
      value: function renderUL(items) {
        var listItems = _ValidComponentChildren.map(items, function (item, index) {
          return _react.cloneElement(item, { listItem: true });
        });

        return _React.createElement(
          'ul',
          _extends({}, this.props, {
            className: _classNames(this.props.className, 'list-group') }),
          listItems
        );
      }
    }, {
      key: 'renderDiv',
      value: function renderDiv(items) {
        return _React.createElement(
          'div',
          _extends({}, this.props, {
            className: _classNames(this.props.className, 'list-group') }),
          items
        );
      }
    }]);

    return ListGroup;
  })(_React.Component);

  ListGroup.propTypes = {
    className: _React.PropTypes.string,
    id: _React.PropTypes.string
  };

  module.exports = ListGroup;
});