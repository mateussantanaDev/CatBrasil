import { readFileSync, readdirSync } from 'node:fs';
import { resolve, join } from 'node:path';

const directories = process.argv.slice(2);
if (directories.length === 0) {
  console.error('Uso: node scripts/check-bundle-urls.mjs apps/client/dist [apps/admin/dist]');
  process.exit(2);
}

const forbidden = /\b(?:https?|wss?):\/\/(?:localhost|127(?:\.\d{1,3}){3}|\[::1\])(?=[:/?#]|$)|VITE_API_BASE_URL|VITE_API_URL|VITE_WS_URL/i;
let failed = false;

for (const directory of directories) {
  const root = resolve(directory);
  let jsFiles = 0;
  let hasRelativeApi = false;

  function inspect(path) {
    for (const entry of readdirSync(path, { withFileTypes: true })) {
      const file = join(path, entry.name);
      if (entry.isDirectory()) {
        inspect(file);
      } else if (entry.isFile() && /\.(?:js|css|html)$/.test(entry.name)) {
        const content = readFileSync(file, 'utf8');
        if (entry.name.endsWith('.js')) jsFiles++;
        if (content.includes('/api/v1')) hasRelativeApi = true;
        const match = content.match(forbidden);
        if (match) {
          console.error(`${file}: URL ou variável proibida no bundle: ${match[0]}`);
          failed = true;
        }
      }
    }
  }

  inspect(root);
  if (jsFiles === 0 || !hasRelativeApi) {
    console.error(`${root}: bundle JS ou rota relativa /api/v1 ausente`);
    failed = true;
  }
}

if (failed) process.exit(1);
console.log(`Bundles verificados: ${directories.join(', ')}`);
