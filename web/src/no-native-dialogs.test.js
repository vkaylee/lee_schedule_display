import { strict as assert } from 'node:assert';
import { parse as parseSvelte } from 'svelte/compiler';
import { readFileSync, readdirSync } from 'node:fs';
import { extname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// ponytail: checks direct calls with static names; add scope-aware analysis for aliases or dynamic keys.
const forbidden = new Set(['alert', 'confirm', 'prompt']);
const sourceRoot = fileURLToPath(new URL('./', import.meta.url));
const entryHtml = fileURLToPath(new URL('../index.html', import.meta.url));

function propertyName(node) {
  if (node?.type !== 'MemberExpression') return null;
  if (!node.computed && node.property.type === 'Identifier') return node.property.name;
  if (node.computed && ['Literal', 'StringLiteral'].includes(node.property.type)) return node.property.value;
  return null;
}

function findNativeDialogs(node, violations = []) {
  if (!node || typeof node !== 'object') return violations;
  if (node.type === 'Attribute' && /^on/i.test(node.name) && Array.isArray(node.value)) {
    for (const part of node.value) {
      if (part.type === 'Text') findNativeDialogs(acornParse(part.data, { ecmaVersion: 'latest', allowReturnOutsideFunction: true }), violations);
    }
  }
  if (node.type === 'CallExpression') {
    const callee = node.callee;
    const isBare = callee.type === 'Identifier' && forbidden.has(callee.name);
    const isNativeHost = callee.type === 'MemberExpression'
      && ['window', 'globalThis'].includes(callee.object?.name)
      && forbidden.has(propertyName(callee));
    if (isBare || isNativeHost) violations.push(node.start);
  }
  for (const [key, value] of Object.entries(node)) {
    if (key === 'parent' || key === 'loc') continue;
    if (Array.isArray(value)) value.forEach(child => findNativeDialogs(child, violations));
    else if (value && typeof value === 'object') findNativeDialogs(value, violations);
  }
  return violations;
}

function parseSource(source, extension) {
  if (extension !== '.svelte' && extension !== '.html') return { body: acornParse(source, { ecmaVersion: 'latest', sourceType: 'module' }) };
  const component = parseSvelte(source);
  return { body: [component.instance?.content, component.module?.content, component.html] };
}

function filesBelow(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return entry.name === 'node_modules' ? [] : filesBelow(path);
    assert.notEqual(extname(path), '.ts', `${path}: add TypeScript parser to dialog check`);
    return !entry.name.endsWith('.test.js') && ['.js', '.mjs', '.cjs', '.svelte', '.html'].includes(extname(path)) ? [path] : [];
  });
}

const { parse: acornParse } = await import('acorn');

for (const file of [...filesBelow(sourceRoot), entryHtml]) {
  const source = readFileSync(file, 'utf8');
  const { body } = parseSource(source, extname(file));
  assert.deepEqual(findNativeDialogs(body), [], `${file} contains native dialog call`);
}

const invalidFixtures = [
  'alert("x")',
  'confirm("x")',
  'prompt("x")',
  'window.alert("x")',
  'window["confirm"]("x")',
  'globalThis.prompt("x")',
  'globalThis["alert"]("x")',
  '<script>window.confirm("x")</script>',
  '<button onclick={() => prompt("x")}>x</button>',
  '<script type="module">globalThis.alert("x")</script>',
  '<button onclick="window.confirm(\'x\')">x</button>'
];
for (const fixture of invalidFixtures) {
  const { body } = parseSource(fixture, fixture.startsWith('<') ? '.svelte' : '.js');
  assert.notEqual(findNativeDialogs(body).length, 0, `missed forbidden fixture: ${fixture}`);
}

const validFixtures = [
  'const alert = "text"; const message = "window.confirm(\'x\')";',
  'const confirm = () => true; const prompt = 2; const obj = { alert: 1 };',
  'window.alertName(); globalThis.confirmed = true; other.alert(); window[method]();'
];
for (const fixture of validFixtures) {
  const { body } = parseSource(fixture, '.js');
  assert.deepEqual(findNativeDialogs(body), [], `false positive for valid fixture: ${fixture}`);
}
