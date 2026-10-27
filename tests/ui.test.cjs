// DOM-only functional checks. No browser, screenshots, or image artifacts.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { JSDOM } = require('../.local/ui/node_modules/jsdom');
const vm = require('node:vm');
const html = readFileSync('web/index.html', 'utf8');
const script = readFileSync('web/app.js', 'utf8');
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
async function mount(role) {
 const dom = new JSDOM(html, { url: 'http://localhost/materials', runScripts: 'outside-only' });
 const calls = [];
 dom.window.fetch = async (path, options) => {
  calls.push({ path, options });
  const body = path === '/api/me'
   ? { username: 'test_user', role, class_id: 1, class_name: 'A 班', csrf_token: 'local-test-only' }
   : [{ id: 1, title: '<script>unsafe</script>', original_filename: 'a.txt', byte_size: 12, created_at: '2026-10-13T00:00:00Z' }];
  return { status: 200, ok: true, json: async () => body };
 };
 dom.window.eval(script); await pause(30); return { dom, calls };
}
test('student sees own material/download and no upload form; title escaped', async () => {
 const { dom, calls } = await mount('student');
 assert.equal(dom.window.document.querySelector('#upload-form'), null);
 assert.equal(dom.window.document.querySelector('.material h3').textContent, '<script>unsafe</script>');
 assert.equal(dom.window.document.querySelector('.material h3 script'), null);
 assert.equal(dom.window.document.querySelector('.material a').getAttribute('href'), '/api/materials/1/file');
 assert.equal(calls[0].path, '/api/me'); assert.equal(calls[1].path, '/api/materials'); dom.window.close();
});
test('teacher receives upload form and me-derived identity', async () => {
 const { dom } = await mount('teacher');
 assert.ok(dom.window.document.querySelector('#upload-form'));
 assert.match(dom.window.document.querySelector('#identity').textContent, /test_user.*教师/);
 assert.equal(dom.window.document.querySelector('input[type=file]').accept, '.txt,.md'); dom.window.close();
});
test('login is JSON POST, ordinary account/password form, no account picker', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/login', runScripts: 'outside-only' });
 dom.window.fetch = async () => ({status:200,ok:true,json:async()=>({demo_registration:true,classes:[]})});
 dom.window.eval(script);
 await pause(10);
 assert.ok(dom.window.document.querySelector('input[autocomplete=username]'));
 assert.ok(dom.window.document.querySelector('input[type=password]'));
 assert.equal(dom.window.document.querySelector('select'), null);
 assert.equal(dom.window.document.querySelector('#account-note a').getAttribute('href'), '/register');
 const calls=[]; let finishLogin;
 dom.window.fetch=async(path,options)=>{calls.push({path,options});return new Promise(resolve=>{finishLogin=resolve;});};
 const form=dom.window.document.querySelector('#login-form');form.elements.username.value='fixture_teacher';form.elements.password.value='wrong';
 form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));
 assert.equal(form.querySelector('button').disabled,true);assert.equal(form.querySelector('button').getAttribute('aria-busy'),'true');
 assert.equal(calls[0].path,'/api/login');assert.equal(calls[0].options.method,'POST');assert.equal(calls[0].options.headers['Content-Type'],'application/json');
 assert.deepEqual(JSON.parse(calls[0].options.body),{username:'fixture_teacher',password:'wrong'});
 finishLogin({status:401,ok:false,json:async()=>({error:'账号或密码错误'})});await pause(10);
 assert.equal(form.querySelector('button').disabled,false);assert.equal(form.elements.password.getAttribute('aria-invalid'),'true');
 assert.equal(dom.window.document.querySelector('#login-error').hidden,false);assert.match(dom.window.document.querySelector('#login-error').textContent,/账号或密码错误/);
 form.elements.password.dispatchEvent(new dom.window.Event('input',{bubbles:true}));assert.equal(form.elements.password.hasAttribute('aria-invalid'),false);
 dom.window.close();
});

test('first-login material page navigates to profile before fetching business data', async () => {
 let target;const calls=[];const appNode={};
 vm.runInNewContext(script,{document:{querySelector:()=>appNode},location:{pathname:'/materials',replace:url=>{target=url;}},fetch:async path=>{calls.push(path);return {status:200,ok:true,json:async()=>({must_change_password:true})};},Error});
 await pause(10);assert.equal(target,'/profile');assert.deepEqual(calls,['/api/me']);assert.equal(appNode.innerHTML,undefined);
});
test('registration posts only username/class code and displays local initial password', async () => {
 const dom=new JSDOM(html,{url:'http://localhost/register',runScripts:'outside-only'});const calls=[];
 dom.window.fetch=async(path,options)=>{calls.push({path,options});return {status:path==='/api/register'?201:200,ok:true,json:async()=>path==='/api/register'?{ok:true}:{demo_registration:true,classes:[{code:'A',name:'A 班'},{code:'B',name:'B 班'}]}};};
 dom.window.eval(script);await pause(10);const form=dom.window.document.querySelector('#register-form');form.elements.username.value='new_student';form.elements.class_code.value='B';form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));await pause(10);
 assert.deepEqual(JSON.parse(calls[1].options.body),{username:'new_student',class_code:'B'});assert.match(dom.window.document.querySelector('#register-status').textContent,/123456/);assert.equal(form.hidden,true);dom.window.close();
});
test('profile shows trusted identity, first change hides current password, normal requires it',async()=>{
 for(const first of [true,false]){
 const dom=new JSDOM(html,{url:'http://localhost/profile',runScripts:'outside-only'});
 dom.window.fetch=async()=>({status:200,ok:true,json:async()=>({username:'student_test',role:'student',class_name:'A 班',must_change_password:first,csrf_token:'test-token'})});dom.window.eval(script);await pause(10);
 assert.match(dom.window.document.querySelector('#profile-identity').textContent,/student_test.*学生.*A 班/);
 assert.equal(dom.window.document.querySelector('#current-password-label').hidden,first);assert.equal(dom.window.document.querySelector('[name=current_password]').required,!first);assert.equal(dom.window.document.querySelector('#profile-back').hidden,first);assert.equal(dom.window.document.querySelector('[name=role]'),null);assert.equal(dom.window.document.querySelector('[name=class_id]'),null);dom.window.close();}
});
test('production login hides registration and production register page refuses form',async()=>{
 for(const path of ['/login','/register']){const dom=new JSDOM(html,{url:'http://localhost'+path,runScripts:'outside-only'});dom.window.fetch=async()=>({status:200,ok:true,json:async()=>({demo_registration:false,classes:[]})});dom.window.eval(script);await pause(10);assert.equal(dom.window.document.querySelector('[href="/register"]'),null);assert.equal(dom.window.document.querySelector('#register-form'),null);dom.window.close();}
});
test('anonymous API 401 navigates material page to login before rendering data', async () => {
 let target;
 const appNode = {};
 vm.runInNewContext(script, {
  document: { querySelector: () => appNode },
  location: { pathname: '/materials', replace: url => { target = url; } },
  fetch: async () => ({ status: 401, ok: false, json: async () => ({ error: '请先登录' }) }),
  Error,
 });
 await pause(10);
 assert.equal(target, '/login');
 assert.equal(appNode.textContent, '请先登录');
 assert.equal(appNode.innerHTML, undefined);
});

test('gateway HTML and network failures produce readable messages without exposing HTML', async () => {
 const context=vm.createContext({document:{querySelector:()=>({replaceChildren(){}})},location:{pathname:'/materials',replace(){}},fetch:async()=>{throw new Error('offline');},Error});
 vm.runInContext(script,context);await pause(10);
 await assert.rejects(vm.runInContext("api('/api/materials')",context),/网络连接中断/);
 context.fetch=async()=>({status:502,ok:false,json:async()=>{throw new SyntaxError('<html>bad gateway</html>');}});
 await assert.rejects(vm.runInContext("api('/api/materials')",context),/服务暂不可用（HTTP 502）/);
});

test('committed upload followed by list failure stays successful and can refresh without reupload', async () => {
 const {dom}=await mount('teacher');let uploads=0;let failList=true;
 dom.window.fetch=async path=>{
  if(path==='/api/materials/upload'){uploads++;return {status:201,ok:true,json:async()=>({material_id:2,knowledge_entry_id:2})};}
  if(failList)return {status:503,ok:false,json:async()=>({error:'服务暂不可用'})};
  return {status:200,ok:true,json:async()=>[{id:2,title:'已提交材料',original_filename:'saved.txt',byte_size:5,created_at:'2026-10-27T00:00:00Z'}]};
 };
 const form=dom.window.document.querySelector('#upload-form');
 form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));await pause(10);
 assert.match(dom.window.document.querySelector('#status').textContent,/上传已成功.*列表刷新失败.*无需重复上传/);
 assert.equal(form.querySelector('button').disabled,false);assert.equal(uploads,1);
 failList=false;dom.window.document.querySelector('#refresh-materials').click();await pause(10);
 assert.equal(uploads,1);assert.equal(dom.window.document.querySelector('.material h3').textContent,'已提交材料');dom.window.close();
});
test('successful login preserves ordinary and first-change navigation and clears submitted password',async()=>{
 for(const first of [false,true]){
  const dom=new JSDOM(html,{url:'http://localhost/login',runScripts:'outside-only'});let target;
  const context=vm.createContext({document:dom.window.document,location:{pathname:'/login',assign:url=>{target=url;}},fetch:async path=>({status:200,ok:true,json:async()=>path==='/api/account-options'?{demo_registration:false}:{must_change_password:first}}),Error});
  vm.runInContext(script,context);await pause(10);const form=dom.window.document.querySelector('#login-form');form.elements.username.value='fixture';form.elements.password.value='FixturePass1';form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));await pause(10);
  assert.equal(target,first?'/profile':'/materials');assert.equal(form.elements.password.value,'');dom.window.close();
 }
});
test('file selection is safe text; upload keeps multipart, CSRF, busy state and resets selection',async()=>{
 const {dom}=await mount('teacher');const form=dom.window.document.querySelector('#upload-form');const input=form.elements.file;
 const selected=new dom.window.File(['lecture content'],'<unsafe>.md',{type:'text/markdown'});
 Object.defineProperty(input,'files',{value:[selected],configurable:true});input.dispatchEvent(new dom.window.Event('change',{bubbles:true}));
 assert.equal(dom.window.document.querySelector('#file-name').textContent,'<unsafe>.md');assert.equal(dom.window.document.querySelector('#file-name unsafe'),null);assert.match(dom.window.document.querySelector('#file-meta').textContent,/15 B/);
 const originalFetch=dom.window.fetch;let uploadOptions;let finishUpload;
 dom.window.fetch=async(path,options)=>{if(path==='/api/materials/upload'){uploadOptions=options;return new Promise(resolve=>{finishUpload=resolve;});}return originalFetch(path,options);};
 form.elements.title.value='课程材料';form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));
 assert.equal(form.querySelector('button').disabled,true);assert.equal(form.getAttribute('aria-busy'),'true');assert.equal(uploadOptions.headers['X-CSRF-Token'],'local-test-only');assert.equal(uploadOptions.headers['Content-Type'],undefined);assert.ok(uploadOptions.body instanceof dom.window.FormData);assert.equal(uploadOptions.body.get('title'),'课程材料');assert.ok(uploadOptions.body.has('file'));
 finishUpload({status:201,ok:true,json:async()=>({material_id:2,knowledge_entry_id:2})});await pause(10);
 assert.equal(form.querySelector('button').disabled,false);assert.equal(form.getAttribute('aria-busy'),'false');assert.equal(dom.window.document.querySelector('#file-name').textContent,'尚未选择文件');assert.equal(dom.window.document.querySelector('#status').dataset.tone,'success');dom.window.close();
});
test('refresh shows busy state, restores control and presents empty state without changing permissions',async()=>{
 const {dom}=await mount('student');let finishRefresh;
 dom.window.fetch=async()=>new Promise(resolve=>{finishRefresh=resolve;});const button=dom.window.document.querySelector('#refresh-materials');button.click();
 assert.equal(button.disabled,true);assert.match(button.textContent,/刷新中/);assert.equal(dom.window.document.querySelector('#material-list').getAttribute('aria-busy'),'true');
 finishRefresh({status:200,ok:true,json:async()=>[]});await pause(10);
 assert.equal(button.disabled,false);assert.match(dom.window.document.querySelector('.empty').textContent,/本班暂时没有材料/);assert.equal(dom.window.document.querySelector('#upload-form'),null);assert.equal(dom.window.document.querySelector('#status').dataset.tone,'success');dom.window.close();
});
test('view body uses protected detail URL, escapes content and removes closed dialog',async()=>{
 const {dom}=await mount('student');const originalFetch=dom.window.fetch;let detailPath;
 dom.window.HTMLDialogElement.prototype.showModal=function(){this.open=true;};dom.window.HTMLDialogElement.prototype.close=function(){this.open=false;this.dispatchEvent(new dom.window.Event('close'));};
 dom.window.fetch=async(path,options)=>{if(path==='/api/materials/1'){detailPath=path;return {status:200,ok:true,json:async()=>({title:'<img onerror=bad>',original_filename:'test.md',byte_size:12,body_text:'<script>bad()</script>'})};}return originalFetch(path,options);};
 dom.window.document.querySelector('.actions button').click();await pause(10);const dialog=dom.window.document.querySelector('dialog');assert.equal(detailPath,'/api/materials/1');assert.equal(dialog.open,true);assert.equal(dialog.querySelector('pre').textContent,'<script>bad()</script>');assert.equal(dialog.querySelector('script,img'),null);dialog.querySelector('button').click();assert.equal(dom.window.document.querySelector('dialog'),null);dom.window.close();
});
test('profile password form retains exact protocol and CSRF while displaying server validation',async()=>{
 const dom=new JSDOM(html,{url:'http://localhost/profile',runScripts:'outside-only'});let submitted;
 dom.window.fetch=async(path,options)=>{if(path==='/api/password'){submitted=options;return {status:400,ok:false,json:async()=>({error:'当前密码错误'})};}return {status:200,ok:true,json:async()=>({username:'fixture',role:'teacher',class_name:'A 班',csrf_token:'test-token',must_change_password:false})};};
 dom.window.eval(script);await pause(10);const form=dom.window.document.querySelector('#password-form');form.elements.current_password.value='WrongCurrent1';form.elements.new_password.value='FixtureNext1';form.elements.confirm_password.value='FixtureNext1';form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));await pause(10);
 assert.equal(submitted.headers['X-CSRF-Token'],'test-token');assert.deepEqual(JSON.parse(submitted.body),{current_password:'WrongCurrent1',new_password:'FixtureNext1',confirm_password:'FixtureNext1'});assert.equal(form.elements.current_password.getAttribute('aria-invalid'),'true');assert.equal(form.querySelector('button').disabled,false);assert.equal(dom.window.document.querySelector('#identity a').getAttribute('href'),'/profile');dom.window.close();
});