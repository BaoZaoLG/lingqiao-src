#!/usr/bin/env node
/*
 * check_ai_preflight.js — regression gate for the AutoExam AI request path.
 *
 * Two behaviours are protected here, both of which used to fail silently:
 *
 *  1. The page must refuse to enter its "fetching answer" state when the loaded
 *     CefHook.dll does not expose the AI mailbox methods. Previously it sent the
 *     request, polled a method the DLL did not implement, swallowed the
 *     METHOD_NOT_FOUND in an unconditional retry loop and sat in the wait state
 *     until the 45s watchdog fired — with no error ever shown.
 *
 *  2. A terminal bridge error from the poll must settle the request instead of
 *     being retried forever.
 *
 * This harness loads the real packages/autoexam/src/autoexam.js with a minimal
 * DOM, points it at a fake DLL whose behaviour each scenario controls, and
 * asserts on the observable result. No build and no browser required.
 *
 * Usage: node scripts/check_ai_preflight.js [path/to/autoexam.js]
 *        LQ_DEBUG=1 node scripts/check_ai_preflight.js     (dump raw logs)
 */
'use strict';

const fs = require('fs');
const path = require('path');
const vm = require('vm');

const DEBUG = !!process.env.LQ_DEBUG;
const src = process.argv.slice(2).find(a => a.indexOf('-') !== 0) ||
  path.join(__dirname, '..', 'packages', 'autoexam', 'src', 'autoexam.js');
const SCRIPT = fs.readFileSync(src, 'utf8');

const ALL_METHODS = ['runtime.ping', 'runtime.getCapabilities', 'runtime.getStatus',
                     'diagnostics.log', 'ai.chatComplete', 'ai.pollResult',
                     'native.pollTrigger', 'page.reportQuestionState'];

let failures = 0;

function ok(label, cond, extra) {
  if (!cond) failures++;
  console.log((cond ? 'PASS  ' : 'FAIL  ') + label +
    (extra !== undefined ? '   [' + extra + ']' : ''));
}

function makeEl(tag) {
  const el = {
    tagName: (tag || 'div').toUpperCase(),
    id: '', className: '', value: '', textContent: '', innerHTML: '',
    isConnected: true, style: {}, children: [], parentNode: null,
    classList: { add() {}, remove() {}, contains: () => false, toggle() {} },
    getAttribute: () => null, setAttribute() {}, removeAttribute() {},
    querySelector: () => null, querySelectorAll: () => [],
    appendChild(child) { el.children.push(child); if (child) child.parentNode = el; return child; },
    removeChild() {}, insertBefore() {}, addEventListener() {}, removeEventListener() {},
    dispatchEvent() {},
    getBoundingClientRect: () => ({ top: 0, left: 0, width: 0, height: 0 })
  };
  return el;
}

/*
 * scenario:
 *   capabilities — the method list the fake DLL reports
 *   pollRejects  — when set, ai.pollResult rejects with this error object
 *                  (mirrors the real bootstrap rejecting on envelope ok:false)
 */
function buildEnvironment(scenario) {
  const styles = [];
  const consoleErrors = [];
  const consoleWarns = [];
  const diagLogs = [];
  const sent = [];
  const intervals = [];
  const pollReady = [];

  const questionLi = makeEl('div');
  const head = makeEl('head');
  const body = makeEl('body');

  /* A few elements with class names, so the dom-state class sampler has
   * something to report. Scenario D relies on this. */
  const bodyNodes = ['examPaperHead', 'timuWrapper', 'answerArea'].map((cls) => {
    const el = makeEl('div');
    el.className = cls;
    return el;
  });

  const document = {
    readyState: 'complete',
    head, body,
    documentElement: makeEl('html'),
    activeElement: null,
    onselectstart: null,
    createElement(tag) {
      const el = makeEl(tag);
      if (String(tag).toLowerCase() === 'style') styles.push(el);
      return el;
    },
    createEvent: (t) => ({ type: t, initEvent() {} }),
    querySelector(sel) {
      if (sel === '.questionLi' && scenario.hasExamDom !== false) return questionLi;
      return null;
    },
    querySelectorAll(sel) {
      return sel === 'body *' ? bodyNodes : [];
    },
    getElementById: () => null,
    addEventListener() {}, removeEventListener() {}
  };

  const store = new Map();
  const sessionStorage = {
    getItem: (k) => (store.has(k) ? store.get(k) : null),
    setItem: (k, v) => store.set(k, String(v)),
    removeItem: (k) => store.delete(k)
  };

  const bridge = {
    ready: true,
    version: '2.0',
    transport: 'console',
    __send(envelope) {
      if (envelope && envelope.method) sent.push(envelope.method);
      return true;
    },
    request(method, params) {
      if (method === 'runtime.getCapabilities') {
        return Promise.resolve({ methods: scenario.capabilities });
      }
      if (method === 'runtime.getStatus') {
        return Promise.resolve({
          build_stamp: 'Sep 16 2026 18:09:00',
          build_arch: 'x86',
          method_count: scenario.capabilities.length,
          ai_ready: scenario.capabilities.indexOf('ai.pollResult') >= 0
        });
      }
      if (method === 'diagnostics.log') {
        diagLogs.push(params && params.message);
        return Promise.resolve({ logged: true });
      }
      if (method === 'ai.pollResult') {
        if (scenario.pollRejects) return Promise.reject(scenario.pollRejects);
        return Promise.resolve({ pending: true });
      }
      if (method === 'native.pollTrigger') {
        pollReady.push(params ? params.ready : undefined);
        return Promise.resolve({ pending: false });
      }
      return Promise.resolve({});
    }
  };

  /* Short delays (the poll's own ticks) are compressed so the test is fast.
   * Long delays are the 45s loading watchdog: recorded but never fired, so a
   * scenario can only leave the loading state through the code path under test
   * rather than being rescued by the watchdog. */
  const longTimers = [];
  const setTimeoutStub = (fn, ms) => {
    const d = ms || 0;
    if (d > 2000) {
      longTimers.push(d);
      return -1;
    }
    return setTimeout(fn, Math.max(1, Math.min(d, 5)));
  };

  const sandbox = {
    document, sessionStorage, localStorage: sessionStorage,
    console: {
      log() {}, debug() {}, info() {},
      warn(...a) { consoleWarns.push(a.join(' ')); },
      error(...a) { consoleErrors.push(a.join(' ')); }
    },
    setTimeout: setTimeoutStub,
    clearTimeout,
    setInterval: (fn) => { intervals.push(fn); return intervals.length; },
    clearInterval() {},
    JSON, Object, Promise, Date, Math, RegExp, String, Array, Boolean, Error,
    isNaN, parseInt, parseFloat
  };
  sandbox.window = sandbox;
  sandbox.globalThis = sandbox;
  sandbox.window.location = { href: 'https://exam.example/paper' };
  sandbox.window.__AUTOEXAM_AI_CONFIG = { model: 'test-model', baseUrl: 'https://api.example/v1' };
  sandbox.window.LingQiaoBridge = bridge;
  sandbox.window.addEventListener = () => {};
  sandbox.window.removeEventListener = () => {};
  sandbox.window.postMessage = () => {};

  return { sandbox, styles, consoleErrors, consoleWarns, diagLogs, sent, longTimers, intervals, pollReady };
}

function allText(list) { return list.join('\n'); }

/* The script reports through both channels depending on severity. */
function uiText(env) {
  return env.consoleErrors.join('\n') + '\n' + env.consoleWarns.join('\n');
}

function cursorIsWaiting(styles) {
  return styles.some(s => String(s.textContent || '').indexOf('cursor:wait') >= 0);
}

/* Real timers, because pollAiResult schedules its first tick with setTimeout. */
function settle(ms) {
  return new Promise(r => setTimeout(r, ms));
}

async function runScenario(name, scenario) {
  const env = buildEnvironment(scenario);
  vm.createContext(env.sandbox);
  vm.runInContext(SCRIPT, env.sandbox, { filename: name + '.js' });

  await settle(30);                       /* let the capability probe settle */

  const autoExam = env.sandbox.window.AutoExam;
  ok('[' + name + '] AutoExam installed',
     !!(autoExam && typeof autoExam.trigger === 'function'));

  /* Middle-click equivalent. */
  autoExam.trigger('test');
  await settle(60);                       /* let the request/poll cycle run */

  env.state = autoExam.debugInfo ? autoExam.debugInfo().middleButtonState : 'n/a';

  if (DEBUG) {
    console.log('  [debug ' + name + '] state=' + env.state);
    console.log('  [debug ' + name + '] sent=' + JSON.stringify(env.sent));
    console.log('  [debug ' + name + '] errors=' + JSON.stringify(env.consoleErrors));
    console.log('  [debug ' + name + '] warns=' + JSON.stringify(env.consoleWarns));
    console.log('  [debug ' + name + '] diag=' + JSON.stringify(env.diagLogs));
  }
  return env;
}

(async function main() {
  /* ---- A: DLL lacks the AI mailbox entirely ---------------------------- */
  console.log('--- A: DLL is missing the AI mailbox ---');
  const a = await runScenario('missing', {
    capabilities: ['runtime.ping', 'runtime.getCapabilities', 'ai.chatComplete']
  });

  ok('probe reports the missing capability',
     uiText(a).indexOf('缺少 AI 能力') >= 0);
  ok('captures the DLL build identity',
     uiText(a).indexOf('build_stamp=') >= 0);
  ok('aborts before entering the wait state',
     uiText(a).indexOf('不含 ai.pollResult') >= 0);
  ok('does NOT start the wait cursor', cursorIsWaiting(a.styles) === false);
  ok('does NOT send an AI request', a.sent.indexOf('ai.chatComplete') < 0);
  ok('stays idle', a.state === 'idle', 'state=' + a.state);

  /* ---- B: healthy DLL, request goes out (positive control) ------------- */
  console.log('--- B: healthy DLL sends the request ---');
  const b = await runScenario('ready', { capabilities: ALL_METHODS });

  ok('probe reports the capability as OK',
     allText(b.diagLogs).indexOf('capability ok') >= 0);
  ok('does NOT log a missing-capability abort',
     uiText(b).indexOf('不含 ai.pollResult') < 0);
  ok('sends the AI request', b.sent.indexOf('ai.chatComplete') >= 0);
  ok('does NOT ask for the build identity when nothing is wrong',
     uiText(b).indexOf('build_stamp=') < 0);

  /* ---- C: poll fails terminally -> must settle, not spin --------------- */
  console.log('--- C: terminal poll error settles instead of hanging ---');
  const c = await runScenario('terminal-error', {
    capabilities: ALL_METHODS,
    pollRejects: { code: 'METHOD_NOT_FOUND', message: 'Unknown method: ai.pollResult' }
  });

  ok('sent the AI request', c.sent.indexOf('ai.chatComplete') >= 0);
  ok('armed the 45s loading watchdog (held back by the harness)',
     c.longTimers.indexOf(45000) >= 0, 'longTimers=' + JSON.stringify(c.longTimers));
  ok('leaves the loading state without the watchdog rescuing it',
     c.state !== 'loading', 'state=' + c.state);
  ok('surfaces the failure', uiText(c).indexOf('API error') >= 0 ||
     allText(c.diagLogs).indexOf('api error') >= 0);
  ok('clears the wait cursor', cursorIsWaiting(c.styles) === false);

  /* ---- D: a frame with no exam DOM must say so, loudly ------------------ */
  console.log('--- D: a frame without exam DOM reports why nothing happens ---');
  const d = await runScenario('no-exam-dom', {
    capabilities: ALL_METHODS,
    hasExamDom: false
  });

  /* Drive the 300ms trigger poll interval by hand. */
  d.intervals.forEach(fn => { try { fn(); } catch (e) {} });
  await settle(20);

  ok('poll reports ready=0 so the DLL withholds the trigger',
     d.pollReady.length > 0 && d.pollReady.every(v => v === 0),
     'ready=' + JSON.stringify(d.pollReady));

  const domLine = d.diagLogs.filter(l => l && l.indexOf('dom-state') === 0)[0];
  ok('logs a dom-state line naming this frame',
     !!domLine && domLine.indexOf('ready=0') >= 0, domLine);
  ok('dom-state carries selector counts',
     !!domLine && domLine.indexOf('.questionLi=') >= 0);
  ok('dom-state carries the frame url',
     !!domLine && domLine.indexOf('href=') >= 0);
  ok('dom-state samples the real class names when not ready',
     !!domLine && domLine.indexOf('classes=') >= 0 &&
     domLine.indexOf('timuWrapper') >= 0,
     domLine && domLine.slice(domLine.indexOf('classes='), domLine.indexOf('href=')));
  ok('no AI request is sent from a non-exam frame',
     d.sent.indexOf('ai.chatComplete') < 0);

  console.log(failures === 0
    ? '\nAll AI preflight checks passed.'
    : '\n' + failures + ' AI preflight check(s) FAILED.');
  process.exit(failures === 0 ? 0 : 1);
})();
