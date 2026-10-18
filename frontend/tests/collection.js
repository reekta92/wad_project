import { expect } from '@playwright/test';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import newman from 'newman';

const root = resolve('..');
const executed = new Set();
const counts = [];

function contractOperations() {
  const operations = new Set();
  let path;
  for (const line of readFileSync(resolve(root, 'docs/openapi.yaml'), 'utf8').split('\n')) {
    const route = /^  (\/[^:]+):$/.exec(line);
    if (route) path = route[1];
    const method = /^    (get|post|put|delete):$/.exec(line);
    if (method && path) operations.add(`${method[1].toUpperCase()} ${path}`);
  }
  return operations;
}

export async function verifyCollection(request, token, administrator) {
  const collection = JSON.parse(readFileSync(resolve(root, 'docs/postman/exams.postman_collection.json'), 'utf8'));
  const variables = [
    { key: 'base_url', value: 'http://localhost:8080' },
    { key: 'issuer', value: 'http://localhost:8081/realms/exams' },
    { key: 'q', value: 'web' },
    { key: 'access_token', value: token, type: 'secret' },
  ];
  if (administrator) {
    const created = await request.post('/api/v1/unavailability-requests', { headers: { Authorization: `Bearer ${token}` }, data: { personnel_id: 1, date: '2026-11-06', session_id: null, type: 'leave', reason: 'Disposable Postman collection decision fixture' } });
    expect(created.status()).toBe(201);
    variables.push({ key: 'decision_request_id', value: String((await created.json()).id) });
    collection.item = collection.item.filter((item) => item.name !== 'Own reports (invigilator)');
  } else collection.item = collection.item.filter((item) => item.name === 'Own reports (invigilator)');
  const summary = await new Promise((done, reject) => {
    newman.run({ collection, environment: { values: variables }, reporters: [], timeoutRequest: 60000, bail: true }, (error, result) => {
      if (error) reject(error); else done(result);
    }).on('request', (error, args) => {
      if (!error) {
        const path = args.request.url.getPath().replace(/^\/api\/v1/, '').replace(/\/[1-9]\d*(?=\/|$)/g, '/{id}');
        if (path !== '/missing') executed.add(`${args.request.method} ${path}`);
      }
    });
  });
  const failures = summary.run.failures.map((failure) => ({ name: failure.source?.name, message: failure.error.message }));
  expect(failures, 'Postman collection assertions').toEqual([]);
  counts.push({ role: administrator ? 'administrator' : 'invigilator', requests: summary.run.stats.requests.total, assertions: summary.run.stats.assertions.total });
  if (administrator) {
    expect([...executed].sort()).toEqual([...contractOperations()].sort());
    const directory = resolve(root, '.local/collection-evidence');
    mkdirSync(directory, { recursive: true });
    // Full Newman summaries contain tokens and environment values; persist only counts and inventory.
    writeFileSync(resolve(directory, 'summary.json'), JSON.stringify({ counts, operations: [...executed].sort() }, null, 2), { mode: 0o600 });
    console.log(`Postman collection proof: ${executed.size} API operations, ${counts.reduce((total, run) => total + run.assertions, 0)} passing assertions, tokens from real PKCE/MFA login.`);
  }
}
