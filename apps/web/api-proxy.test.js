// Tests apps/web/api-proxy.js against a stand-in API on a real port: an RPC the
// page makes on its own origin reaches the API with its method, headers and body,
// the API's answer (errors included) comes back as it is, and nothing outside the
// API's paths is forwarded.
const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const express = require('express');
const { apiProxy } = require('./api-proxy');

function listen(server) {
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${server.address().port}`)));
}

async function setup(apiHandler) {
  const seen = [];
  const api = http.createServer(async (req, res) => {
    const chunks = [];
    for await (const c of req) chunks.push(c);
    seen.push({ method: req.method, url: req.url, headers: req.headers, body: Buffer.concat(chunks).toString() });
    apiHandler(req, res);
  });
  const apiOrigin = await listen(api);
  const app = express();
  app.use(apiProxy(apiOrigin));
  app.use((_req, res) => res.status(200).send('the page'));
  const web = http.createServer(app);
  const origin = await listen(web);
  return { seen, origin, close: () => { api.close(); web.close(); } };
}

test('an RPC is forwarded with its method, headers and body, and the answer comes back', async (t) => {
  const { seen, origin, close } = await setup((_req, res) => {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end('{"version":"abc1234"}');
  });
  t.after(close);
  const res = await fetch(`${origin}/issho.v1.HealthService/Check`, {
    method: 'POST',
    headers: { 'content-type': 'application/json', authorization: 'Bearer t' },
    body: '{}',
  });
  assert.equal(res.status, 200);
  assert.equal(res.headers.get('content-type'), 'application/json');
  assert.deepEqual(await res.json(), { version: 'abc1234' });
  assert.equal(seen.length, 1);
  assert.equal(seen[0].method, 'POST');
  assert.equal(seen[0].url, '/issho.v1.HealthService/Check');
  assert.equal(seen[0].body, '{}');
  assert.equal(seen[0].headers.authorization, 'Bearer t');
  assert.equal(seen[0].headers['content-type'], 'application/json');
});

test("the API's errors pass through with their status and body", async (t) => {
  const { origin, close } = await setup((_req, res) => {
    res.writeHead(401, { 'content-type': 'application/json' });
    res.end('{"code":"unauthenticated"}');
  });
  t.after(close);
  const res = await fetch(`${origin}/issho.v1.PlanService/List`, { method: 'POST', body: '{}', headers: { 'content-type': 'application/json' } });
  assert.equal(res.status, 401);
  assert.deepEqual(await res.json(), { code: 'unauthenticated' });
});

test('a path outside the API is the page, and never reaches the API', async (t) => {
  const { seen, origin, close } = await setup((_req, res) => res.end('api'));
  t.after(close);
  for (const p of ['/', '/plans', '/issho.v1', '/x/issho.v1.HealthService/Check']) {
    const res = await fetch(origin + p);
    assert.equal(await res.text(), 'the page', p);
  }
  assert.equal(seen.length, 0);
});

test('an API that cannot be reached answers 502', async (t) => {
  const app = express();
  app.use(apiProxy('http://127.0.0.1:1'));
  const web = http.createServer(app);
  const origin = await listen(web);
  t.after(() => web.close());
  const res = await fetch(`${origin}/issho.v1.HealthService/Check`, { method: 'POST', body: '{}' });
  assert.equal(res.status, 502);
});

// Issho keeps no IP address anywhere (docs/spec/privacy.md), so the member's
// address, which Cloudflare and kamal-proxy put in these, never reaches the API.
const CLIENT_ADDRESS = {
  'cf-connecting-ip': '198.51.100.7',
  'cf-connecting-ipv6': '2001:db8::7',
  'cf-pseudo-ipv4': '198.51.100.8',
  'cf-ipcountry': 'VN',
  'x-forwarded-for': '198.51.100.7, 203.0.113.1',
  'x-real-ip': '198.51.100.7',
  forwarded: 'for=198.51.100.7;proto=https',
  'true-client-ip': '198.51.100.7',
  'x-client-ip': '198.51.100.7',
  'x-cluster-client-ip': '198.51.100.7',
  'client-ip': '198.51.100.7',
};

test("the member's address is not passed on to the API", async (t) => {
  const { seen, origin, close } = await setup((_req, res) => res.end('{}'));
  t.after(close);
  await fetch(`${origin}/issho.v1.HealthService/Check`, {
    method: 'POST',
    headers: { ...CLIENT_ADDRESS, 'content-type': 'application/json', authorization: 'Bearer t' },
    body: '{}',
  });
  assert.equal(seen.length, 1);
  for (const name of Object.keys(CLIENT_ADDRESS)) assert.equal(seen[0].headers[name], undefined, name);
  // Fetch's own connection to the API is the forwarder's, never the member's.
  assert.doesNotMatch(JSON.stringify(seen[0].headers), /198\.51\.100|2001:db8/);
  assert.equal(seen[0].headers.authorization, 'Bearer t');
});
