const esbuild = require('esbuild');

async function minifyCss(code) {
  const result = await esbuild.transform(code, {
    loader: 'css',
    minify: true,
    target: 'es2020'
  });

  return result.code.trim();
}

async function minifyJs(code) {
  const result = await esbuild.transform(code, {
    loader: 'js',
    minify: true,
    target: 'es2020'
  });

  return result.code.trim();
}

module.exports = {
  minifyCss,
  minifyJs
};
