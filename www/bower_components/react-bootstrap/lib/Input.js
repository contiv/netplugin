define(['exports', 'module', 'react', 'classnames', './Button', './FormGroup'], function (exports, module, _react, _classnames, _Button, _FormGroup) {
  'use strict';

  var _interopRequire = function (obj) { return obj && obj.__esModule ? obj['default'] : obj; };

  var _objectWithoutProperties = function (obj, keys) { var target = {}; for (var i in obj) { if (keys.indexOf(i) >= 0) continue; if (!Object.prototype.hasOwnProperty.call(obj, i)) continue; target[i] = obj[i]; } return target; };

  var _classCallCheck = function (instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError('Cannot call a class as a function'); } };

  var _createClass = (function () { function defineProperties(target, props) { for (var i = 0; i < props.length; i++) { var descriptor = props[i]; descriptor.enumerable = descriptor.enumerable || false; descriptor.configurable = true; if ('value' in descriptor) descriptor.writable = true; Object.defineProperty(target, descriptor.key, descriptor); } } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; })();

  var _inherits = function (subClass, superClass) { if (typeof superClass !== 'function' && superClass !== null) { throw new TypeError('Super expression must either be null or a function, not ' + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) subClass.__proto__ = superClass; };

  var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

  var _React = _interopRequire(_react);

  var _classNames = _interopRequire(_classnames);

  var _Button2 = _interopRequire(_Button);

  var _FormGroup2 = _interopRequire(_FormGroup);

  var Input = (function (_React$Component) {
    function Input() {
      _classCallCheck(this, Input);

      if (_React$Component != null) {
        _React$Component.apply(this, arguments);
      }
    }

    _inherits(Input, _React$Component);

    _createClass(Input, [{
      key: 'getInputDOMNode',
      value: function getInputDOMNode() {
        return _React.findDOMNode(this.refs.input);
      }
    }, {
      key: 'getValue',
      value: function getValue() {
        if (this.props.type === 'static') {
          return this.props.value;
        } else if (this.props.type) {
          if (this.props.type === 'select' && this.props.multiple) {
            return this.getSelectedOptions();
          } else {
            return this.getInputDOMNode().value;
          }
        } else {
          throw 'Cannot use getValue without specifying input type.';
        }
      }
    }, {
      key: 'getChecked',
      value: function getChecked() {
        return this.getInputDOMNode().checked;
      }
    }, {
      key: 'getSelectedOptions',
      value: function getSelectedOptions() {
        var values = [];

        Array.prototype.forEach.call(this.getInputDOMNode().getElementsByTagName('option'), function (option) {
          if (option.selected) {
            var value = option.getAttribute('value') || option.innerHtml;
            values.push(value);
          }
        });

        return values;
      }
    }, {
      key: 'isCheckboxOrRadio',
      value: function isCheckboxOrRadio() {
        return this.props.type === 'checkbox' || this.props.type === 'radio';
      }
    }, {
      key: 'isFile',
      value: function isFile() {
        return this.props.type === 'file';
      }
    }, {
      key: 'renderInputGroup',
      value: function renderInputGroup(children) {
        var addonBefore = this.props.addonBefore ? _React.createElement(
          'span',
          { className: 'input-group-addon', key: 'addonBefore' },
          this.props.addonBefore
        ) : null;

        var addonAfter = this.props.addonAfter ? _React.createElement(
          'span',
          { className: 'input-group-addon', key: 'addonAfter' },
          this.props.addonAfter
        ) : null;

        var buttonBefore = this.props.buttonBefore ? _React.createElement(
          'span',
          { className: 'input-group-btn' },
          this.props.buttonBefore
        ) : null;

        var buttonAfter = this.props.buttonAfter ? _React.createElement(
          'span',
          { className: 'input-group-btn' },
          this.props.buttonAfter
        ) : null;

        var inputGroupClassName = undefined;
        switch (this.props.bsSize) {
          case 'small':
            inputGroupClassName = 'input-group-sm';break;
          case 'large':
            inputGroupClassName = 'input-group-lg';break;
        }

        return addonBefore || addonAfter || buttonBefore || buttonAfter ? _React.createElement(
          'div',
          { className: _classNames(inputGroupClassName, 'input-group'), key: 'input-group' },
          addonBefore,
          buttonBefore,
          children,
          addonAfter,
          buttonAfter
        ) : children;
      }
    }, {
      key: 'renderIcon',
      value: function renderIcon() {
        var classes = {
          glyphicon: true,
          'form-control-feedback': true,
          'glyphicon-ok': this.props.bsStyle === 'success',
          'glyphicon-warning-sign': this.props.bsStyle === 'warning',
          'glyphicon-remove': this.props.bsStyle === 'error'
        };

        return this.props.hasFeedback ? _React.createElement('span', { className: _classNames(classes), key: 'icon' }) : null;
      }
    }, {
      key: 'renderHelp',
      value: function renderHelp() {
        return this.props.help ? _React.createElement(
          'span',
          { className: 'help-block', key: 'help' },
          this.props.help
        ) : null;
      }
    }, {
      key: 'renderCheckboxAndRadioWrapper',
      value: function renderCheckboxAndRadioWrapper(children) {
        var classes = {
          checkbox: this.props.type === 'checkbox',
          radio: this.props.type === 'radio'
        };

        return _React.createElement(
          'div',
          { className: _classNames(classes), key: 'checkboxRadioWrapper' },
          children
        );
      }
    }, {
      key: 'renderWrapper',
      value: function renderWrapper(children) {
        return this.props.wrapperClassName ? _React.createElement(
          'div',
          { className: this.props.wrapperClassName, key: 'wrapper' },
          children
        ) : children;
      }
    }, {
      key: 'renderLabel',
      value: function renderLabel(children) {
        var classes = {
          'control-label': !this.isCheckboxOrRadio()
        };
        classes[this.props.labelClassName] = this.props.labelClassName;

        return this.props.label ? _React.createElement(
          'label',
          { htmlFor: this.props.id, className: _classNames(classes), key: 'label' },
          children,
          this.props.label
        ) : children;
      }
    }, {
      key: 'renderInput',
      value: function renderInput() {
        if (!this.props.type) {
          return this.props.children;
        }

        switch (this.props.type) {
          case 'select':
            return _React.createElement(
              'select',
              _extends({}, this.props, { className: _classNames(this.props.className, 'form-control'), ref: 'input', key: 'input' }),
              this.props.children
            );
          case 'textarea':
            return _React.createElement('textarea', _extends({}, this.props, { className: _classNames(this.props.className, 'form-control'), ref: 'input', key: 'input' }));
          case 'static':
            return _React.createElement(
              'p',
              _extends({}, this.props, { className: _classNames(this.props.className, 'form-control-static'), ref: 'input', key: 'input' }),
              this.props.value
            );
          case 'submit':
            return _React.createElement(_Button2, _extends({}, this.props, { componentClass: 'input', ref: 'input', key: 'input' }));
        }

        var className = this.isCheckboxOrRadio() || this.isFile() ? '' : 'form-control';
        return _React.createElement('input', _extends({}, this.props, { className: _classNames(this.props.className, className), ref: 'input', key: 'input' }));
      }
    }, {
      key: 'renderFormGroup',
      value: function renderFormGroup(children) {
        if (this.props.type === 'submit') {
          var _props = this.props;
          var bsStyle = _props.bsStyle;

          var other = _objectWithoutProperties(_props, ['bsStyle']);

          /* eslint no-unused-vars: 0 */
          return _React.createElement(
            _FormGroup2,
            other,
            children
          );
        }

        return _React.createElement(
          _FormGroup2,
          this.props,
          children
        );
      }
    }, {
      key: 'renderChildren',
      value: function renderChildren() {
        return !this.isCheckboxOrRadio() ? [this.renderLabel(), this.renderWrapper([this.renderInputGroup(this.renderInput()), this.renderIcon(), this.renderHelp()])] : this.renderWrapper([this.renderCheckboxAndRadioWrapper(this.renderLabel(this.renderInput())), this.renderHelp()]);
      }
    }, {
      key: 'render',
      value: function render() {
        var children = this.renderChildren();
        return this.renderFormGroup(children);
      }
    }]);

    return Input;
  })(_React.Component);

  Input.propTypes = {
    type: _React.PropTypes.string,
    label: _React.PropTypes.node,
    help: _React.PropTypes.node,
    addonBefore: _React.PropTypes.node,
    addonAfter: _React.PropTypes.node,
    buttonBefore: _React.PropTypes.node,
    buttonAfter: _React.PropTypes.node,
    bsSize: _React.PropTypes.oneOf(['small', 'medium', 'large']),
    bsStyle: function bsStyle(props) {
      if (props.type === 'submit') {
        return null;
      }
      return _React.PropTypes.oneOf(['success', 'warning', 'error']).apply(null, arguments);
    },
    hasFeedback: _React.PropTypes.bool,
    id: _React.PropTypes.string,
    groupClassName: _React.PropTypes.string,
    wrapperClassName: _React.PropTypes.string,
    labelClassName: _React.PropTypes.string,
    multiple: _React.PropTypes.bool,
    disabled: _React.PropTypes.bool,
    value: _React.PropTypes.any
  };

  module.exports = Input;
});