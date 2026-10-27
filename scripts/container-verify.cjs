'use strict';
// Real Nginx/Go/MySQL HTTP acceptance against an isolated Compose project.
// Passwords/cookies are kept only in ignored .local state, never in evidence.
const fs=require('node:fs'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const base=process.env.VERIFY_URL||'http://127.0.0.1:18080';
const phase=process.argv[2]||'first';
const output='deliverables/lesson-03/container-http-'+phase+'.json';
const stateFile='.local/container-verification-state.json';
const checks=[];
const sha=data=>crypto.createHash('sha256').update(data).digest('hex');
const freshPassword=()=> 'Classroom9'+crypto.randomBytes(10).toString('hex');
async function call(name,path,{method='GET',body,cookie,csrf,form}={}){
 const headers={};if(body)headers['Content-Type']='application/json';if(cookie)headers.Cookie=cookie;if(csrf)headers['X-CSRF-Token']=csrf;
 const response=await fetch(base+path,{method,headers,body:form|| (body?JSON.stringify(body):undefined)});
 const raw=Buffer.from(await response.arrayBuffer());const json=(response.headers.get('content-type')||'').includes('json');const result=json?JSON.parse(raw.toString()):raw;
 const recordedHeaders=Object.fromEntries(response.headers);if(recordedHeaders['set-cookie'])recordedHeaders['set-cookie']=recordedHeaders['set-cookie'].replace(/^[^;]+/,'campusclaw_session=[REDACTED]');
 const safeBody=json?{...result}: {bytes:raw.length,sha256:sha(raw)};if(Array.isArray(result)){checks.push({name,method,url:base+path,status:response.status,headers:recordedHeaders,body:result});}
 else{delete safeBody.csrf_token;checks.push({name,method,url:base+path,status:response.status,headers:recordedHeaders,body:safeBody});}
 return {status:response.status,body:result,raw,cookie:response.headers.get('set-cookie')?.split(';')[0]};
}
function status(value,expected){assert.equal(value.status,expected);return value;}
async function login(name,password){return call('login:'+name,'/api/login',{method:'POST',body:{username:name,password}});}
async function identity(c){const me=status(await call('identity','/api/me',{cookie:c}),200);return me.body;}
async function change(c,csrf,current,password,confirm=password,extras={}){return call('password-change','/api/password',{method:'POST',cookie:c,csrf,body:{current_password:current,new_password:password,confirm_password:confirm,...extras}});}
async function upload(c,csrf,filename,data,extras={}){const form=new FormData();form.append('file',new Blob([data]),filename);for(const [key,value]of Object.entries(extras))form.append(key,value);return call('upload:'+filename,'/api/materials/upload',{method:'POST',cookie:c,csrf,form});}
(async()=>{
 const names=['teacher_a','teacher_b','student_a1','student_b1'];
 let state=phase==='first'?{passwords:{}}:JSON.parse(fs.readFileSync(stateFile,'utf8'));
 const people={};
 for(const page of ['/login','/register','/profile','/materials'])status(await call('page:'+page,page),200);
 const health=status(await call('health','/health'),200);assert.deepEqual(health.body,{status:'ok'});
 status(await call('anonymous-me','/api/me'),401);
 if(phase==='first'){
  const wrong=status(await login('teacher_a','wrong-password'),401);const unknown=status(await login('unknown_account','wrong-password'),401);assert.deepEqual(wrong.body,unknown.body);
  for(const name of names){
   const r=status(await login(name,'123456'),200);assert.equal(r.body.must_change_password,true);const u=await identity(r.cookie);
   const blocked=status(await call('initial-business-block','/api/materials',{cookie:r.cookie}),403);assert.equal(blocked.body.code,'password_change_required');
   const pw=freshPassword();status(await change(r.cookie,u.csrf_token,'',pw),200);status(await call('changed-cookie-replay','/api/me',{cookie:r.cookie}),401);status(await login(name,'123456'),401);state.passwords[name]=pw;
  }
 }
 for(const name of names){const r=status(await login(name,state.passwords[name]),200);assert.equal(r.body.must_change_password,false);people[name]={cookie:r.cookie,user:await identity(r.cookie)};}
 const a=people.teacher_a,b=people.teacher_b,sa=people.student_a1,sb=people.student_b1;
 const listA=status(await call('A-material-list','/api/materials',{cookie:a.cookie}),200).body;
 const listB=status(await call('B-material-list','/api/materials',{cookie:b.cookie}),200).body;
 assert(listA.every(m=>m.class_id===a.user.class_id));assert(listB.every(m=>m.class_id===b.user.class_id));
 const own=listA[0].id,other=listB[0].id;
 status(await call('anonymous-download',`/api/materials/${own}/file`),401);
 const ownFile=status(await call('student-own-download',`/api/materials/${own}/file`,{cookie:sa.cookie}),200);
 const teacherFile=status(await call('teacher-own-download',`/api/materials/${own}/file`,{cookie:a.cookie}),200);assert.equal(sha(ownFile.raw),sha(teacherFile.raw));
 const cross=status(await call('teacher-cross-download',`/api/materials/${other}/file`,{cookie:a.cookie}),404);
 const missing=status(await call('missing-download','/api/materials/999999999/file',{cookie:a.cookie}),404);assert.deepEqual(cross.body,missing.body);
 status(await call('student-cross-download',`/api/materials/${other}/file`,{cookie:sa.cookie}),404);
 status(await call('cross-detail',`/api/materials/${other}`,{cookie:sa.cookie}),404);
 status(await call('unprotected-storage-path','/uploads/1/guessed.txt'),404);
 if(phase==='first'){
  status(await upload(sa.cookie,sa.user.csrf_token,'student.txt','denied'),403);
  status(await upload(a.cookie,undefined,'csrf.txt','denied'),403);
  for(const [name,data,expected]of [['bad.pdf','invalid',400],['bad.docx','invalid',400],['bad.png','invalid',400],['bad.txt',Buffer.from([255]),400],['empty.md',' ',400],['large.txt',Buffer.alloc((5<<20)+1,65),413],['nginx-large.txt',Buffer.alloc((6<<20)+1,65),413]])status(await upload(a.cookie,a.user.csrf_token,name,data),expected);
  const after=status(await call('failed-upload-no-dirty-record','/api/materials',{cookie:a.cookie}),200);assert.equal(after.body.length,listA.length);
  const text='第三课容器验收：真实 Nginx、Go、MySQL 与持久化。\n';
  const uploaded=status(await upload(a.cookie,a.user.csrf_token,'container-acceptance.txt',text,{class_id:String(b.user.class_id)}),201);
  const detail=status(await call('uploaded-student-detail',`/api/materials/${uploaded.body.material_id}`,{cookie:sa.cookie}),200);assert.equal(detail.body.class_id,a.user.class_id);assert.equal(detail.body.body_text,text);
  const knowledge=status(await call('uploaded-knowledge',`/api/knowledge-entries/${uploaded.body.knowledge_entry_id}`,{cookie:a.cookie}),200);assert.equal(knowledge.body.body_text,text);
  const file=status(await call('uploaded-student-file',`/api/materials/${uploaded.body.material_id}/file`,{cookie:sa.cookie}),200);assert.equal(sha(file.raw),sha(Buffer.from(text)));
  state.upload={...uploaded.body,sha256:sha(file.raw),text};
  status(await upload(a.cookie,a.user.csrf_token,'container-acceptance.md','# 容器验收\nMarkdown 教学材料。\n'),201);
  status(await call('uploaded-cross-class-file',`/api/materials/${uploaded.body.material_id}/file`,{cookie:sb.cookie}),404);
  status(await call('csrf-less-password','/api/password',{method:'POST',cookie:a.cookie,body:{current_password:state.passwords.teacher_a,new_password:'DifferentSafe987',confirm_password:'DifferentSafe987'}}),403);
  const options=status(await call('registration-options','/api/account-options'),200);assert.equal(options.body.demo_registration,true);
  const registered='container_student_'+crypto.randomBytes(4).toString('hex');
  status(await call('register-student','/api/register',{method:'POST',body:{username:registered,class_code:'B'}}),201);
  status(await call('register-duplicate','/api/register',{method:'POST',body:{username:registered,class_code:'B'}}),409);
  status(await call('register-teacher-denied','/api/register',{method:'POST',body:{username:registered+'_teacher',class_code:'B',role:'teacher'}}),400);
  const initial=status(await login(registered,'123456'),200);assert.equal(initial.body.role,'student');assert.equal(initial.body.class_id,b.user.class_id);const ru=await identity(initial.cookie);
  status(await call('registered-first-block','/api/materials',{cookie:initial.cookie}),403);
  const pw=freshPassword();status(await change(initial.cookie,ru.csrf_token,'','123456'),400);status(await change(initial.cookie,ru.csrf_token,'',pw,'mismatch'),400);
  status(await change(initial.cookie,ru.csrf_token,'',pw,pw,{role:'teacher'}),400);status(await change(initial.cookie,ru.csrf_token,'',pw,pw,{class_id:1}),400);
  status(await change(initial.cookie,ru.csrf_token,'',pw),200);status(await login(registered,'123456'),401);
  const first=status(await login(registered,pw),200);const second=status(await login(registered,pw),200);const me=await identity(first.cookie);const next=freshPassword();
  status(await change(first.cookie,me.csrf_token,'',next),400);status(await change(first.cookie,me.csrf_token,'wrong',next),400);status(await change(first.cookie,me.csrf_token,pw,next),200);
  status(await call('password-session-one-revoked','/api/me',{cookie:first.cookie}),401);status(await call('password-session-two-revoked','/api/me',{cookie:second.cookie}),401);status(await login(registered,pw),401);status(await login(registered,next),200);
  state.passwords[registered]=next;state.registered=registered;
  status(await call('logout','/api/logout',{method:'POST',cookie:a.cookie,csrf:a.user.csrf_token}),200);status(await call('logout-replay','/api/me',{cookie:a.cookie}),401);
  fs.writeFileSync(stateFile,JSON.stringify(state));
 }else{
  const detail=status(await call('persisted-material',`/api/materials/${state.upload.material_id}`,{cookie:a.cookie}),200);assert.equal(detail.body.body_text,state.upload.text);
  const knowledge=status(await call('persisted-knowledge',`/api/knowledge-entries/${state.upload.knowledge_entry_id}`,{cookie:a.cookie}),200);assert.equal(knowledge.body.body_text,state.upload.text);
  const file=status(await call('persisted-file',`/api/materials/${state.upload.material_id}/file`,{cookie:sa.cookie}),200);assert.equal(sha(file.raw),state.upload.sha256);
  status(await login(state.registered,state.passwords[state.registered]),200);
 }
 fs.writeFileSync(output,JSON.stringify({date:'2026-10-13',transport:'Actual Docker Compose Nginx → Go → MySQL',entry:base,phase,passed:true,count:checks.length,checks},null,2));console.log(phase+': '+checks.length+' real container HTTP checks passed; evidence sanitized.');
})().catch(error=>{console.error('Container acceptance failed: '+error.message);process.exit(1);});
