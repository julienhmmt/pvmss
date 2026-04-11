import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { join, dirname } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const enPath = join(__dirname, '../src/lib/i18n/en.json');
const frPath = join(__dirname, '../src/lib/i18n/fr.json');

function flattenKeys(obj, prefix = '') {
  return Object.entries(obj).flatMap(([key, value]) => {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    return typeof value === 'object' && value !== null && !Array.isArray(value)
      ? flattenKeys(value, fullKey)
      : [fullKey];
  });
}

const enKeys = new Set(flattenKeys(JSON.parse(readFileSync(enPath, 'utf8'))));
const frKeys = new Set(flattenKeys(JSON.parse(readFileSync(frPath, 'utf8'))));

const missingInFr = [...enKeys].filter(k => !frKeys.has(k));
const missingInEn = [...frKeys].filter(k => !enKeys.has(k));

if (missingInFr.length > 0) {
  console.error('Keys in EN but missing in FR:');
  missingInFr.forEach(k => console.error('  -', k));
}
if (missingInEn.length > 0) {
  console.error('Keys in FR but missing in EN:');
  missingInEn.forEach(k => console.error('  -', k));
}
if (missingInFr.length === 0 && missingInEn.length === 0) {
  console.log(`✓ Parity check passed: ${enKeys.size} keys match in both files.`);
  process.exit(0);
} else {
  process.exit(1);
}
