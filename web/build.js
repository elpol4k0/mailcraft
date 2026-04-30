const esbuild = require('esbuild');
const fs = require('fs');
const path = require('path');

const watch = process.argv.includes('--watch');
const outDir = path.join(__dirname, '..', 'ui');

// Ensure output directory exists
if (!fs.existsSync(outDir)) {
  fs.mkdirSync(outDir, { recursive: true });
}

function copyCSS() {
  const src = path.join(__dirname, 'src', 'styles', 'main.css');
  const dst = path.join(outDir, 'app.css');
  fs.copyFileSync(src, dst);
  console.log('CSS copied');
}

function copyHTML() {
  const src = path.join(__dirname, 'index.html');
  const dst = path.join(outDir, 'index.html');
  fs.copyFileSync(src, dst);
  console.log('HTML copied');
}

const buildOptions = {
  entryPoints: [path.join(__dirname, 'src', 'main.ts')],
  bundle: true,
  outfile: path.join(outDir, 'app.js'),
  minify: !watch,
  sourcemap: true,
  target: ['es2020'],
  platform: 'browser',
  logLevel: 'info',
};

if (watch) {
  esbuild.context(buildOptions).then(ctx => {
    copyCSS();
    copyHTML();
    ctx.watch();
    console.log('Watching for changes...');
  });
} else {
  esbuild.build(buildOptions).then(() => {
    copyCSS();
    copyHTML();
    console.log('Build complete');
  }).catch(() => process.exit(1));
}
