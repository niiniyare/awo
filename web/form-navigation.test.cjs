// Tests for the create/edit form navigation fix (R.14/R.14-followup) in
// web/dist/index.html: applyFormPostSubmitNavigation, adaptResponse, and
// recordIdFromPath.
//
// Run with: node web/form-navigation.test.cjs
//
// SCOPE — read this before trusting what this file proves:
// These are UNIT tests of three pure functions, run under plain Node, with
// no DOM and no real amis SDK. They prove the functions produce the
// correct SHAPE for real, captured backend payloads — they do NOT prove
// amis itself behaves as expected when given that shape. That stronger
// claim (does amis's redirect/jumpTo mechanism, fetcher-response contract,
// and template resolution for "${id}" actually work as this code assumes)
// was verified separately, this session, by downloading the real,
// unmodified amis v6.7.0 SDK and executing it inside a jsdom DOM — real
// clicks on real rendered buttons/dropdowns, a real `amis.embed()` call,
// a real (mocked-network) submit — not just reading its source or testing
// our own code in isolation. That jsdom harness is NOT committed to this
// repo (it required `npm install jsdom`, a new devDependency this change
// intentionally does not add without explicit authorization) — ask if you
// want it added permanently. Its result, for the record: real-amis-SDK
// execution confirmed (a) a successful create navigates to the list, (b) a
// rejected create preserves entered values and shows the real error with
// no navigation, (c) a successful edit navigates to the correct detail URL
// including the real record id. See the findings register (§B.16) for the
// full transcript.
//
// This repo has no JS test framework (web/package.json only has vite, a
// bundler, not a test runner) and this change intentionally does not add
// one. This script uses only Node's built-in `assert` and extracts the
// real functions straight out of the shipped web/dist/index.html (via a
// regex, the same technique used to verify them live during development)
// so these tests can never silently drift from the code that actually
// ships — if a function's name or shape changes without this file being
// updated, extraction itself fails loudly rather than testing a stale copy.

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const indexHtml = fs.readFileSync(path.join(__dirname, 'dist', 'index.html'), 'utf8');

function extract(name, bodyEndPattern) {
  const re = new RegExp('function ' + name + '\\([^)]*\\) \\{[\\s\\S]*?' + bodyEndPattern);
  const m = indexHtml.match(re);
  assert.ok(m, name + ' not found in web/dist/index.html — extraction regex is stale');
  return m[0];
}

// eslint-disable-next-line no-eval
eval(extract('applyFormPostSubmitNavigation', '\\n    \\}'));
// eslint-disable-next-line no-eval
eval(extract('adaptResponse', '\\n    \\}'));
// eslint-disable-next-line no-eval
eval(extract('recordIdFromPath', '\\n    \\}'));

let passed = 0;
function test(name, fn) {
  fn();
  passed++;
  console.log('ok -', name);
}

// Real shape returned by GET /api/v1/ui/business/expenses/create (captured
// live from the running backend during development).
function realExpenseCreateSchema() {
  return {
    body: [{
      type: 'form',
      title: 'Expense',
      api: { method: 'POST', url: '/api/v1/business/expenses' },
      actions: [{ actionType: 'submit', id: 'submit', label: 'Save', type: 'button' }],
      body: [
        { name: 'description', type: 'input-text', required: true },
        { name: 'submitted_by', type: 'select', disabled: true, required: true, source: {} }
      ]
    }],
    title: 'Expense'
  };
}

// Real shape returned by GET /api/v1/ui/business/expenses/{id}/edit.
function realExpenseEditSchema() {
  return {
    body: [{
      type: 'form',
      title: 'Expense',
      api: { method: 'PATCH', url: '/api/v1/business/expenses/${id}' },
      initApi: 'GET:/api/v1/business/expenses/${id}',
      actions: [{ actionType: 'submit', id: 'submit', label: 'Save', type: 'button' }],
      body: [{ name: 'description', type: 'input-text', required: true }]
    }],
    title: 'Expense'
  };
}

// Real shape returned by GET /api/v1/ui/business/expenses (the list) —
// used to prove the fix leaves non-form pages alone.
function realExpenseListSchema() {
  return {
    body: [{ type: 'crud', api: { method: 'GET', url: '/api/v1/business/expenses' } }],
    title: 'Expenses'
  };
}

// A genuinely different entity's create schema (GET
// /api/v1/ui/iam/users/create) — proves the fix is generic, not
// Expense-specific.
function realIamUserCreateSchema() {
  return {
    body: [{
      type: 'form',
      api: { method: 'POST', url: '/api/v1/iam/users' },
      body: [{ name: 'email', type: 'input-text', required: true }]
    }],
    title: 'User'
  };
}

test('create form: redirect is set back to the list path', function () {
  const schema = applyFormPostSubmitNavigation(realExpenseCreateSchema(), 'business/expenses/create');
  assert.strictEqual(schema.body[0].redirect, '/ui/business/expenses');
});

test('edit form: redirect is set back to the detail path, not the list', function () {
  const id = '979fb9a4-83cc-40ac-929c-60b4aa4c2744';
  const schema = applyFormPostSubmitNavigation(realExpenseEditSchema(), 'business/expenses/' + id + '/edit');
  assert.strictEqual(schema.body[0].redirect, '/ui/business/expenses/' + id);
});

test('list page (not a create/edit form) is left completely unmodified', function () {
  const original = realExpenseListSchema();
  const before = JSON.stringify(original);
  const schema = applyFormPostSubmitNavigation(realExpenseListSchema(), 'business/expenses');
  assert.strictEqual(JSON.stringify(schema), before, 'list schema must be byte-for-byte unchanged');
  assert.strictEqual(schema.body[0].redirect, undefined);
});

test('a redirect the backend already supplied is never overwritten', function () {
  const schema = realExpenseCreateSchema();
  schema.body[0].redirect = '/ui/some/other/target';
  const result = applyFormPostSubmitNavigation(schema, 'business/expenses/create');
  assert.strictEqual(result.body[0].redirect, '/ui/some/other/target');
});

test('generic: a completely different entity (iam/users) gets the same treatment', function () {
  const schema = applyFormPostSubmitNavigation(realIamUserCreateSchema(), 'iam/users/create');
  assert.strictEqual(schema.body[0].redirect, '/ui/iam/users');
});

test('a malformed/empty schema does not throw', function () {
  assert.strictEqual(applyFormPostSubmitNavigation(null, 'business/expenses/create'), null);
  assert.deepStrictEqual(applyFormPostSubmitNavigation({}, 'business/expenses/create'), {});
});

/* ── adaptResponse (R.14 follow-up): proves the fetcher's returned shape
   never carries a top-level "status" property of its own — the exact
   defect that made amis treat every successful submission as a failure,
   because our own entities' business "status" field (e.g. Expense's
   draft/submitted/approved) got misread as amis's application-level
   status code once nested under a {status, data} wrapper. ────────────── */

test('adaptResponse: a real single-record success has no top-level "status" key', function () {
  // Real shape captured from a live POST /api/v1/business/expenses.
  var backendBody = {
    data: {
      id: '979fb9a4-83cc-40ac-929c-60b4aa4c2744',
      description: 'Diesel for generator',
      amount: 3400,
      category: 'supplies',
      status: 'approved', // the business field that caused the original bug
      submitted_by: '00d5e30b-822d-49a1-8b8c-9cc60ed3a687'
    }
  };
  var result = adaptResponse(backendBody);
  assert.ok(!Object.prototype.hasOwnProperty.call(result, 'status'),
    'adaptResponse\'s return value must never have its own top-level "status" — amis reads *.data.status, and our own records legitimately have that field');
  assert.deepStrictEqual(result, { data: backendBody });
});

test('adaptResponse: a real error response passes through so amis\'s own error.code/message heuristic applies', function () {
  var backendBody = { error: { code: 'expense.invalid_status_transition', message: 'cannot move expense from "draft" to "approved"' } };
  var result = adaptResponse(backendBody);
  assert.ok(!Object.prototype.hasOwnProperty.call(result, 'status'));
  assert.deepStrictEqual(result, { data: backendBody });
});

test('adaptResponse: a real list response is reshaped to {items, total} matching the schema\'s itemsKey', function () {
  var backendBody = {
    data: [{ id: 'a', description: 'One' }, { id: 'b', description: 'Two' }],
    meta: { total: 2, page: 1, page_size: 20 }
  };
  var result = adaptResponse(backendBody);
  assert.ok(!Object.prototype.hasOwnProperty.call(result, 'status'));
  assert.deepStrictEqual(result, { data: { items: backendBody.data, total: 2 } });
});

test('adaptResponse: an empty list still reshapes correctly (no off-by-one on falsy data)', function () {
  var backendBody = { data: [], meta: { total: 0, page: 1, page_size: 20 } };
  var result = adaptResponse(backendBody);
  assert.deepStrictEqual(result, { data: { items: [], total: 0 } });
});

/* ── recordIdFromPath (R.14 follow-up): proves the real bug behind "Edit:
   navigation behaves unexpectedly" — initApi/api templates referencing
   "${id}" never resolved because nothing supplied an id into the
   embedded instance's data scope, so an edit form always rendered blank
   and Save always failed client-side validation before reaching the
   server. ──────────────────────────────────────────────────────────── */

test('recordIdFromPath: extracts the id from a real edit path', function () {
  assert.strictEqual(
    recordIdFromPath('business/expenses/979fb9a4-83cc-40ac-929c-60b4aa4c2744/edit'),
    '979fb9a4-83cc-40ac-929c-60b4aa4c2744'
  );
});

test('recordIdFromPath: extracts the id from a real detail path', function () {
  assert.strictEqual(
    recordIdFromPath('business/expenses/979fb9a4-83cc-40ac-929c-60b4aa4c2744'),
    '979fb9a4-83cc-40ac-929c-60b4aa4c2744'
  );
});

test('recordIdFromPath: a list path has no id', function () {
  assert.strictEqual(recordIdFromPath('business/expenses'), undefined);
});

test('recordIdFromPath: a create path has no id ("create" is not a record id)', function () {
  assert.strictEqual(recordIdFromPath('business/expenses/create'), undefined);
});

test('recordIdFromPath: generic across a completely different entity', function () {
  assert.strictEqual(recordIdFromPath('iam/users/00d5e30b-822d-49a1-8b8c-9cc60ed3a687/edit'), '00d5e30b-822d-49a1-8b8c-9cc60ed3a687');
  assert.strictEqual(recordIdFromPath('iam/users/00d5e30b-822d-49a1-8b8c-9cc60ed3a687'), '00d5e30b-822d-49a1-8b8c-9cc60ed3a687');
  assert.strictEqual(recordIdFromPath('iam/users/create'), undefined);
});

/* ── amisEnv notify/alert (R.15 / §B.16): a real, user-reported defect —
   "Status field is not reliably selectable" — turned out to have nothing
   wrong with the Status field, the schema, or the lifecycle guard (all
   three confirmed correct by direct inspection and live API tests). The
   actual cause: this file's own amisEnv previously overrode amis's
   `notify`/`alert` with console.log-only stubs, silently swallowing every
   visible error amis would otherwise show — including a correctly-
   rejected status transition (e.g. "approved" is terminal; the backend
   correctly returns a real 409). amis.embed()'s own real, shipped default
   env (confirmed by reading the actual amis v6.7.0 SDK source) already
   wires notify to a real, visible toast (f.toast[type](msg), rendered via
   the ToastComponent embed() itself mounts into the page) and alert to
   amis's own modal (f.alert) — neither is a no-op, and neither needs a
   custom override. This is a pure STRUCTURAL guard (can't prove a toast
   renders without a DOM — that was verified separately this session via
   the real amis SDK in jsdom, not committed here; see findings register
   §B.16 for that transcript) but it directly prevents the regression: if
   either key is ever reintroduced, this fails immediately. ────────────── */

test('amisEnv does not override notify or alert — real amis defaults must handle them', function () {
  var envMatch = indexHtml.match(/var amisEnv = \{[\s\S]*?\n    \};/);
  assert.ok(envMatch, 'amisEnv object literal not found — extraction is stale');
  var envSource = envMatch[0];
  assert.ok(!/\n\s*notify\s*:/.test(envSource),
    'amisEnv must not define its own "notify" — it silently replaces amis\'s real, visible toast with a no-op, hiding every rejected action (including correctly-enforced lifecycle rejections) from the user');
  assert.ok(!/\n\s*alert\s*:/.test(envSource),
    'amisEnv must not define its own "alert" — same reasoning as notify');
  // What SHOULD still be there: these are genuinely NOT provided by amis's
  // own embed() defaults (confirmed from source), so overriding them is
  // correct, not redundant.
  assert.ok(/\bfetcher\s*:/.test(envSource), 'fetcher must still be overridden (auth headers, page_size translation)');
  assert.ok(/\bjumpTo\s*:/.test(envSource), 'jumpTo must still be overridden (SPA routing integration)');
  assert.ok(/\bisCancel\s*:/.test(envSource), 'isCancel must still be provided — amis calls it unconditionally in some paths');
});

console.log(passed + '/' + passed + ' passed');
