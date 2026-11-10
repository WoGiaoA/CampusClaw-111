"""Headless Chrome UI checks with local API fixtures; no real model or account access."""
from pathlib import Path
from playwright.sync_api import sync_playwright
from urllib.parse import urlparse
import json
import os
os.chdir(Path(__file__).resolve().parent.parent)
web=Path('web');out=Path('.local/ui-interaction');out.mkdir(exist_ok=True)
apps=[dict(id=4,name='课程资料问答',enabled=True,role_prompt='你是课程资料助教，优先依据相关资料回答。',template_id=3,skill_ids=[9],material_ids=list(range(1,9))),dict(id=5,name='解题引导',enabled=True,role_prompt='帮助学生自行推导，循序给出提示。',template_id=None,skill_ids=[9],material_ids=[])]
templates=[dict(id=3,name='课程资料问答',body='概括要点，说明依据和适用条件，使用清晰简洁的语言。')]
skills=[dict(id=9,name='解题引导',enabled=True,conditions='解题、怎么做、判别式',procedure='先询问题意，再循序给提示，不直接给最终答案。',raw_text='名称：解题引导\n适用条件：解题\n启用：是\n\n循序提示。',tool_refs=[]),dict(id=10,name='人工测试解题引导',enabled=True,conditions='青竹验收',procedure='只给下一步提示。',raw_text='测试原文',tool_refs=[])]
materials=[dict(id=i,title=f'{i:02d}_'+['一元一次方程','二元一次方程组','一元二次方程与判别式','一元二次方程求解方法'][i%4]+'.md',original_filename=f'lecture-{i}.md') for i in range(1,25)]
conversations=[dict(id=8,app_id=4,app_name='课程资料问答',title='期末项目如何提交？'),dict(id=9,app_id=4,app_name='课程资料问答',title='用一句话介绍什么是一元二次方程，并解释判别式的意义和使用条件')]
hit=dict(number=1,material_id=1,chunk_id=7,material_title='星桥班课程考核与提交说明',chunk_index=1,char_start=0,char_end=60,excerpt='摘要',chunk_text='项目需要提交源代码仓库地址、README 运行说明及演示视频，并遵守规定的提交时间。')
errors=[];checks=[]
with sync_playwright() as pw:
 browser=pw.chromium.launch(executable_path=r'C:\Program Files\Google\Chrome\Application\chrome.exe',headless=True)
 for width,height in [(1463,775),(1280,720),(390,780)]:
  for role in ['teacher','student']:
   apps[0]['material_ids']=list(range(1,9)); apps[0]['enabled']=True; conversations[0]['app_enabled']=True
   page=browser.new_page(viewport=dict(width=width,height=height),is_mobile=width==390,has_touch=width==390)
   page.on('pageerror',lambda e:errors.append(str(e)))
   def reply(route):
    path=urlparse(route.request.url).path;method=route.request.method
    if path in ['/app.js','/style.css']:return route.fulfill(path=str(web/path[1:]))
    if not path.startswith('/api/'):return route.fulfill(path=str(web/'index.html'))
    values={'/api/me':dict(username='fixture_'+role,role=role,class_name='隔离检查 A 班',csrf_token='fixture'),'/api/ai/apps':dict(items=apps if role=='teacher' else [dict(id=a['id'],name=a['name']) for a in apps if a['enabled']]),'/api/ai/prompt-templates':dict(items=templates),'/api/ai/skills':dict(items=skills),'/api/materials':materials,'/api/mcp/servers':dict(items=[],configured=True,allowed_base_urls=['http://mcp-server:8080']),'/api/ai/conversations':dict(items=conversations)}
    if path=='/api/ai/apps/4' and method=='PATCH':apps[0].update(route.request.post_data_json);return route.fulfill(json=apps[0])
    if path=='/api/ai/conversations/8':
     if method=='DELETE':return route.fulfill(json=dict(status='deleted'))
     return route.fulfill(json=dict(conversation=conversations[0],items=[dict(status='completed',messages=[dict(role='user',body='期末项目如何提交？'),dict(role='assistant',body='根据参考资料，项目需要提交源代码仓库地址、README 运行说明及演示视频，并遵守规定的提交时间。[1]',metadata=dict(citations=[hit]))])]))
    if path.endswith('/tool-grants'):return route.fulfill(json=dict(items=[]))
    if path not in values:raise AssertionError(path)
    return route.fulfill(json=values[path])
   page.route('http://campusclaw.test/**',reply)
   def overflow(label):
    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'),label+' horizontal overflow'
    checks.append(f'{width}x{height} {role} {label}')
   if role=='teacher':
    page.goto('http://campusclaw.test/workspace');page.get_by_text('2 个已启用',exact=True).wait_for();overflow('home')
    page.goto('http://campusclaw.test/applications');page.get_by_role('button',name='保存配置',exact=True).wait_for();overflow('basic')
    assert page.get_by_role('button',name='保存配置',exact=True).is_disabled()
    # Show the requested left-aligned switch and save feedback in actual Chrome.
    page.get_by_role('textbox',name='应用名称',exact=True).fill(f'课程资料问答 · {width}复查')
    page.get_by_role('button',name='保存配置',exact=True).click()
    page.locator('#global-toast').wait_for();assert page.get_by_role('button',name='保存配置',exact=True).is_disabled()
    page.get_by_role('button',name='提示词模板',exact=True).click();overflow('prompts');page.screenshot(path=str(out/f'teacher-prompts-{width}.png'),full_page=True)
    page.get_by_role('button',name='Skill 技能',exact=True).click();overflow('skills')
    card=page.locator('.skill-card').first
    card.locator('summary',has_text='编辑配置').click()
    card.locator('summary',has_text='查看上传原文').click()
    assert card.locator(':scope > details[open]').count()==2
    panel=card.locator('.skill-detail-panel').first
    assert panel.evaluate('(n)=>getComputedStyle(n).backgroundColor')=='rgb(252, 252, 253)'
    assert panel.evaluate('(n)=>getComputedStyle(n).borderTopWidth')=='1px'
    overflow('skill-panels-expanded')
    panel.scroll_into_view_if_needed();page.screenshot(path=str(out/f'teacher-skill-panels-{width}.png'),full_page=True)
    page.get_by_role('button',name='知识库绑定',exact=True).click();overflow('materials');assert page.locator('.binding-list').evaluate('(n)=>n.scrollHeight>n.clientHeight')
    page.screenshot(path=str(out/f'teacher-materials-{width}.png'),full_page=True)
    page.get_by_role('searchbox',name='搜索资料名称').fill('判别式');page.get_by_role('button',name='全选筛选结果',exact=True).click();assert page.get_by_role('button',name='保存资料绑定').is_enabled()
    page.get_by_role('button',name='基本配置',exact=True).click();page.get_by_role('dialog').wait_for();page.get_by_role('button',name='取消',exact=True).click();assert page.get_by_role('searchbox').input_value()=='判别式'
    page.get_by_role('button',name='保存资料绑定',exact=True).click();page.get_by_role('button',name='基本配置',exact=True).click()
    page.get_by_role('button',name='+ 新建',exact=True).click();page.get_by_role('dialog').wait_for();overflow('new-app-dialog');page.get_by_role('button',name='取消',exact=True).click()
    assert page.get_by_role('switch',name='启用应用',exact=True).is_checked()
    page.get_by_role('button',name='删除应用',exact=True).click();page.get_by_role('dialog').wait_for();overflow('delete-app-dialog')
    page.screenshot(path=str(out/f'teacher-delete-app-{width}.png'),full_page=True)
    page.get_by_role('button',name='取消',exact=True).click()
   else:
    page.goto('http://campusclaw.test/assistant');page.locator('.session-open').first.click();page.locator('.compact-source summary').wait_for();overflow('conversation')
    source=page.locator('.citation-details');assert not source.evaluate('(n)=>n.open');assert source.locator('.citation-full').is_hidden()
    page.screenshot(path=str(out/f'student-sources-closed-{width}.png'),full_page=True)
    source.locator('summary').click();assert source.locator('.citation-full').is_visible();assert source.locator('a').get_attribute('href')=='/materials?material_id=1&chunk_id=7';overflow('source-expanded')
    page.locator('.session-row').first.hover();page.locator('.session-menu').first.click();page.get_by_role('dialog').wait_for();assert page.get_by_role('button',name='取消',exact=True).evaluate('(n)=>n===document.activeElement');overflow('delete-dialog')
    page.screenshot(path=str(out/f'student-delete-{width}.png'),full_page=True);page.get_by_role('button',name='取消',exact=True).click()
    apps[0]['enabled']=False;conversations[0]['app_enabled']=False
    page.reload();page.locator('.session-open').first.click()
    page.locator('.application-readonly-notice').wait_for()
    assert page.get_by_role('textbox',name='你的问题').is_disabled()
    assert page.get_by_role('button',name='发送',exact=True).is_disabled()
    overflow('disabled-app-history')
    page.screenshot(path=str(out/f'student-disabled-app-{width}.png'),full_page=True)
   page.close()
 browser.close()
assert not errors,errors
(out/'checks.json').write_text(json.dumps(dict(checks=checks,errors=errors,source='isolated browser API fixtures; no course model calls'),ensure_ascii=False,indent=2),encoding='utf-8')
print(f'{len(checks)} Chrome layout/interaction checks passed; no page errors; screenshots: {out}')
