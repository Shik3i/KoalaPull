const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const websiteDir = __dirname;
const wwwDir = path.join(websiteDir, 'www');

function read(file) {
  return fs.readFileSync(path.join(wwwDir, file), 'utf8');
}

test('website build emits hardened static output without inline code', () => {
  execFileSync('node', ['build.js'], {
    cwd: websiteDir,
    stdio: 'pipe'
  });

  const html = read('index.html');
  const headers = read('_headers');

  assert.match(headers, /Content-Security-Policy:/);
  assert.match(headers, /X-Frame-Options:/);
  assert.match(headers, /Referrer-Policy:/);
  assert.match(html, /<script[^>]*type="application\/ld\+json"[^>]*>/i);
  assert.doesNotMatch(html, /<script\b(?![^>]*\bsrc=)(?![^>]*type="application\/ld\+json")[^>]*>/i);
  assert.doesNotMatch(html, /\sstyle="/i);
  assert.match(html, /<section id="experience"/);
  assert.match(html, /assets\/NewLogoIcon_384\.webp/);
});
