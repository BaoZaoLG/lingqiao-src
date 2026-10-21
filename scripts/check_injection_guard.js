#!/usr/bin/env node
/*
 * check_injection_guard.js — regression gate for the AutoExam injection wrapper.
 *
 * The wrapper lives as a C++ string template in client/src/hook/page_inject.cpp.
 * This harness extracts that template from the real source, substitutes the two
 * interpolated values and the remote payload, and then drives it three times
 * against a fake window to prove the guard behaves:
 *
 *   1. fresh window            -> payload runs
 *   2. already injected, alive -> skipped (no duplicate init)
 *   3. flag set, AutoExam gone -> payload runs AGAIN
 *
 * Case 3 is the one that matters. A bare per-window guard traps a frame forever
 * if the payload ever fails to finish: the frame ends up with no AutoExam, never
 * reports ready=1, never takes the native trigger, and nothing appears in the
 * log — which is indistinguishable from a dead AI path.
 *
 * No build and no browser required.
 *
 * Usage: node scripts/check_injection_guard.js [path/to/page_inject.cpp]
 */
'use strict';

const fs = require('fs');
const path = require('path');
const vm = require('vm');

const cppPath = process.argv.slice(2).find(a => a.indexOf('-') !== 0) ||
  path.join(__dirname, '..', 'client', 'src', 'hook', 'page_inject.cpp');

/* Prefer the packaged payload (what production actually injects); fall back to
 * the source entry so the gate still works on a tree without build output. */
function payloadPath() {
  const dist = path.join(__dirname, '..', 'packages', 'autoexam', 'dist', 'autoexam.bundle.js');
  if (fs.existsSync(dist)) return dist;
  return path.join(__dirname, '..', 'packages', 'autoexam', 'src', 'autoexam.js');
}

let failures = 0;
function ok(label, cond, extra) {
  if (!cond) failures++;
  console.log((cond ? 'PASS  ' : 'FAIL  ') + label +
    (extra !== undefined ? '   [' + extra + ']' : ''));
}

/* ---- 1. Extract the js template from the real C++ source ---------------- */
const src = fs.readFileSync(cppPath, 'utf8');

const start = src.indexOf('std::string js;');
if (start < 0) throw new Error('js template not found in ' + cppPath);
const endMarker = src.indexOf('int wideLen', start);
if (endMarker < 0) throw new Error('end of js template not found in ' + cppPath);

let block = src.slice(start, endMarker);

/* Drop C++ comments first — the explanatory comment contains quoted words that
 * would otherwise be mistaken for template fragments. */
block = block.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '');

/* Mark the two runtime interpolations and the remote payload. */
block = block
  .replace(/JsEscape\(secrets\.model\.c_str\(\)\)/g, '__LQ_MODEL__')
  .replace(/JsEscape\(secrets\.baseUrl\.c_str\(\)\)/g, '__LQ_BASEURL__')
  .replace(/js\s*\+=\s*remoteScript\s*;/g, 'js += "__LQ_REMOTE__";');

/* Join every string literal of the js += statements, in source order. */
const literals = [...block.matchAll(/"((?:[^"\\]|\\.)*)"/g)].map(m => m[1]);
if (!literals.length) throw new Error('no template fragments found');

const unescape = (s) => s.replace(/\\(.)/g, (_, c) =>
  c === 'n' ? '\n' : c === 't' ? '\t' : c === 'r' ? '\r' : c);

let template = literals.map(unescape).join('');

const REMOTE = fs.readFileSync(payloadPath(), 'utf8');
template = template
  .replace('__LQ_MODEL__', 'test-model')
  .replace('__LQ_BASEURL__', 'https://api.example/v1')
  .replace('__LQ_REMOTE__', REMOTE);

ok('template extracted from source', template.indexOf('__AUTOEXAM_INJECTED__') >= 0,
   template.length + ' chars');
ok('template is valid JavaScript', (() => {
  try { new vm.Script(template); return true; } catch (e) { return false; }
})());

/* ---- 2. A minimal window to run it against ----------------------------- */
function makeEnv() {
  const diagLogs = [];
  const makeEl = (tag) => ({
    tagName: String(tag || 'div').toUpperCase(),
    id: '', className: '', value: '', textContent: '', innerHTML: '',
    isConnected: true, style: {}, children: [], parentNode: null,
    classList: { add() {}, remove() {}, contains: () => false, toggle() {} },
    getAttribute: () => null, setAttribute() {}, removeAttribute() {},
    querySelector: () => null, querySelectorAll: () => [],
    appendChild(c) { this.children.push(c); return c; },
    removeChild() {}, insertBefore() {}, addEventListener() {}, removeEventListener() {},
    dispatchEvent() {}, getBoundingClientRect: () => ({ top: 0, left: 0, width: 0, height: 0 })
  });
  const store = new Map();
  const sandbox = {
    document: {
      readyState: 'complete',
      head: makeEl('head'), body: makeEl('body'), documentElement: makeEl('html'),
      activeElement: null, onselectstart: null,
      createElement: makeEl,
      createEvent: (t) => ({ type: t, initEvent() {} }),
      querySelector: () => null, querySelectorAll: () => [],
      getElementById: () => null, addEventListener() {}, removeEventListener() {}
    },
    sessionStorage: {
      getItem: (k) => (store.has(k) ? store.get(k) : null),
      setItem: (k, v) => store.set(k, String(v)),
      removeItem: (k) => store.delete(k)
    },
    console: { log() {}, debug() {}, info() {}, warn() {}, error() {} },
    setTimeout: () => 0, clearTimeout, setInterval: () => 0, clearInterval() {},
    JSON, Object, Promise, Date, Math, RegExp, String, Array, Boolean, Error,
    isNaN, parseInt, parseFloat
  };
  sandbox.window = sandbox;
  sandbox.globalThis = sandbox;
  sandbox.window.location = { href: 'https://exam.example/paper' };
  sandbox.window.addEventListener = () => {};
  sandbox.window.removeEventListener = () => {};
  sandbox.window.postMessage = () => {};
  sandbox.window.LingQiaoBridge = {
    ready: true,
    request(method, params) {
      if (method === 'diagnostics.log') { diagLogs.push(params && params.message); }
      return Promise.resolve({});
    }
  };
  /* Contextify once. Deleting a global from the sandbox object afterwards does
   * NOT remove it from the context's global, so state changes must be made from
   * inside the context (see run 3). */
  vm.createContext(sandbox);
  return { sandbox, diagLogs };
}

function run(env) {
  vm.runInContext(template, env.sandbox, { filename: 'injected.js' });
}

function lastHookLog(env) {
  return env.diagLogs.filter(l => l && l.indexOf('autoexam-') === 0).slice(-1)[0] || '';
}

/* ---- 3. Three runs ----------------------------------------------------- */
console.log('--- run 1: fresh window ---');
const e1 = makeEnv();
run(e1);
ok('payload ran', typeof e1.sandbox.window.AutoExam === 'object' &&
   typeof e1.sandbox.window.AutoExam.trigger === 'function');
ok('reported autoexam-ran recovered=0',
   lastHookLog(e1).indexOf('autoexam-ran recovered=0') === 0, lastHookLog(e1));

console.log('--- run 2: re-injection while healthy ---');
run(e1);
ok('reported autoexam-skip', lastHookLog(e1).indexOf('autoexam-skip') === 0, lastHookLog(e1));

console.log('--- run 3: flag set but AutoExam gone ---');
/* Simulate the frame losing AutoExam (payload threw before assigning it, or the
 * document was swapped). Must be done inside the context. */
vm.runInContext('delete window.AutoExam;', e1.sandbox);
ok('AutoExam really is gone before run 3',
   vm.runInContext('typeof window.AutoExam', e1.sandbox) === 'undefined');
run(e1);
ok('payload ran again (frame recovered)',
   typeof e1.sandbox.window.AutoExam === 'object' &&
   typeof e1.sandbox.window.AutoExam.trigger === 'function');
ok('reported autoexam-ran recovered=1',
   lastHookLog(e1).indexOf('autoexam-ran recovered=1') === 0, lastHookLog(e1));

console.log(failures === 0
  ? '\nAll injection guard checks passed.'
  : '\n' + failures + ' injection guard check(s) FAILED.');
process.exit(failures === 0 ? 0 : 1);
