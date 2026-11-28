import { strict as assert } from 'node:assert';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { translate } from './i18n.js';

const sourceDirectory = dirname(fileURLToPath(import.meta.url));
const catalog = readFileSync(join(sourceDirectory, 'i18n.js'), 'utf8');
const englishKeys = new Set([...catalog.matchAll(/'([^']+)':/g)].map(([, key]) => key));
for (const key of englishKeys) {
  const value = translate('', key, 'zh-CN');
  assert.ok(typeof value === 'string' && value.trim(), `empty zh-CN translation: ${key}`);
}
assert.equal(translate('你好', 'Hello', 'vi'), '你好');
assert.equal(translate('你好', 'Hello', 'en'), 'Hello');
assert.equal(translate('xin chào', 'Hello', 'unsupported'), 'xin chào');
assert.equal(translate('你好', 'Sign in', 'zh-CN'), '登录');
assert.equal(translate('xin chào', 'Signing in…', 'zh-CN'), '正在登录…');
assert.equal(translate('xin chào', 'Untranslated test probe', 'zh-CN'), 'Untranslated test probe');

for (const file of readdirSync(sourceDirectory).filter(name => name.endsWith('.svelte'))) {
  const source = readFileSync(join(sourceDirectory, file), 'utf8');
  const englishPhrases = [
    ...[...source.matchAll(/(?:\bt|\bsetMessage|\btranslate)\(\s*'((?:\\.|[^'])*)'\s*,\s*'((?:\\.|[^'])*)'\s*[,)]/g)].map(([, , english]) => english),
    ...[...source.matchAll(/\['((?:\\.|[^'])*)',\s*'((?:\\.|[^'])*)'\]/g)].filter(([, first, second]) => first !== 'en' && second !== 'zh-CN').map(([, , english]) => english),
  ];
  for (const english of englishPhrases) {
    assert.ok(englishKeys.has(english), `${file} missing zh-CN translation for: ${english}`);
  }
}
