#!/usr/bin/env node
'use strict';

var fs = require('fs');
var path = require('path');

var localesDir = path.join(__dirname, 'locales');
var files = fs.readdirSync(localesDir).filter(function (f) {
  return f.endsWith('.json');
});

if (files.length < 2) {
  console.error('Need at least 2 locale files to compare.');
  process.exit(1);
}

var locales = {};
files.forEach(function (file) {
  var content = fs.readFileSync(path.join(localesDir, file), 'utf8');
  var keys = Object.keys(JSON.parse(content))
    .filter(function (k) { return !k.startsWith('SELECTED_'); })
    .sort();
  locales[file] = keys;
});

var referenceFile = files[0];
var referenceKeys = locales[referenceFile];
var hasError = false;

files.slice(1).forEach(function (file) {
  var keys = locales[file];
  var missing = referenceKeys.filter(function (k) { return keys.indexOf(k) === -1; });
  var extra = keys.filter(function (k) { return referenceKeys.indexOf(k) === -1; });

  if (missing.length || extra.length) {
    hasError = true;
    console.error('\nMismatch between ' + referenceFile + ' and ' + file + ':');
    if (missing.length) {
      console.error('  Missing in ' + file + ': ' + missing.join(', '));
    }
    if (extra.length) {
      console.error('  Extra in ' + file + ': ' + extra.join(', '));
    }
  }
});

if (!hasError) {
  console.log('All ' + files.length + ' locale files have matching keys (' + referenceKeys.length + ' keys each).');
  process.exit(0);
} else {
  process.exit(1);
}
