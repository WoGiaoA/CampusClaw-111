// DOM-only functional checks. No browser, screenshots, or image artifacts.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { JSDOM } = require('../.local/ui/node_modules/jsdom');
const vm = require('node:vm');
const html = readFileSync('web/index.html', 'utf8');
const script = readFileSync('web/app.js', 'utf8');
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
async function mount(role, path = '/materials') {
 const dom = new JSDOM(html, { url: 'http://localhost' + path, runScripts: 'outside-only' });
 const calls = [];
 dom.window.fetch = async (path, options) => {
  calls.push({ path, options });
  const body = path === '/api/me'
   ? { username: 'test_user', role, class_id: 1, class_name: 'A 班', csrf_token: 'local-test-only' }
   : [{ id: 1, title: '<script>unsafe</script>', original_filename: 'a.txt', byte_size: 12, created_at: '2026-10-13T00:00:00Z' }];
  return { status: 200, ok: true, json: async () => body };
 };
 dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText; window.__renderSources = renderSources;'); await pause(30); return { dom, calls };
}

test('material deletion is teacher-only, confirms safely and cancels without DELETE', async () => {
 const {dom,calls}=await mount('teacher');const {document,HTMLDialogElement}=dom.window;
 HTMLDialogElement.prototype.showModal=function(){this.open=true;};
 HTMLDialogElement.prototype.close=function(){this.dispatchEvent(new dom.window.Event('close'));};
 const trigger=document.querySelector('.material-delete');assert.ok(trigger);trigger.focus();trigger.click();
 let dialog=document.querySelector('.delete-material-dialog');
 assert.match(dialog.textContent,/不可撤销/);assert.match(dialog.textContent,/AI 应用/);
 assert.equal(dialog.querySelector('script'),null);assert.match(dialog.textContent,/<script>unsafe<\/script>/);
 assert.equal(document.activeElement.textContent,'取消');
 [...dialog.querySelectorAll('button')].find(b=>b.textContent==='取消').click();
 assert.equal(document.querySelector('dialog'),null);assert.equal(document.activeElement,trigger);
 assert.equal(calls.filter(c=>c.options?.method==='DELETE').length,0);
 trigger.click();dialog=document.querySelector('dialog');
 const escape=new dom.window.Event('cancel',{cancelable:true});assert.equal(dialog.dispatchEvent(escape),true);
 dialog.close();assert.equal(calls.filter(c=>c.options?.method==='DELETE').length,0);dom.window.close();
 const student=await mount('student');assert.equal(student.dom.window.document.querySelector('.material-delete'),null);student.dom.window.close();
});

test('accepted deletion sends CSRF/Bearer once and distinguishes failed list refresh', async () => {
 for(const result of ['cleanup_pending','deleted']){
 const {dom,calls}=await mount('teacher');const {document,HTMLDialogElement}=dom.window;
 HTMLDialogElement.prototype.showModal=function(){this.open=true;};HTMLDialogElement.prototype.close=function(){this.dispatchEvent(new dom.window.Event('close'));};
 dom.window.sessionStorage.setItem('campusclaw_token','fixture-delete-token');let finish;
 dom.window.fetch=async(path,options)=>{calls.push({path,options});if(options?.method==='DELETE')return await new Promise(resolve=>{finish=resolve;});throw new Error('offline');};
 document.querySelector('.material-delete').click();const dialog=document.querySelector('dialog');const confirm=dialog.querySelector('.danger-button');confirm.click();confirm.click();
 assert.equal(confirm.disabled,true);assert.equal(dialog.querySelector('.secondary').disabled,true);
 const cancel=new dom.window.Event('cancel',{cancelable:true});assert.equal(dialog.dispatchEvent(cancel),false);
 const request=calls.find(c=>c.options?.method==='DELETE');assert.equal(request.path,'/api/materials/1');assert.equal(request.options.headers['X-CSRF-Token'],'local-test-only');assert.equal(request.options.headers.Authorization,'Bearer fixture-delete-token');
 assert.equal(calls.filter(c=>c.options?.method==='DELETE').length,1);
 finish({status:result==='deleted'?200:202,ok:true,json:async()=>({material_id:1,status:result})});await pause(20);
 assert.equal(document.querySelector('dialog'),null);assert.equal(document.querySelectorAll('.material').length,0);assert.equal(document.querySelector('#count').textContent,'0 份材料');
 assert.match(document.querySelector('#status').textContent,/列表刷新失败/);assert.match(document.querySelector('#status').textContent,result==='deleted'?/材料已删除/:/关联资源正在清理/);dom.window.close();
 }
});

test('rejected deletion keeps the row and permits retry; stale source gives explicit deletion error', async()=>{
 const {dom}=await mount('teacher');const {document,HTMLDialogElement}=dom.window;
 HTMLDialogElement.prototype.showModal=function(){this.open=true;};HTMLDialogElement.prototype.close=function(){this.dispatchEvent(new dom.window.Event('close'));};
 dom.window.fetch=async()=>({status:403,ok:false,json:async()=>({error:'没有删除权限'})});
 document.querySelector('.material-delete').click();document.querySelector('.danger-button').click();await pause(10);
 assert.ok(document.querySelector('.material'));assert.match(document.querySelector('dialog .status').textContent,/没有删除权限/);assert.equal(document.querySelector('.danger-button').disabled,false);
 document.querySelector('dialog').close();
 dom.window.fetch=async()=>({status:410,ok:false,json:async()=>({error:'材料已删除，原文与引用片段不可用',code:'material_deleted'})});
 await dom.window.__testShowText(1);assert.match(document.querySelector('#status').textContent,/材料已删除/);
 const target=document.createElement('div');
 dom.window.__renderSources(target,[{number:1,source_status:'material_deleted',material_id:1,chunk_text:'must not display',excerpt:'hidden'}],true,true);
 assert.match(target.textContent,/\[1\] 材料已删除/);assert.equal(target.querySelector('a,details'),null);assert.equal(target.textContent.includes('must not display'),false);dom.window.close();
});
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
 assert.match(dom.window.document.querySelector('#identity').textContent, /教师.*A 班/);
 assert.equal(dom.window.document.querySelector('input[type=file]').accept, '.txt,.md'); dom.window.close();
});
test('login is JSON POST, ordinary account/password form, no account picker', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/login', runScripts: 'outside-only' });
 dom.window.fetch = async () => ({status:200,ok:true,json:async()=>({demo_registration:true,classes:[]})});
 dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText;');
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
 dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText;');await pause(10);const form=dom.window.document.querySelector('#register-form');form.elements.username.value='new_student';form.elements.class_code.value='B';form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));await pause(10);
 assert.deepEqual(JSON.parse(calls[1].options.body),{username:'new_student',class_code:'B'});assert.match(dom.window.document.querySelector('#register-status').textContent,/123456/);assert.equal(form.hidden,true);dom.window.close();
});
test('profile shows trusted identity, first change hides current password, normal requires it',async()=>{
 for(const first of [true,false]){
 const dom=new JSDOM(html,{url:'http://localhost/profile',runScripts:'outside-only'});
 dom.window.fetch=async()=>({status:200,ok:true,json:async()=>({username:'student_test',role:'student',class_name:'A 班',must_change_password:first,csrf_token:'test-token'})});dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText;');await pause(10);
 assert.equal(dom.window.document.querySelector('#profile-identity').textContent,'学生 · A 班');
 assert.equal(Boolean(dom.window.document.getElementById('change-nickname-title')),!first);assert.equal(dom.window.document.querySelector('#open-password').hidden,first);assert.equal(Boolean(dom.window.document.querySelector('#password-form').closest('dialog')),!first);assert.equal(dom.window.document.querySelector('#current-password-label').hidden,first);assert.equal(dom.window.document.querySelector('[name=current_password]').required,!first);assert.equal(dom.window.document.querySelector('#profile-back').hidden,first);assert.equal(dom.window.document.querySelector('[name=role]'),null);assert.equal(dom.window.document.querySelector('[name=class_id]'),null);dom.window.close();}
});
test('production login hides registration and production register page refuses form',async()=>{
 for(const path of ['/login','/register']){const dom=new JSDOM(html,{url:'http://localhost'+path,runScripts:'outside-only'});dom.window.fetch=async()=>({status:200,ok:true,json:async()=>({demo_registration:false,classes:[]})});dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText;');await pause(10);assert.equal(dom.window.document.querySelector('[href="/register"]'),null);assert.equal(dom.window.document.querySelector('#register-form'),null);dom.window.close();}
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
  assert.equal(target,first?'/profile':'/workspace');assert.equal(form.elements.password.value,'');dom.window.close();
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
 dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText;');await pause(10);const form=dom.window.document.querySelector('#password-form');form.elements.current_password.value='WrongCurrent1';form.elements.new_password.value='FixtureNext1';form.elements.confirm_password.value='FixtureNext1';form.dispatchEvent(new dom.window.Event('submit',{bubbles:true,cancelable:true}));await pause(10);
 assert.equal(submitted.headers['X-CSRF-Token'],'test-token');assert.deepEqual(JSON.parse(submitted.body),{current_password:'WrongCurrent1',new_password:'FixtureNext1',confirm_password:'FixtureNext1'});assert.equal(form.elements.current_password.getAttribute('aria-invalid'),'true');assert.equal(form.querySelector('button').disabled,false);assert.equal(dom.window.document.querySelector('#identity a').getAttribute('href'),'/profile');dom.window.close();
});

test('Bearer token persists on login and is added to protected JSON and multipart requests', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/materials', runScripts: 'outside-only' }); const calls=[];
 dom.window.sessionStorage.setItem('campusclaw_token','opaque-test-token');
 dom.window.fetch=async(path,options)=>{calls.push({path,options});return {status:200,ok:true,json:async()=>path==='/api/me'?{username:'teacher',role:'teacher',class_name:'A 班',csrf_token:'csrf'}:[]};};
 dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText;');await pause(30);
 assert.equal(calls[0].options.headers.Authorization,'Bearer opaque-test-token');assert.equal(calls[1].options.headers.Authorization,'Bearer opaque-test-token');
 const form=dom.window.document.querySelector('#upload-form');form.dispatchEvent(new dom.window.Event('submit',{cancelable:true}));await pause(30);
 const upload=calls.find(c=>c.path==='/api/materials/upload');assert.equal(upload.options.headers.Authorization,'Bearer opaque-test-token');assert.equal(upload.options.headers['X-CSRF-Token'],'csrf');assert.equal(upload.options.headers['Content-Type'],undefined);dom.window.close();
});

test('login stores returned token and 401 clears it',async()=>{
 const dom=new JSDOM(html,{url:'http://localhost/login',runScripts:'outside-only'});
 dom.window.fetch=async path=>({status:200,ok:true,json:async()=>path==='/api/login'?{access_token:'new-token',must_change_password:false}:{demo_registration:false}});
 dom.window.eval(script + '\nwindow.__testAPI = api; window.__testShowText = showText;');await pause(20);const form=dom.window.document.querySelector('#login-form');form.elements.username.value='teacher';form.elements.password.value='private';form.dispatchEvent(new dom.window.Event('submit',{cancelable:true}));await pause(20);assert.equal(dom.window.sessionStorage.getItem('campusclaw_token'),'new-token');
 dom.window.fetch=async()=>({status:401,ok:false,json:async()=>({error:'请先登录'})});await dom.window.__testAPI('/api/materials').catch(()=>{});assert.equal(dom.window.sessionStorage.getItem('campusclaw_token'),null);dom.window.close();
});

test('RAG submits question, renders answer and citations as safe text, restores buttons',async()=>{
 const {dom,calls}=await mount('student');dom.window.sessionStorage.setItem('campusclaw_token','rag-token');
 dom.window.fetch=async(path,options)=>{calls.push({path,options});return {status:200,ok:true,json:async()=>({answer:'项目占40% [1] <img src=x>',citations:[{number:1,material_id:1,chunk_id:7,material_title:'<script>unsafe</script>',chunk_index:1,char_start:0,char_end:20,excerpt:'正文 <img src=x>',source_url:'/materials?material_id=1&chunk_id=7'}]})};};
 const form=dom.window.document.querySelector('#ask-form');form.elements.question.value='期末项目占多少？';form.dispatchEvent(new dom.window.Event('submit',{cancelable:true}));await pause(20);
 const call=calls.at(-1);assert.equal(call.path,'/api/ask');assert.deepEqual(JSON.parse(call.options.body),{question:'期末项目占多少？'});assert.equal(call.options.headers.Authorization,'Bearer rag-token');
 assert.match(dom.window.document.querySelector('#rag-answer').textContent,/40%.*<img/);assert.equal(dom.window.document.querySelector('#rag-answer img'),null);assert.equal(dom.window.document.querySelector('#rag-sources script'),null);assert.match(dom.window.document.querySelector('#rag-sources a').textContent,/\[1\]/);assert.equal(form.querySelector('button').disabled,false);dom.window.close();
});

test('search defaults hybrid and sends selected modes; empty versus dependency error differ',async()=>{
 const {dom,calls}=await mount('student');let fail=false;
 dom.window.fetch=async(path,options)=>{calls.push({path,options});return {status:fail?503:200,ok:!fail,json:async()=>fail?{error:'模型服务暂不可用'}:{hits:[],message:'资料中未找到相关内容'}};};
 const form=dom.window.document.querySelector('#ask-form');form.elements.question.value='检索问题';assert.equal(form.elements.mode.value,'hybrid');
 for(const mode of ['hybrid','keyword','vector']) {form.elements.mode.value=mode;dom.window.document.querySelector('#search-only').click();await pause(15);assert.equal(calls.at(-1).path,'/api/search');assert.equal(JSON.parse(calls.at(-1).options.body).mode,mode);assert.match(dom.window.document.querySelector('#rag-status').textContent,/未找到/);}
 fail=true;dom.window.document.querySelector('#search-only').click();await pause(15);assert.match(dom.window.document.querySelector('#rag-status').textContent,/暂不可用/);assert.equal(form.querySelector('button').disabled,false);dom.window.close();
});

test('citation opens protected chunk and original; teacher rebuild includes CSRF and Bearer',async()=>{
 const {dom,calls}=await mount('teacher');dom.window.sessionStorage.setItem('campusclaw_token','source-token');
 dom.window.HTMLDialogElement.prototype.showModal=function(){this.open=true;};dom.window.HTMLDialogElement.prototype.close=function(){this.dispatchEvent(new dom.window.Event('close'));};
 dom.window.fetch=async(path,options)=>{calls.push({path,options});const data=path.endsWith('/reindex')?{index_status:'pending'}:path.includes('/chunks/')?{chunk_index:2,char_start:720,char_end:850,chunk_text:'引用原文😀'}:{id:1,title:'讲义',original_filename:'a.txt',byte_size:30,body_text:'完整原文',index_status:'ready',chunk_count:2};return {status:200,ok:true,json:async()=>data};};
 await dom.window.__testShowText(1, 7);assert.ok(calls.some(c=>c.path==='/api/materials/1/chunks/7'));assert.match(dom.window.document.querySelector('dialog').textContent,/720, 850.*引用原文😀.*完整原文/s);
 const rebuild=[...dom.window.document.querySelectorAll('dialog button')].find(b=>b.textContent==='重建索引');rebuild.click();await pause(20);const call=calls.find(c=>c.path.endsWith('/reindex'));assert.equal(call.options.headers.Authorization,'Bearer source-token');assert.equal(call.options.headers['X-CSRF-Token'],'local-test-only');dom.window.close();
});

test('download uses authenticated fetch and Blob, never token in URL',async()=>{
 const {dom,calls}=await mount('student');dom.window.sessionStorage.setItem('campusclaw_token','download-token');let blobCreated=false;
 dom.window.URL.createObjectURL=()=>{blobCreated=true;return 'blob:local-test'};dom.window.URL.revokeObjectURL=()=>{};dom.window.HTMLAnchorElement.prototype.click=function(){};
 dom.window.fetch=async(path,options)=>{calls.push({path,options});return {status:200,ok:true,blob:async()=>new dom.window.Blob(['bytes'])};};
 dom.window.document.querySelector('.material a').dispatchEvent(new dom.window.MouseEvent('click',{cancelable:true}));await pause(20);assert.equal(calls.at(-1).options.headers.Authorization,'Bearer download-token');assert.equal(new URL(calls.at(-1).path).search,'');assert.equal(blobCreated,true);dom.window.close();
});

test('workspaces expose role-specific navigation and reject cross-role routes before business calls', async () => {
 for (const [role, own, forbidden] of [['teacher', '/applications', '/assistant'], ['student', '/assistant', '/applications']]) {
  const home = await mount(role, '/workspace');
  const nav = home.dom.window.document.querySelector('.work-nav');
  assert.equal(nav.querySelectorAll('a').length, 6);
  assert.ok(nav.querySelector('a[href="' + own + '"]'));
  assert.equal(nav.querySelector('a[href="' + forbidden + '"]'), null);
  assert.equal(nav.querySelector('[aria-current=page]').getAttribute('href'), '/workspace');
  assert.equal(nav.querySelectorAll('svg.line-icon').length, 6);
  assert.deepEqual(home.calls.map(call => call.path), role === 'teacher' ? ['/api/me', '/api/ai/apps', '/api/mcp/servers', '/api/materials'] : ['/api/me']);
  if (role === 'teacher') assert.match(home.dom.window.document.querySelector('.home-overview').textContent, /1 份.*应用读取失败.*工具读取失败/);
  else assert.ok(home.dom.window.document.querySelector('.home-actions a[href="/assistant"]'));
  home.dom.window.close();
  const denied = await mount(role, forbidden);
  assert.match(denied.dom.window.document.querySelector('h1').textContent, /不可访问/);
  assert.deepEqual(denied.calls.map(call => call.path), ['/api/me']);
  denied.dom.window.close();
 }
});

test('teacher saves a real application prompt and tool allowlist', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/applications', runScripts: 'outside-only' });
 const calls = [];
 const application = { id: 4, name: '解题引导', enabled: true, role_prompt: '旧指令', template_id: 3, skill_ids: [], material_ids: [] };
 dom.window.fetch = async (path, options) => {
  calls.push({ path, options });
  const body = {
   '/api/me': { username: 'teacher', role: 'teacher', class_name: 'A 班', csrf_token: 'csrf' },
   '/api/ai/apps': { items: [application] },
   '/api/ai/prompt-templates': { items: [{ id: 3, name: '共享模板', body: '模板正文' }] },
   '/api/ai/skills': { items: [] }, '/api/materials': [],
   '/api/mcp/servers': { items: [], configured: true },
   '/api/ai/apps/4/tool-grants': { items: [] }
  }[path];
  if (path === '/api/ai/apps/4') Object.assign(application, JSON.parse(options.body));
  return { status: 200, ok: true, json: async () => path === '/api/ai/apps/4' ? application : body };
 };
 dom.window.eval(script);
 await pause(30);
 const doc = dom.window.document;
 const tabs = doc.querySelectorAll('.config-tab');
 tabs[1].click();
 doc.querySelector('.config-controls textarea').value = '新指令';
 doc.querySelector('.config-controls textarea').dispatchEvent(new dom.window.Event('input', { bubbles: true }));
 doc.querySelector('.config-controls .primary').click();
 await pause(10);
 assert.equal(calls.at(-1).options.headers['X-CSRF-Token'], 'csrf');
 assert.deepEqual(JSON.parse(calls.at(-1).options.body), { role_prompt: '新指令', template_id: 3 });
 assert.equal(doc.querySelector('.config-controls textarea').value, '新指令');
 tabs[4].click();
 await pause(10);
 doc.querySelector('.config-controls input[type=checkbox]').click();
 [...doc.querySelectorAll('.config-controls button')].find(button => button.textContent === '保存工具授权').click();
 await pause(10);
 const saved = calls.find(call => call.path === '/api/ai/apps/4/tool-grants' && call.options?.method === 'PUT');
 assert.deepEqual(JSON.parse(saved.options.body), { refs: ['knowledge_search'] });
 dom.window.close();
 for (const path of ['/assignments/manage']) {
  const page = await mount('teacher', path);
  assert.match(page.dom.window.document.querySelector('.pending-panel').textContent, /暂未开放/);
  assert.deepEqual(page.calls.map(call => call.path), ['/api/me']);
  page.dom.window.close();
 }
});

test('student sees incremental answer, then a switch discards late events', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/assistant', runScripts: 'outside-only' });
 const calls = [];
 const conversation = { id: 8, app_id: 4, app_name: '解题引导', title: '历史会话' };
 const enc = new TextEncoder();
 dom.window.TextDecoder = TextDecoder;
 let push;
 const reader = {
  read: () => new Promise(resolve => { push = text => resolve({ value: enc.encode(text), done: false }); }),
  cancel: async () => {}
 };
 dom.window.fetch = async (path, options) => {
  calls.push({ path, options });
  const data = {
   '/api/me': { username: 'student', role: 'student', csrf_token: 'csrf' },
   '/api/ai/apps': { items: [{ id: 4, name: '解题引导' }] },
   '/api/ai/conversations': options?.method === 'POST' ? conversation : { items: [conversation] },
   '/api/ai/conversations/8': { conversation, items: [] }
  }[path];
  if (path.endsWith('/messages')) return { ok: true, headers: new Headers({ 'Content-Type': 'text/event-stream' }), body: { getReader: () => reader } };
  return { ok: true, status: 200, json: async () => data };
 };
 dom.window.eval(script);
 await pause(30);
 const doc = dom.window.document;
 doc.querySelector('.suggestions button').click();
 const form = doc.querySelector('#ask-form');
 assert.match(form.elements.question.value, /期末项目/);
 assert.equal(calls.length, 3);
 form.dispatchEvent(new dom.window.Event('submit', { cancelable: true }));
 await pause(10);
 const send = calls.at(-1);
 assert.match(send.path, /\/messages$/);
 assert.equal(send.options.headers['X-CSRF-Token'], 'csrf');
 push('event: meta\ndata: {"turn_id":1,"retrieval_status":"unbound","skill":{"name":"解题引导"}}\n\nevent: tool\ndata: {"turn_id":1,"attempt_id":"a1","tool":"formula_hint@campus-tools","status":"success","summary":"返回了提示"}\n\nevent: delta\ndata: {"turn_id":1,"text":"逐段显示"}\n\n');
 await pause(10);
 assert.equal(doc.querySelector('.conversation-turn .rag-answer').textContent, '逐段显示');
 assert.match(doc.querySelector('.conversation-turn details').textContent, /formula_hint@campus-tools.*成功/);
 assert.equal(doc.querySelector('.application-picker select').disabled, true);
 doc.querySelector('.reset-chat').click();
 assert.equal(send.options.signal.aborted, true);
 push('event: delta\ndata: {"turn_id":1,"text":"迟到"}\n\nevent: done\ndata: {"turn_id":1,"citations":[]}\n\n');
 await pause(10);
 assert.equal(doc.querySelector('.conversation-turn'), null);
 assert.ok(doc.querySelector('.chat-empty'));
 dom.window.close();
});


test('legacy source deep link requests the protected material and exact chunk', async () => {
 const path = '/materials?material_id=1&chunk_id=7';
 const dom = new JSDOM(html, { url: 'http://localhost' + path, runScripts: 'outside-only' });
 const calls = [];
 dom.window.HTMLDialogElement.prototype.showModal = function () { this.open = true; };
 dom.window.fetch = async url => {
  calls.push(url);
  const body = {
   '/api/me': { username: 'reader', role: 'student', class_name: 'A 班' },
   '/api/materials': [],
   '/api/materials/1': { title: '材料', original_filename: 'a.txt', byte_size: 8, body_text: '全文' },
   '/api/materials/1/chunks/7': { chunk_index: 2, char_start: 0, char_end: 2, chunk_text: '切片' }
  }[url];
  return { status: 200, ok: true, json: async () => body };
 };
 dom.window.eval(script);
 await pause(30);
 assert.deepEqual(calls, ['/api/me', '/api/materials', '/api/materials/1', '/api/materials/1/chunks/7']);
 assert.match(dom.window.document.querySelector('dialog').textContent, /切片 2.*切片.*全文/s);
 dom.window.close();
});

test('nickname uses the account API and prefills the previous browser nickname only before first save', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/profile', runScripts: 'outside-only' });
 const { document, HTMLDialogElement, Event } = dom.window;
 HTMLDialogElement.prototype.showModal = function () { this.open = true; };
 HTMLDialogElement.prototype.close = function () { this.open = false; this.dispatchEvent(new Event('close')); };
 dom.window.localStorage.setItem('campusclaw.nickname:7', '<林老师>');
 const calls = [];
 dom.window.fetch = async (path, options) => {
  calls.push({ path, options });
  const body = options.method === 'PATCH' ? JSON.parse(options.body)
   : { id: 7, username: 'test_user', role: 'teacher', class_name: 'A 班', csrf_token: 'fixture', nickname: null };
  return { status: 200, ok: true, json: async () => body };
 };
 dom.window.eval(script);
 await pause(30);
 assert.equal(document.querySelector('.identity-name'), null);
 const dialog = document.getElementById('change-nickname-title').closest('dialog');
 const form = dialog.querySelector('form');
 const open = () => document.querySelector('.profile-actions .profile-outline').click();
 const submit = () => form.dispatchEvent(new Event('submit', { cancelable: true }));
 open();
 assert.equal(form.elements.nickname.value, '<林老师>');
 submit();
 await pause(10);
 assert.equal(calls[1].path, '/api/me');
 assert.equal(calls[1].options.method, 'PATCH');
 assert.equal(calls[1].options.headers['X-CSRF-Token'], 'fixture');
 assert.deepEqual(JSON.parse(calls[1].options.body), { nickname: '<林老师>' });
 assert.equal(document.querySelector('.identity-group .identity-name').textContent, '<林老师>');
 assert.equal(dom.window.localStorage.getItem('campusclaw.nickname:7'), null);
 open();
 form.elements.nickname.value = '';
 submit();
 await pause(10);
 dom.window.localStorage.setItem('campusclaw.nickname:7', '过期昵称');
 open();
 assert.equal(form.elements.nickname.value, '');
 assert.equal(document.querySelector('.identity-name'), null);
 dom.window.close();
});

test('teacher overview reports a failed material lookup without a fictional count', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/workspace', runScripts: 'outside-only' });
 dom.window.fetch = async path => path === '/api/me'
  ? { status: 200, ok: true, json: async () => ({ role: 'teacher', class_name: 'A 班' }) }
  : { status: 503, ok: false, json: async () => ({ error: '服务暂不可用' }) };
 dom.window.eval(script);
 await pause(30);
 const count = dom.window.document.querySelector('.overview-card [role=status]');
 assert.equal(count.textContent, '资料读取失败');
 assert.equal(count.dataset.tone, 'warning');
 dom.window.close();
});

test('restored citations expand independently and keep original source links', async () => {
 const dom = new JSDOM(html, { url: 'http://localhost/assistant', runScripts: 'outside-only' });
 const conversation = { id: 8, app_id: 4, app_name: '解题引导', title: '历史会话' };
 const hit = { material_title: '课程讲义', material_id: 1, chunk_id: 2, chunk_index: 3, char_start: 10, char_end: 80, excerpt: '摘要', chunk_text: '完整片段' };
 const bodies = {
  '/api/me': { username: 'student', role: 'student', csrf_token: 'csrf' },
  '/api/ai/apps': { items: [{ id: 4, name: '解题引导' }] },
  '/api/ai/conversations': { items: [conversation] },
  '/api/ai/conversations/8': { conversation, items: [{ status: 'completed', messages: [
   { role: 'user', body: '资料问题' }, { role: 'assistant', body: '回答', metadata: { citations: [hit, { ...hit, chunk_id: 4 }] } }
  ] }] }
 };
 dom.window.fetch = async path => ({ status: 200, ok: true, json: async () => bodies[path] });
 dom.window.eval(script);
 await pause(25);
 const doc = dom.window.document;
 doc.querySelector('.session-list button').click();
 await pause(10);
 const items = [...doc.querySelectorAll('.compact-source')];
 assert.equal(items.length, 2);
 assert.equal(items[0].querySelector('a').getAttribute('href'), '/materials?material_id=1&chunk_id=2');
 const [first, second] = items.map(item => item.querySelector('details'));
 first.querySelector('summary').click();
 await pause(10);
 assert.equal(first.open, true);
 assert.equal(second.open, false);
 assert.match(first.textContent, /原文字符 \[10, 80\).*完整片段/);
 dom.window.close();
});

async function interactionPage(path='/applications', role='teacher') {
 const dom=new JSDOM(html,{url:'http://localhost'+path,runScripts:'outside-only'});
 dom.window.HTMLDialogElement.prototype.showModal=function(){this.open=true;};
 dom.window.HTMLDialogElement.prototype.close=function(){this.open=false;this.dispatchEvent(new dom.window.Event('close'));};
 const calls=[];
 const application={id:4,name:'资料应用',enabled:true,role_prompt:'旧指令',template_id:3,skill_ids:[],material_ids:[1]};
 const bodies={
  '/api/me':{username:role,role,class_name:'A 班',csrf_token:'csrf'},
  '/api/ai/apps':{items:[application]},
  '/api/ai/prompt-templates':{items:[{id:3,name:'共享模板',body:'共享正文'}]},
  '/api/ai/skills':{items:[{id:9,name:'测试技能',enabled:true,conditions:'测试',procedure:'提示',raw_text:'原文',tool_refs:[]}]},
  '/api/materials':[{id:1,title:'甲资料'},{id:2,title:'乙资料'},{id:3,title:'丙资料'}],
  '/api/mcp/servers':{items:[],configured:true},
  '/api/ai/conversations':{items:[{id:8,app_id:4,app_name:'资料应用',title:'历史会话'}]},
  '/api/ai/conversations/8':{conversation:{id:8,app_id:4,app_name:'资料应用',title:'历史会话'},items:[]}
 };
 let intercept;
 dom.window.fetch=async(path,options)=>{
  calls.push({path,options});
  const custom=await intercept?.(path,options); if(custom)return custom;
  if(path==='/api/ai/apps/4'){Object.assign(application,JSON.parse(options.body));return{status:200,ok:true,json:async()=>application};}
  return{status:200,ok:true,json:async()=>bodies[path]};
 };
 dom.window.eval(script);await pause(25);
 return{dom,calls,bodies,intercept:fn=>{intercept=fn;},doc:dom.window.document};
}
const clickText=(doc,text)=>[...doc.querySelectorAll('button')].find(b=>b.textContent===text).click();
const editValue=(dom,node,value)=>{node.value=value;node.dispatchEvent(new dom.window.Event('input',{bubbles:true}));};

test('configuration disables unchanged saves and preserves drafts/errors on rejection',async()=>{
 const {dom,doc,calls,intercept}=await interactionPage();
 const save=[...doc.querySelectorAll('button')].find(b=>b.textContent==='保存配置');assert.equal(save.disabled,true);
 editValue(dom,doc.querySelector('.config-controls input'),'新应用名');assert.equal(save.disabled,false);
 intercept(async(path,options)=>options?.method==='PATCH'?{status:503,ok:false,json:async()=>({error:'保存暂不可用'})}:null);
 save.click();await pause(15);assert.equal(save.disabled,false);assert.equal(doc.querySelector('.config-controls input').value,'新应用名');assert.match(doc.querySelector('.app-config > .feedback').textContent,/保存暂不可用/);assert.equal(doc.querySelector('#global-toast'),null);
 clickText(doc,'知识库绑定');await pause(5);assert.ok(doc.querySelector('dialog'));clickText(doc,'取消');await pause(5);assert.equal(doc.querySelector('.config-controls input').value,'新应用名');
 clickText(doc,'知识库绑定');await pause(5);clickText(doc,'放弃修改');await pause(5);assert.ok(doc.querySelector('input[type=search]'));assert.equal(calls.filter(c=>c.options?.method==='PATCH').length,1);dom.window.close();
});

test('filtered material select-all preserves hidden selection and submits the full set',async()=>{
 const {dom,doc,calls}=await interactionPage();clickText(doc,'知识库绑定');
 const query=doc.querySelector('input[type=search]');editValue(dom,query,'乙');clickText(doc,'全选筛选结果');
 assert.match(doc.querySelector('.binding-toolbar').textContent,/2 项已选择/);
 editValue(dom,query,'丙');assert.equal(doc.querySelector('.binding-row input').checked,false);
 clickText(doc,'保存资料绑定');await pause(15);
 const request=calls.find(c=>c.options?.method==='PATCH');assert.deepEqual(JSON.parse(request.options.body),{material_ids:[1,2]});assert.equal(request.options.headers['X-CSRF-Token'],'csrf');
 assert.match(doc.querySelector('#global-toast').textContent,/操作已保存/);assert.equal([...doc.querySelectorAll('button')].find(b=>b.textContent==='保存资料绑定').disabled,true);dom.window.close();
});

test('template creation does not silently bind and application save retains a template draft',async()=>{
 const {dom,doc,calls,intercept}=await interactionPage();clickText(doc,'提示词模板');
 assert.equal(doc.querySelectorAll('.config-section').length,2);assert.match(doc.querySelectorAll('.config-section')[1].textContent,/所有绑定/);
 const fields=doc.querySelectorAll('textarea');editValue(dom,fields[1],'共享草稿');editValue(dom,fields[0],'应用草稿');
 clickText(doc,'保存应用提示词');await pause(15);assert.equal(fields[1].value,'共享草稿');
 intercept(async(path,options)=>path==='/api/ai/prompt-templates'&&options?.method==='POST'?{status:201,ok:true,json:async()=>({id:10,name:'新模板',body:'共享草稿'})}:null);
 clickText(doc,'另存为新模板');await pause(15);assert.equal(doc.querySelector('.config-section select').value,'3');assert.equal(calls.filter(c=>c.path==='/api/ai/apps/4').length,1);dom.window.close();
});

test('enabled skill and current application binding are independent states',async()=>{
 const {dom,doc,calls}=await interactionPage();clickText(doc,'Skill 技能');
 assert.match(doc.querySelector('.binding-row').textContent,/已启用 · 未绑定/);assert.equal(doc.querySelector('.binding-row input').checked,false);
 const sw=doc.querySelector('input[role=switch]');assert.equal(sw.checked,true);assert.equal(sw.closest('label').firstChild,sw);
 doc.querySelector('.binding-row input').click();clickText(doc,'保存 Skill 绑定');await pause(15);
 assert.deepEqual(JSON.parse(calls.find(c=>c.options?.method==='PATCH').options.body),{skill_ids:[9]});assert.equal(sw.checked,true);
 assert.ok([...doc.querySelectorAll('summary')].some(v=>v.textContent==='上传新 Skill'));dom.window.close();
});

test('create application uses a real POST, starts disabled and selects returned configuration',async()=>{
 const {dom,doc,calls,intercept}=await interactionPage();
 intercept(async(path,options)=>path==='/api/ai/apps'&&options?.method==='POST'?{status:201,ok:true,json:async()=>({id:12,name:'新助手',enabled:false,role_prompt:'',template_id:null,skill_ids:[],material_ids:[]})}:null);
 clickText(doc,'+ 新建');await pause(5);editValue(dom,doc.querySelector('dialog input'),'新助手');clickText(doc,'创建应用');await pause(15);
 const request=calls.find(c=>c.path==='/api/ai/apps'&&c.options?.method==='POST');assert.deepEqual(JSON.parse(request.options.body),{name:'新助手'});assert.equal(request.options.headers['X-CSRF-Token'],'csrf');assert.equal(doc.querySelector('dialog'),null);assert.equal(doc.querySelector('.config-header h2').textContent,'新助手');assert.equal(doc.querySelector('input[role=switch]').checked,false);dom.window.close();
});

test('conversation deletion confirms, preserves failure, and resets only the deleted active session',async()=>{
 const {dom,doc,calls,intercept,bodies}=await interactionPage('/assistant','student');
 doc.querySelector('.session-open').click();await pause(10);assert.equal(doc.querySelector('.session-row').dataset.active,'true');
 doc.querySelector('.session-menu').click();assert.equal(doc.activeElement.textContent,'取消');clickText(doc,'取消');assert.equal(calls.some(c=>c.options?.method==='DELETE'),false);
 let reject=true;intercept(async(path,options)=>{
  if(options?.method!=='DELETE')return;
  if(reject)return{status:409,ok:false,json:async()=>({error:'此会话正在生成'})};
  bodies['/api/ai/conversations']={items:[]};return{status:200,ok:true,json:async()=>({status:'deleted'})};
 });
 doc.querySelector('.session-menu').click();clickText(doc,'删除会话');await pause(10);assert.ok(doc.querySelector('.session-row'));assert.match(doc.querySelector('dialog .feedback').textContent,/正在生成/);
 reject=false;clickText(doc,'删除会话');await pause(15);assert.equal(doc.querySelector('dialog'),null);assert.equal(doc.querySelector('.session-row'),null);assert.ok(doc.querySelector('.chat-empty'));assert.equal(doc.querySelector('.application-picker select').disabled,false);
 assert.equal(calls.find(c=>c.options?.method==='DELETE').options.headers['X-CSRF-Token'],'csrf');dom.window.close();
});

test('teacher overview counts enabled apps and connected services from API data',async()=>{
 const {dom,doc}=await interactionPage('/workspace');
 assert.match(doc.querySelector('.home-overview').textContent,/1 个已启用/);assert.match(doc.querySelector('.home-overview').textContent,/暂无已连接服务/);dom.window.close();
});

test('application toggle saves full disable; app deletion confirms and preserves failures',async()=>{
 const {dom,doc,calls,intercept}=await interactionPage();
 const toggle=doc.querySelector('input[role=switch]');assert.match(toggle.closest('label').textContent,/启用应用/);
 assert.match(doc.querySelector('.config-section').textContent,/仅可查看历史/);
 toggle.click();clickText(doc,'保存配置');await pause(10);
 assert.equal(JSON.parse(calls.find(c=>c.options?.method==='PATCH').options.body).enabled,false);
 clickText(doc,'删除应用');await pause(5);clickText(doc,'取消');assert.equal(calls.some(c=>c.options?.method==='DELETE'),false);
 let rejected=true;intercept(async(path,options)=>options?.method==='DELETE'?{status:rejected?409:200,ok:!rejected,json:async()=>rejected?{error:'仍有会话正在生成'}:{status:'deleted'}}:null);
 clickText(doc,'删除应用');await pause(5);clickText(doc,'确认删除');await pause(10);assert.ok(doc.querySelector('.app-choice'));assert.match(doc.querySelector('dialog .feedback').textContent,/正在生成/);
 rejected=false;clickText(doc,'确认删除');await pause(10);assert.equal(doc.querySelector('dialog'),null);assert.equal(doc.querySelector('.app-choice'),null);assert.match(doc.querySelector('.config-header h2').textContent,/创建第一个/);
 assert.equal(calls.find(c=>c.options?.method==='DELETE').options.headers['X-CSRF-Token'],'csrf');dom.window.close();
});

test('disabled application history is read-only and focus refresh revokes a stale composer',async()=>{
 const {dom,doc,calls,bodies}=await interactionPage('/assistant','student');
 bodies['/api/ai/conversations/8'].conversation.app_enabled=false;
 doc.querySelector('.session-open').click();await pause(10);
 assert.equal(doc.querySelector('#ask-form button.primary').disabled,true);assert.equal(doc.querySelector('#ask-form textarea').disabled,true);
 assert.match(doc.querySelector('.application-readonly-notice').textContent,/仅可查看历史/);
 doc.querySelector('#ask-form').elements.question.value='不应发送';doc.querySelector('#ask-form').dispatchEvent(new dom.window.Event('submit',{cancelable:true}));await pause(10);
 assert.equal(calls.some(c=>c.path.endsWith('/messages')),false);
 // Back to an enabled history; revisiting after teacher disable refreshes availability.
 bodies['/api/ai/conversations/8'].conversation.app_enabled=true;
 doc.querySelector('.session-open').click();await pause(10);assert.equal(doc.querySelector('#ask-form textarea').disabled,false);
 bodies['/api/ai/apps']={items:[]};Object.defineProperty(doc,'hidden',{configurable:true,value:false});dom.window.dispatchEvent(new dom.window.Event('focus'));await pause(10);
 assert.equal(doc.querySelector('#ask-form textarea').disabled,true);assert.equal(doc.querySelector('.application-picker select option').textContent.includes('已停用'),true);dom.window.close();
});

test('legacy assistant tool syntax is hidden while the original user question remains intact',async()=>{
 const {dom,doc,bodies}=await interactionPage('/assistant','student');
 const marker='<|FunctionCallBegin|>[{"name":"formula_hint"}]<|FunctionCallEnd|>';
 bodies['/api/ai/conversations/8'].items=[{status:'completed',messages:[{role:'user',body:marker},{role:'assistant',body:marker}]}];
 doc.querySelector('.session-open').click();await pause(10);
 assert.equal(doc.querySelector('.chat-question').textContent,marker);
 assert.match(doc.querySelector('.rag-answer').textContent,/未执行.*已隐藏/);
 assert.equal(doc.querySelector('.rag-answer').textContent.includes('FunctionCall'),false);
 assert.equal(doc.querySelector('.chat-capabilities'),null);dom.window.close();
});
