"""Fail closed on accidental private files, private text, or missing public docs."""
from pathlib import Path
import json
import re
import subprocess
import sys
from html.parser import HTMLParser

root=Path(__file__).resolve().parent.parent
command=['git','ls-files','--cached','--others','--exclude-standard']
files=subprocess.run(command,cwd=root,check=True,capture_output=True,text=True).stdout.splitlines()
forbidden_ext={'.keystore','.jks','.dpapi','.pfx','.p12'}
text_ext={'.md','.go','.java','.cs','.xaml','.xml','.yml','.yaml','.ps1','.py','.json','.toml','.txt','.mod','.sum','.html','.iss','.pem'}
failures=[]
for name in files:
    path=root/name
    if any(part in {'.local','build','results','bin','obj','.git'} for part in Path(name).parts) or path.suffix.lower() in forbidden_ext:
        failures.append('Private/generated file selected: '+name);continue
    if path.name in {'progress.md','desktop-plan.md'}:failures.append('Private work notes selected: '+name)
    if path.suffix.lower() not in text_ext or not path.is_file():continue
    text=path.read_text(encoding='utf-8-sig')
    if re.search(r'-----BEGIN (?:RSA |EC |OPENSSH |ENCRYPTED )?PRIVATE KEY-----',text):failures.append('Private-key block: '+name)
    if not name.startswith('third_party/'):
        if re.search(r'\b[A-Z]:[/\\]Users[/\\](?!Public\b|Default\b)[^/\\\s]+',text,re.I):failures.append('Personal Windows home path: '+name)
        if re.search(r'[A-Z0-9._%+-]+@(?:gmail|hotmail|outlook|icloud)\.com',text,re.I):failures.append('Personal email: '+name)
        if any(character in text for character in '\u011f\u011e\u0131\u0130\u015f\u015e'):failures.append('Non-English Turkish text: '+name)
    if name.startswith(('docs/','site/')) or name=='README.md':
        if re.search(r'(?i)\b(?:test[ _-]?id|device[ _-]?serial|phone[ _-]?serial)\s*[:=]?\s*[A-Za-z0-9]{8,}',text):failures.append('Session/device identifier in public documentation: '+name)
        if re.search(r'\bSM-S[0-9]{3}[A-Z]?\b',text):failures.append('Device-specific model in public documentation: '+name)
        if re.search(r'(?:\.\./)?(?:results/|\.local/)',text):failures.append('Private evidence link in public documentation: '+name)
    if path.suffix=='.md':
        for link in re.findall(r'!?\[[^\]]*\]\(([^)]+)\)',text):
            if re.match(r'^[a-z]+:',link) or link.startswith('#'):continue
            target=(path.parent/link.split('#',1)[0]).resolve()
            if not target.is_relative_to(root) or not target.exists():failures.append('Broken local link in '+name+': '+link)
required=['docs/reverse-tethering-engineering.md','site/engineering.html','LICENSE','README.md','CONTRIBUTING.md','SECURITY.md','PRIVACY.md','docs/INSTALLATION.md','docs/START-HERE.html','packaging/platform-tools.json','packaging/android-signing-certificate.pem']
for name in required:
    if not (root/name).is_file():failures.append('Missing public file: '+name)
if 'GNU GENERAL PUBLIC LICENSE' not in (root/'LICENSE').read_text(encoding='utf-8'):failures.append('Missing GPL license text')
metadata=json.loads((root/'packaging/platform-tools.json').read_text(encoding='utf-8'))
if not re.fullmatch('[a-f0-9]{64}',metadata['sha256']) or not metadata['url'].startswith('https://dl.google.com/android/repository/'):failures.append('Unpinned/unexpected SDK download')
managed=(root/'internal/adb/managed_tools.go').read_text(encoding='utf-8')
if '"'+metadata['version']+'"' not in managed:failures.append('Managed SDK version differs between UI and daemon')
for workflow in (root/'.github/workflows').glob('*.yml'):
    for action in re.findall(r'uses:\s*([^\s#]+)',workflow.read_text(encoding='utf-8')):
        if not re.fullmatch(r'[^@]+@[a-f0-9]{40}',action):failures.append('Unpinned action in '+workflow.name+': '+action)
class GuideParser(HTMLParser):
    def __init__(self):super().__init__();self.ids=set();self.links=[]
    def handle_starttag(self,tag,attrs):
        attrs=dict(attrs)
        if 'id' in attrs:self.ids.add(attrs['id'])
        if tag=='a':self.links.append(attrs.get('href',''))
guide=GuideParser();guide.feed((root/'docs/START-HERE.html').read_text(encoding='utf-8'))
for link in guide.links:
    if link.startswith('#') and link[1:] not in guide.ids:failures.append('Broken offline guide anchor: '+link)
if not any(link.startswith('https://github.com/enesoglu/android-usb-reverse-tethering/releases') for link in guide.links):failures.append('Offline download link was not rendered correctly')
article_check=subprocess.run([sys.executable,str(root/'scripts/render-engineering.py'),'--check'],cwd=root)
if article_check.returncode:failures.append('Engineering article render mismatch')
if failures:
    for failure in failures:print('FAIL: '+failure)
    raise SystemExit(1)
print('PASS: English publication files, local links, pinned dependencies, private-file exclusion and offline guide')
