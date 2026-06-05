const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const sharp = require('sharp');
const { minifyCss, minifyJs } = require('./lib/minify');

const websiteDir = __dirname;
const rootDir = path.resolve(websiteDir, '..');
const wwwDir = path.join(websiteDir, 'www');
const assetsOutDir = path.join(wwwDir, 'assets');
const sourceIcon = path.join(rootDir, 'assets', 'Icon.png');
const sourceFontsDir = path.join(rootDir, 'frontend', 'public', 'fonts');
const placeholderDomain = 'https://pull.koalastuff.net';
const repoUrl = 'https://github.com/Shik3i/KoalaPull';
const languages = ['en', 'de', 'fr'];
const legalPages = [
  { template: 'impressum.html', output: { en: 'imprint.html', de: 'impressum.html' } },
  { template: 'datenschutz.html', output: { en: 'privacy.html', de: 'datenschutz.html' } }
];

const log = (msg) => console.log(`[build] ${msg}`);

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function hash8(input) {
  return crypto.createHash('sha256').update(input).digest('hex').slice(0, 8);
}

function readText(filePath) {
  return fs.readFileSync(filePath, 'utf8');
}

function flattenObject(obj, prefix = '', out = {}) {
  if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return out;
  for (const [key, value] of Object.entries(obj)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      flattenObject(value, nextKey, out);
    } else {
      out[nextKey] = String(value);
    }
  }
  return out;
}

function assertLocaleKeyParity(flatLocales) {
  const entries = Object.entries(flatLocales);
  if (!entries.length) return;

  const [baseLang, baseLocale] = entries[0];
  const baseKeys = Object.keys(baseLocale).sort();

  for (const [lang, locale] of entries.slice(1)) {
    const keys = Object.keys(locale).sort();
    const missing = baseKeys.filter((key) => !keys.includes(key));
    const extra = [];

    if (missing.length || extra.length) {
      throw new Error(
        [
          `Locale key mismatch: ${lang} vs ${baseLang}`,
          missing.length ? `missing: ${missing.join(', ')}` : '',
          extra.length ? `extra: ${extra.join(', ')}` : ''
        ].filter(Boolean).join(' | ')
      );
    }
  }
}

function escapeRegExp(input) {
  return input.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function replaceAllPlaceholders(html, dictionary) {
  let out = html;
  for (const [key, value] of Object.entries(dictionary)) {
    out = out.replace(new RegExp(`\\{\\{${escapeRegExp(key)}\\}\\}`, 'g'), value);
  }
  return out;
}

function injectDefaults(html, localeCode, assetPrefix) {
  const replacements = {
    ASSET_PATH: assetPrefix,
    BASE_URL: placeholderDomain,
    GITHUB_URL: repoUrl,
    HOME_URL: './',
    CANONICAL_URL: localeCode === 'en' ? `${placeholderDomain}/` : `${placeholderDomain}/${localeCode}/`,
    CANONICAL_PATH: localeCode === 'en' ? '' : `${localeCode}/`,
    LOCALE_CODE: localeCode,
    LOCALE_HREFLANG: localeCode.toLowerCase()
  };

  let out = html;
  for (const [key, value] of Object.entries(replacements)) {
    out = out.replace(new RegExp(`\\{\\{${key}\\}\\}`, 'g'), value);
  }
  return out;
}

function rewriteAssetNames(html, styleName, appName, langName) {
  return html
    .replace(/style\.min\.css/g, styleName)
    .replace(/app\.min\.js/g, appName)
    .replace(/lang-init\.min\.js/g, langName);
}

function injectAvifPictures(html) {
  return html.replace(/<img\b([^>]*)>/gi, (match, attrs) => {
    const srcMatch = attrs.match(/\bsrc="([^"]+)"/i);
    if (!srcMatch) return match;
    const src = srcMatch[1];
    if (!/\.(webp|png|jpg|jpeg)$/i.test(src)) return match;
    const avifSrc = src.replace(/\.(webp|png|jpg|jpeg)$/i, '.avif');
    const srcsetMatch = attrs.match(/\bsrcset="([^"]+)"/i);
    const pictureSource = srcsetMatch
      ? `<source srcset="${srcsetMatch[1].replace(/\.(webp|png|jpg|jpeg)/gi, '.avif')}" type="image/avif">`
      : `<source srcset="${avifSrc}" type="image/avif">`;
    return `<picture>${pictureSource}<img${attrs}></picture>`;
  });
}

function copyFileIfExists(src, dest) {
  if (!fs.existsSync(src)) return false;
  ensureDir(path.dirname(dest));
  fs.copyFileSync(src, dest);
  return true;
}

function copyDirContents(srcDir, destDir) {
  if (!fs.existsSync(srcDir)) return;
  ensureDir(destDir);
  for (const entry of fs.readdirSync(srcDir, { withFileTypes: true })) {
    const srcPath = path.join(srcDir, entry.name);
    const destPath = path.join(destDir, entry.name);
    if (entry.isDirectory()) {
      copyDirContents(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}

async function writeImageSet(baseName, width, options = {}) {
  const resizeOptions = {
    fit: options.fit || 'contain',
    background: options.background || { r: 0, g: 0, b: 0, alpha: 0 }
  };
  const pipeline = sharp(sourceIcon).resize({ width, ...resizeOptions });
  const webpPath = path.join(assetsOutDir, `${baseName}.webp`);
  const avifPath = path.join(assetsOutDir, `${baseName}.avif`);
  await Promise.all([
    pipeline.clone().webp({ quality: 90 }).toFile(webpPath),
    pipeline.clone().avif({ quality: 80, speed: 4 }).toFile(avifPath)
  ]);
}

async function buildImages() {
  if (!fs.existsSync(sourceIcon)) {
    throw new Error(`Missing source icon: ${sourceIcon}`);
  }

  ensureDir(assetsOutDir);

  log('Generating icon images...');
  const imageJobs = [];

  for (const spec of [
    { name: 'NewLogoIcon', width: 200 },
    { name: 'NewLogoIcon_384', width: 384 },
    { name: 'NewLogoIcon_768', width: 768 },
    { name: 'NewLogoIcon_128', width: 128 },
    { name: 'NewLogoIcon_64', width: 64 },
    { name: 'IconHero-1x', width: 180 },
    { name: 'IconHero', width: 360 }
  ]) {
    imageJobs.push(writeImageSet(spec.name, spec.width));
  }

  imageJobs.push(
    sharp(sourceIcon).resize({ width: 16, height: 16, fit: 'cover' }).png().toFile(path.join(assetsOutDir, 'favicon-16x16.png')),
    sharp(sourceIcon).resize({ width: 32, height: 32, fit: 'cover' }).png().toFile(path.join(assetsOutDir, 'favicon-32x32.png')),
    sharp(sourceIcon).resize({ width: 192, height: 192, fit: 'cover' }).png().toFile(path.join(assetsOutDir, 'icon-192x192.png')),
    sharp(sourceIcon).resize({ width: 180, height: 180, fit: 'cover' }).png().toFile(path.join(assetsOutDir, 'apple-touch-icon.png'))
  );

  await Promise.all(imageJobs);

  if (fs.existsSync(sourceFontsDir)) {
    log('Copying fonts...');
    copyDirContents(sourceFontsDir, path.join(wwwDir, 'fonts'));
  }
}

async function build() {
  ensureDir(wwwDir);
  ensureDir(assetsOutDir);

  log('Checking source files...');
  const requiredSources = ['template.html', 'impressum.html', 'datenschutz.html', 'style.css', 'app.js', 'lang-init.js'];
  for (const file of requiredSources) {
    const filePath = path.join(websiteDir, file);
    if (!fs.existsSync(filePath)) {
      throw new Error(`Missing website source file: ${filePath}`);
    }
  }

  log('Reading template and source assets...');
  const template = readText(path.join(websiteDir, 'template.html'));
  const legalTemplates = Object.fromEntries(
    legalPages.map((page) => [page.template, readText(path.join(websiteDir, page.template))])
  );
  const styleRaw = readText(path.join(websiteDir, 'style.css'));
  const appRaw = readText(path.join(websiteDir, 'app.js'));
  const langRaw = readText(path.join(websiteDir, 'lang-init.js'));

  log('Minifying CSS...');
  const styleMin = await minifyCss(styleRaw);
  log('Minifying JS...');
  const [appMin, langMin] = await Promise.all([
    minifyJs(appRaw),
    minifyJs(langRaw)
  ]);

  const styleName = `style.${hash8(styleMin)}.min.css`;
  const appName = `app.${hash8(appMin)}.min.js`;
  const langName = `lang-init.${hash8(langMin)}.min.js`;

  log('Cleaning previous build artifacts...');
  const hashCleanup = /^(style|app|lang-init)\.[a-f0-9]+\.min\.(css|js)$/;
  for (const file of fs.readdirSync(wwwDir)) {
    if (hashCleanup.test(file)) {
      fs.rmSync(path.join(wwwDir, file), { force: true });
    }
  }
  log(`Writing ${styleName}, ${appName}, ${langName}`);
  fs.writeFileSync(path.join(wwwDir, styleName), styleMin);
  fs.writeFileSync(path.join(wwwDir, appName), appMin);
  fs.writeFileSync(path.join(wwwDir, langName), langMin);

  const versionPath = path.join(websiteDir, 'version.json');
  const versionInfo = fs.existsSync(versionPath) ? JSON.parse(readText(versionPath)) : { version: '0.0.0', date: '2026-06-03T00:00:00Z' };
  const localeDir = path.join(websiteDir, 'locales');
  const localeFiles = languages.filter((lang) => fs.existsSync(path.join(localeDir, `${lang}.json`)));
  if (localeFiles.length === 0) {
    throw new Error(`No locale files found in ${localeDir}`);
  }

  const flatLocales = {};
  const renderLocales = {};
  const baseLocale = flattenObject(JSON.parse(readText(path.join(localeDir, 'en.json'))));
  for (const lang of localeFiles) {
    const locale = flattenObject(JSON.parse(readText(path.join(localeDir, `${lang}.json`))));
    flatLocales[lang] = locale;
    renderLocales[lang] = {
      ...baseLocale,
      ...locale
    };
  }
  assertLocaleKeyParity(flatLocales);

  log('Generating locale pages...');
  for (const lang of localeFiles) {
    const locale = renderLocales[lang];
    const assetPrefix = lang === 'en' ? '' : '../';
    const outputDir = lang === 'en' ? wwwDir : path.join(wwwDir, lang);
    ensureDir(outputDir);

    const faqPairs = [];
    for (let i = 1; i <= 6; i++) {
      const q = locale[`FAQ_Q${i}`];
      const a = locale[`FAQ_A${i}`];
      if (q && a) {
        faqPairs.push({
          '@type': 'Question',
          name: q,
          acceptedAnswer: { '@type': 'Answer', text: a }
        });
      }
    }
    const faqJsonld = JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: faqPairs
    }, null, 0).replace(/[\n\r]+/g, '');

    const dictionary = {
      ...locale,
      SELECTED_EN: lang === 'en' ? 'selected' : '',
      SELECTED_DE: lang === 'de' ? 'selected' : '',
      SELECTED_FR: lang === 'fr' ? 'selected' : '',
      VERSION: versionInfo.version,
      BUILD_DATE: versionInfo.date || '',
      FAQ_JSONLD: faqJsonld
    };

    for (const page of legalPages) {
      const isImprint = page.template === 'impressum.html';
      const key = isImprint ? 'IMPRINT_LINK' : 'PRIVACY_LINK';
      if (page.output[lang]) {
        dictionary[key] = page.output[lang].replace(/\.html$/, '');
      } else {
        dictionary[key] = `${assetPrefix}${page.output['en'].replace(/\.html$/, '')}`;
      }
    }

    let html = template;
    html = injectDefaults(html, lang, assetPrefix);
    html = replaceAllPlaceholders(html, dictionary);
    html = rewriteAssetNames(html, styleName, appName, langName);
    html = injectAvifPictures(html);

    const unresolved = html.match(/\{\{[A-Z0-9_.-]+\}\}/g);
    if (unresolved) {
      throw new Error(`Unresolved placeholders in ${lang}: ${unresolved.join(', ')}`);
    }

    fs.writeFileSync(path.join(outputDir, 'index.html'), html);
    log(`  ${lang}/index.html`);

    for (const page of legalPages) {
      const outputName = page.output[lang];
      if (!outputName) continue;
      let legalHtml = legalTemplates[page.template];
      legalHtml = injectDefaults(legalHtml, lang, assetPrefix);
      legalHtml = replaceAllPlaceholders(legalHtml, dictionary);
      legalHtml = rewriteAssetNames(legalHtml, styleName, appName, langName);
      legalHtml = injectAvifPictures(legalHtml);

      const legalUnresolved = legalHtml.match(/\{\{[A-Z0-9_.-]+\}\}/g);
      if (legalUnresolved) {
        throw new Error(`Unresolved placeholders in ${lang}/${outputName}: ${legalUnresolved.join(', ')}`);
      }

      fs.writeFileSync(path.join(outputDir, outputName), legalHtml);
      log(`  ${lang}/${outputName}`);
    }
  }

  const staticFiles = ['robots.txt', 'site.webmanifest', 'sitemap.xml', 'version.json', '_headers'];
  for (const file of staticFiles) {
    const ok = copyFileIfExists(path.join(websiteDir, file), path.join(wwwDir, file));
    if (ok) log(`  copied ${file}`);
  }

  log('Building images...');
  await buildImages();
  log('Images done');

  const readmeOk = copyFileIfExists(path.join(websiteDir, 'README.md'), path.join(wwwDir, 'README.md'));
  if (readmeOk) log('  copied README.md');

  log('Build complete.');
}

build().catch((error) => {
  console.error(error.message || error);
  process.exit(1);
});
