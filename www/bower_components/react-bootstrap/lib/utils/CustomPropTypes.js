define(['exports', 'module'], function (exports, module) {
  'use strict';

  var ANONYMOUS = '<<anonymous>>';

  var CustomPropTypes = {
    /**
     * Checks whether a prop provides a DOM element
     *
     * The element can be provided in two forms:
     * - Directly passed
     * - Or passed an object which has a `getDOMNode` method which will return the required DOM element
     *
     * @param props
     * @param propName
     * @param componentName
     * @returns {Error|undefined}
     */
    mountable: createMountableChecker(),
    /**
     * Checks whether a prop matches a key of an associated object
     *
     * @param props
     * @param propName
     * @param componentName
     * @returns {Error|undefined}
     */
    keyOf: createKeyOfChecker
  };

  /**
   * Create chain-able isRequired validator
   *
   * Largely copied directly from:
   *  https://github.com/facebook/react/blob/0.11-stable/src/core/ReactPropTypes.js#L94
   */
  function createChainableTypeChecker(validate) {
    function checkType(isRequired, props, propName, componentName) {
      componentName = componentName || ANONYMOUS;
      if (props[propName] == null) {
        if (isRequired) {
          return new Error('Required prop `' + propName + '` was not specified in ' + '`' + componentName + '`.');
        }
      } else {
        return validate(props, propName, componentName);
      }
    }

    var chainedCheckType = checkType.bind(null, false);
    chainedCheckType.isRequired = checkType.bind(null, true);

    return chainedCheckType;
  }

  function createMountableChecker() {
    function validate(props, propName, componentName) {
      if (typeof props[propName] !== 'object' || typeof props[propName].render !== 'function' && props[propName].nodeType !== 1) {
        return new Error('Invalid prop `' + propName + '` supplied to ' + '`' + componentName + '`, expected a DOM element or an object that has a `render` method');
      }
    }

    return createChainableTypeChecker(validate);
  }

  function createKeyOfChecker(obj) {
    function validate(props, propName, componentName) {
      var propValue = props[propName];
      if (!obj.hasOwnProperty(propValue)) {
        var valuesString = JSON.stringify(Object.keys(obj));
        return new Error('Invalid prop \'' + propName + '\' of value \'' + propValue + '\' ' + ('supplied to \'' + componentName + '\', expected one of ' + valuesString + '.'));
      }
    }
    return createChainableTypeChecker(validate);
  }

  module.exports = CustomPropTypes;
});