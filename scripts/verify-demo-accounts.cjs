// Explicit local verification. Does not reset credentials or edit preset users.
'use strict';
const fs=require('node:fs');
const crypto=require('node:crypto');
const assert=require('node:assert/strict');
const base='http://127.0.0.1:8080';
const env=Object.fromEntries(fs.readFileSync('.env','utf8').split(/\r?\n/).filter(x=>/^[A-Z0-9_]+=/.test(x)).map(x=>{const i=x.indexOf('=');return [x.slice(0,i),x.slice(i+1)];}));
const names=['teacher_a','teacher_b','student_a1','student_b1'];
const privateFile='.local/pre-reset-sessions.json';
let count=0;
async function call(path,method='GET',body,cookie,csrf){
 const headers={};if(body)headers['Content-Type']='application/json';if(cookie)headers.Cookie=cookie;if(csrf)headers['X-CSRF-Token']=csrf;
 const response=await fetch(base+path,{method,headers,body:body?JSON.stringify(body):undefined});
 const data=await response.json();return {status:response.status,body:data,cookie:response.headers.get('set-cookie')?.split(';')[0]};
}
function check(actual,expected){assert.equal(actual,expected);count++;}
async function login(name,password){return call('/api/login','POST',{username:name,password});}
(async()=>{
 if(process.argv[2]==='before'){
  const sessions={};for(const name of names){const r=await login(name,env['SEED_'+name.toUpperCase()+'_PASSWORD']);check(r.status,200);sessions[name]=r.cookie;}
  fs.writeFileSync(privateFile,JSON.stringify(sessions));console.log('4 pre-reset sessions captured privately; no credentials printed.');return;
 }
 assert.equal(process.argv[2],'after');
 const old=JSON.parse(fs.readFileSync(privateFile,'utf8'));
 for(const name of names){
  check((await call('/api/me','GET',undefined,old[name])).status,401);
  check((await login(name,env['SEED_'+name.toUpperCase()+'_PASSWORD'])).status,401);
  const r=await login(name,'123456');check(r.status,200);check(r.body.must_change_password,true);
  const me=await call('/api/me','GET',undefined,r.cookie);check(me.status,200);
  check((await call('/api/materials','GET',undefined,r.cookie)).status,403);
  check((await call('/api/logout','POST',undefined,r.cookie,me.body.csrf_token)).status,200);
 }
 const name='demo_verify_'+crypto.randomBytes(5).toString('hex');
 const created=await call('/api/register','POST',{username:name,class_code:'B'});check(created.status,201);assert.equal(JSON.stringify(created.body).includes('123456'),false);count++;
 check((await call('/api/register','POST',{username:name,class_code:'B'})).status,409);
 check((await call('/api/register','POST',{username:name+'_x',class_code:'B',role:'teacher'})).status,400);
 const first=await login(name,'123456');check(first.status,200);check(first.body.role,'student');check(first.body.must_change_password,true);
 const identity=await call('/api/me','GET',undefined,first.cookie);const csrf=identity.body.csrf_token;
 const password='Personal9'+crypto.randomBytes(8).toString('hex');const next='Second8'+crypto.randomBytes(8).toString('hex');
 check((await call('/api/password','POST',{new_password:password,confirm_password:password},first.cookie)).status,403);
 check((await call('/api/password','POST',{new_password:password,confirm_password:password,class_id:1},first.cookie,csrf)).status,400);
 check((await call('/api/password','POST',{new_password:password,confirm_password:password},first.cookie,csrf)).status,200);
 check((await call('/api/me','GET',undefined,first.cookie)).status,401);
 check((await login(name,'123456')).status,401);
 const normal=await login(name,password);check(normal.status,200);check(normal.body.must_change_password,false);
 const me=await call('/api/me','GET',undefined,normal.cookie);
 check((await call('/api/materials','GET',undefined,normal.cookie)).status,200);
 check((await call('/api/password','POST',{new_password:next,confirm_password:next},normal.cookie,me.body.csrf_token)).status,400);
 check((await call('/api/password','POST',{current_password:password,new_password:next,confirm_password:next},normal.cookie,me.body.csrf_token)).status,200);
 check((await login(name,password)).status,401);check((await login(name,next)).status,200);
 fs.mkdirSync('deliverables/account-management',{recursive:true});
 fs.writeFileSync('deliverables/account-management/http-results.json',JSON.stringify({entry:base,checks:count,passed:true,notes:'Real local HTTP. Passwords, session cookies and CSRF omitted. Preset accounts remain in first-change state.',temporary_account:name},null,2));
 fs.writeFileSync('.local/demo-verify-account.txt',name);fs.unlinkSync(privateFile);
 console.log(count+' local account HTTP checks passed; sanitized report written.');
})().catch(()=>{console.error('Local account verification failed; secrets omitted.');process.exit(1);});
