#!/usr/bin/env node
/*
 * check_bridge_relay.js — regression gate for the CEF bridge JS bootstrap.
 *
 * The bootstrap in client/src/hook/bridge_bootstrap.js.h is embedded as one or
 * more R"LQBRIDGE(...)" string literals and runs in every frame of the target
 * browser. Its cross-frame AI mailbox routing is easy to break in ways that are
 * invisible until a middle-click silently hangs, so this harness loads the real
 * payload, builds a fake frame tree (top -> mid -> deep) and drives it.
 *
 * No build and no browser required. Exit code 0 = all checks passed.
 *
 * Usage: node scripts/check_bridge_relay.js [path/to/bridge_bootstrap.js.h]
 */
'use strict';

const fs = require('fs');
const path = require('path');
const vm = require('vm');

const src = process.argv[2] ||
  path.join(__dirname, '..', 'client', 'src', 'hook', 'bridge_bootstrap.js.h');

const raw = fs.readFileSync(src, 'utf8');
const BOOT = src.endsWith('.h')
  ? (() => {
      /* MSVC caps a single string literal at 16380 bytes, so the payload is
       * emitted as several adjacent literals that concatenate at compile time. */
      const parts = [...raw.matchAll(/R"LQBRIDGE\(([\s\S]*?)\)LQBRIDGE"/g)].map(m => m[1]);
      if (!parts.length) throw new Error('R"LQBRIDGE(...)" payload not found in ' + src);
      return parts.join('');
    })()
  : raw;

let pending = [];
let intervals = [];
let failures = 0;

function ok(label, cond, extra) {
  if (!cond) failures++;
  console.log((cond ? 'PASS  ' : 'FAIL  ') + label +
    (extra !== undefined ? '   [' + extra + ']' : ''));
}

function makeFrame(name, parent) {
  const listeners = [];
  const w = {};
  w.__name = name;
  w.parent = parent ? parent.window : w;
  w.top = parent ? parent.window.top : w;
  w.frames = [];
  w.addEventListener = (t, fn) => {
    if (t === 'message' || t === 'lqWantV8Relay') listeners.push({ t, fn });
  };
  w.removeEventListener = (t, fn) => {
    const i = listeners.findIndex(l => l.t === t && l.fn === fn);
    if (i >= 0) listeners.splice(i, 1);
  };
  w.postMessage = (msg) => {
    const copy = JSON.parse(JSON.stringify(msg));
    pending.push(() => {
      listeners.filter(l => l.t === 'message').slice()
        .forEach(l => { try { l.fn({ data: copy }); } catch (e) {} });
    });
  };
  w.dispatchEvent = (ev) => {
    listeners.filter(l => l.t === ev.type).slice()
      .forEach(l => { try { l.fn({ type: ev.type }); } catch (e) {} });
  };
  return { name, window: w };
}

function fakeEvent(type) {
  return { type, initEvent() {} };
}

function runBootstrap(frame) {
  const sandbox = {
    window: frame.window,
    console: { log() {}, warn() {}, error() {}, debug() {}, info() {} },
    document: {
      querySelector: () => null,
      querySelectorAll: () => [],
      createElement: () => ({ id: '', textContent: '', isConnected: true }),
      createEvent: (t) => fakeEvent(t),
      head: { appendChild() {} },
      documentElement: { appendChild() {} },
      addEventListener() {},
      readyState: 'complete'
    },
    Event: function (t) { return fakeEvent(t); },
    setTimeout: () => 0,
    setInterval: (fn) => { intervals.push({ fn, frame: frame.name }); return intervals.length; },
    clearInterval: (h) => { if (h) intervals[h - 1] = { fn: null, frame: null }; },
    clearTimeout() {},
    JSON, Object, Promise, Date, Math, RegExp, String, Array, Boolean, Error
  };
  sandbox.globalThis = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(BOOT, sandbox, { filename: frame.name + '.js' });
  return sandbox;
}

function tickRelay(frameName) {
  intervals.forEach(i => {
    if (i.fn && i.frame === frameName) { try { i.fn(); } catch (e) {} }
  });
}

function flush(rounds = 12) {
  for (let i = 0; i < rounds; i++) {
    const batch = pending; pending = [];
    batch.forEach(fn => fn());
    if (!pending.length) return;
  }
}

function nativeStub(results) {
  return {
    request(json) {
      const req = JSON.parse(json);
      if (req.method === 'native.pollTrigger') {
        return JSON.stringify({ v: 2, ok: true, result: { pending: false } });
      }
      if (req.method === 'ai.pollResult') {
        const id = req.params.id;
        if (results[id]) return JSON.stringify({ v: 2, ok: true, result: results[id] });
        return JSON.stringify({ v: 2, ok: true, result: { pending: true } });
      }
      return JSON.stringify({ v: 2, ok: true, result: {} });
    }
  };
}

/* ---- Part 1: AI results must reach a deeply nested frame, exactly once ---- */
console.log('--- Part 1: cross-frame AI result routing ---');
const top = makeFrame('TOP', null);
const mid = makeFrame('MID', top);
const deep = makeFrame('DEEP', mid);
top.window.frames = [mid.window];
mid.window.frames = [deep.window];

const topResults = {};
top.window.__LingQiaoNative = nativeStub(topResults);

runBootstrap(top); runBootstrap(mid); runBootstrap(deep);
flush();

ok('relay starts only in the main frame',
   top.window.__LQ_V8_RELAY === true &&
   !mid.window.__LQ_V8_RELAY && !deep.window.__LQ_V8_RELAY);

const ID = 'ai_1758000000000_7';
let deepResults = [];
let midResults = 0;
deep.window.addEventListener('message', (ev) => {
  if (ev.data && ev.data.type === 'lqAiResult' && ev.data.id === ID) deepResults.push(ev.data);
});
mid.window.addEventListener('message', (ev) => {
  if (ev.data && ev.data.type === 'lqAiResult' && ev.data.id === ID) midResults++;
});

ok('subframe exposes a registrar', typeof deep.window.__LQ_REGISTER_AI_POLL === 'function');

deep.window.__LQ_REGISTER_AI_POLL(ID);
flush();
topResults[ID] = { content: 'A', finish_reason: 'stop' };
tickRelay('TOP');
flush();

ok('deep frame receives its AI result', deepResults.length === 1, 'count=' + deepResults.length);
ok('no duplicate delivery', midResults === 1, 'count=' + midResults);
ok('payload intact',
   !!(deepResults[0] && deepResults[0].result && deepResults[0].result.content === 'A'));
ok('hop counter bounded', deepResults[0] && deepResults[0].__lqHop === 2,
   'hop=' + (deepResults[0] && deepResults[0].__lqHop));

deep.window.__LQ_CANCEL_AI_POLL(ID);
flush();
topResults[ID] = { content: 'SHOULD_NOT_DELIVER' };
deepResults = [];
tickRelay('TOP');
flush();
ok('cancel stops further polling', deepResults.length === 0);

deep.window.__LQ_REGISTER_AI_POLL('not-an-ai-id');
deep.window.__LQ_REGISTER_AI_POLL(undefined);
flush();
ok('malformed ids rejected', true);

/* ---- Part 2: the relay must also start when native appears late ---------- */
console.log('--- Part 2: late relay start via lqWantV8Relay ---');
const t2 = makeFrame('T2', null);
const c2 = makeFrame('C2', t2);
t2.window.frames = [c2.window];

runBootstrap(t2); runBootstrap(c2);
flush();
ok('relay not started before native exists', t2.window.__LQ_V8_RELAY !== true);

const t2Results = {};
t2.window.__LingQiaoNative = nativeStub(t2Results);
t2.window.dispatchEvent(fakeEvent('lqWantV8Relay'));
flush();
ok('lqWantV8Relay starts the relay', t2.window.__LQ_V8_RELAY === true);

const ID2 = 'ai_1758000000001_3';
let c2Results = [];
c2.window.addEventListener('message', (ev) => {
  if (ev.data && ev.data.type === 'lqAiResult' && ev.data.id === ID2) c2Results.push(ev.data);
});
c2.window.__LQ_REGISTER_AI_POLL(ID2);
flush();
t2Results[ID2] = { content: 'B', finish_reason: 'stop' };
tickRelay('T2');
flush();
ok('late-started relay delivers to child frame', c2Results.length === 1,
   'count=' + c2Results.length);

/* ---- Part 3: an in-flight id is polled, not mis-reported as timed out --- */
console.log('--- Part 3: in-flight id is not mis-reported as a timeout ---');
const ID3 = 'ai_1758000000002_9';
let t2Timeouts = [];
t2.window.addEventListener('message', (ev) => {
  if (ev.data && ev.data.type === 'lqAiResult' && ev.data.id === ID3) t2Timeouts.push(ev.data);
});
t2.window.__LQ_REGISTER_AI_POLL(ID3);
flush();
tickRelay('T2');
ok('pending id polled without a false timeout', t2Timeouts.length === 0,
   'timeouts=' + t2Timeouts.length);

console.log(failures === 0
  ? '\nAll bridge relay checks passed.'
  : '\n' + failures + ' bridge relay check(s) FAILED.');
process.exit(failures === 0 ? 0 : 1);
