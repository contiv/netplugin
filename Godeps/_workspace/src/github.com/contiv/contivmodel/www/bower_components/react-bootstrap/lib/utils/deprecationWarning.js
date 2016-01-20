define(['exports', 'module'], function (exports, module) {
  'use strict';

  module.exports = deprecationWarning;

  function deprecationWarning(oldname, newname, link) {
    if (process.env.NODE_ENV !== 'production') {
      if (!window.console && typeof console.warn !== 'function') {
        return;
      }

      var message = '' + oldname + ' is deprecated. Use ' + newname + ' instead.';
      console.warn(message);

      if (link) {
        console.warn('You can read more about it here ' + link);
      }
    }
  }
});