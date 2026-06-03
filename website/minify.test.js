const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const { minifyCss, minifyJs } = require('./lib/minify');

const fixturesDir = path.join(__dirname, 'test-fixtures', 'minify');

function readFixture(kind, name) {
  return fs.readFileSync(path.join(fixturesDir, kind, name), 'utf8');
}

test('css minifier preserves strings and data urls while shrinking layout whitespace', async () => {
  const input = readFixture('css', 'edge-case.css');
  const output = await minifyCss(input);

  assert.match(output, /\.banner:before\{content:"\/\* keep this string \*\/"/);
  assert.match(output, /data:image\/svg\+xml;base64,AAA\\ BBB==/);
  assert.match(output, /margin:calc\(100% - 2rem\)/);
  assert.doesNotMatch(output, /\/\* remove this comment \*\//);
});

test('js minifier preserves behavior for modern syntax fixture', async () => {
  const input = readFixture('js', 'edge-case.js');
  const output = await minifyJs(input);

  assert.match(output, /optional chaining ok/);

  const factory = new Function(`${output}; return demoResult;`);
  const result = factory();

  assert.deepEqual(result, {
    label: 'optional chaining ok',
    items: ['A', 'B', 'fallback']
  });
});
