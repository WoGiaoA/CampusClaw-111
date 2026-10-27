// Real HTTP verification; secrets are loaded locally and never written to report.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const env = Object.fromEntries(fs.readFileSync('.env', 'utf8').split(/\r?\n/).filter(line => /^[A-Z_0-9]+=/.test(line)).map(line => { const index = line.indexOf('='); return [line.slice(0, index), line.slice(index + 1)]; }));
const base = process.env.VERIFY_URL || 'http://127.0.0.1:8080';
const evidence = { timestamp: new Date().toISOString(), base, transport: process.env.VERIFY_COMPOSE === 'true' ? 'Docker/Nginx' : 'Local Go development proxy (NOT Docker/Nginx)', checks: [] };
function record(name, response, body, detail = {}) {
 const headers = Object.fromEntries(response.headers.entries());
 if (headers['set-cookie']) headers['set-cookie'] = headers['set-cookie'].replace(/campusclaw_session=[^;]*/, 'campusclaw_session=[REDACTED]');
 if (body && typeof body === 'object' && body.csrf_token) body = { ...body, csrf_token: '[REDACTED]' };
 evidence.checks.push({ name, status: response.status, headers, body, ...detail });
}
async function login(name, passwordOverride) {
 const password = passwordOverride || env[`SEED_${name.toUpperCase()}_PASSWORD`];
 const response = await fetch(`${base}/api/login`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: name, password }) });
 const body = await response.json();
 record(`login:${name}${passwordOverride ? ':failure' : ''}`, response, body, { request: { method: 'POST', url: `${base}/api/login`, content_type: 'application/json', payload: { username: name, password: '[REDACTED]' } } });
 const cookie = response.headers.get('set-cookie')?.split(';')[0];
 return { response, body, cookie };
}
async function get(path, cookie, name = path) {
 const response = await fetch(base + path, { headers: cookie ? { Cookie: cookie } : {} });
 const body = response.headers.get('content-type')?.includes('json') ? await response.json() : await response.text();
 record(name, response, body); return { response, body };
}
(async () => {
 const page = await get('/login'); assert.equal(page.response.status, 200);
 const shell = await get('/materials'); assert.equal(shell.response.status, 200);
 assert(!shell.body.includes('CLASS_A_ONLY') && !shell.body.includes('CLASS_B_ONLY'));
 const health = await get('/health'); assert.equal(health.response.status, 200);
 const wrong = await login('teacher_a', 'deliberately-incorrect'); assert.equal(wrong.response.status, 401);
 const unknown = await login('unknown_account', 'deliberately-incorrect'); assert.equal(unknown.response.status, 401); assert.deepEqual(wrong.body, unknown.body);
 const a = await login('teacher_a'); assert.equal(a.response.status, 200);
 const studentA = await login('student_a1'); const studentB = await login('student_b1');
 const listA = await get('/api/materials', a.cookie); const listB = await get('/api/materials', studentB.cookie);
 assert(listA.body.every(m => m.class_id === a.body.class_id)); assert(listB.body.every(m => m.class_id === studentB.body.class_id));
 const own = listA.body[0].id; const other = listB.body[0].id;
 let result = await get(`/api/materials/${own}/file`, null, 'download:anonymous'); assert.equal(result.response.status, 401);
 result = await get(`/api/materials/${own}/file`, studentA.cookie, 'download:student-own'); assert.equal(result.response.status, 200);
 const cross = await get(`/api/materials/${other}/file`, a.cookie, 'download:cross-class'); assert.equal(cross.response.status, 404);
 const missing = await get('/api/materials/99999999/file', a.cookie, 'download:missing-id'); assert.equal(missing.response.status, 404); assert.deepEqual(cross.body, missing.body);
 result = await get('/uploads/1/guessed.txt', a.cookie, 'download:public-path'); assert.equal(result.response.status, 404);
 const meA = await get('/api/me', a.cookie); const meStudent = await get('/api/me', studentA.cookie);
 const form = () => { const data = new FormData(); data.append('file', new Blob(['第三课 HTTP 验收材料\n'], { type: 'text/plain' }), 'lesson03-http.txt'); data.append('class_id', String(studentB.body.class_id)); return data; };
 let response = await fetch(base + '/api/materials/upload', { method: 'POST', headers: { Cookie: studentA.cookie, 'X-CSRF-Token': meStudent.body.csrf_token }, body: form() }); let body = await response.json(); record('upload:student', response, body); assert.equal(response.status, 403);
 response = await fetch(base + '/api/materials/upload', { method: 'POST', headers: { Cookie: a.cookie, 'X-CSRF-Token': meA.body.csrf_token }, body: form() }); body = await response.json(); record('upload:teacher', response, body); assert.equal(response.status, 201);
 const uploaded = await get(`/api/materials/${body.material_id}`, studentA.cookie, 'upload:visible-to-own-student'); assert.equal(uploaded.response.status, 200); assert.equal(uploaded.body.class_id, a.body.class_id);
 const knowledge = await get(`/api/knowledge-entries/${body.knowledge_entry_id}`, a.cookie, 'upload:knowledge-record'); assert.equal(knowledge.response.status, 200);
 const bad = new FormData(); bad.append('file', new Blob(['not-supported']), 'unsupported.pdf');
 response = await fetch(base + '/api/materials/upload', { method: 'POST', headers: { Cookie: a.cookie, 'X-CSRF-Token': meA.body.csrf_token }, body: bad }); body = await response.json(); record('upload:pdf-rejected', response, body); assert.equal(response.status, 400);
 response = await fetch(base + '/api/logout', { method: 'POST', headers: { Cookie: a.cookie, 'X-CSRF-Token': meA.body.csrf_token } }); body = await response.json(); record('logout', response, body); assert.equal(response.status, 200);
 result = await get('/api/me', a.cookie, 'logout:old-cookie-replay'); assert.equal(result.response.status, 401);
 // Do not save whole HTML shells in evidence; status is sufficient.
 for (const check of evidence.checks) if (typeof check.body === 'string' && check.body.includes('<!doctype html>')) check.body = '[HTML page shell; no business data]';
 fs.mkdirSync('deliverables/lesson-03', { recursive: true });
 fs.writeFileSync('deliverables/lesson-03/http-evidence.json', JSON.stringify(evidence, null, 2));
 console.log(`${evidence.checks.length} real HTTP checks passed. Sanitized evidence written; Docker runtime remains ${evidence.transport.includes('NOT') ? 'PENDING' : 'verified separately'}.`);
})().catch(error => { console.error(error.message); process.exitCode = 1; });
