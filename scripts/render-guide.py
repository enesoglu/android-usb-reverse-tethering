"""Render the trusted English installation guide as an offline, escaped HTML page."""
from pathlib import Path
import html
import re

root=Path(__file__).resolve().parent.parent
source=(root/'docs/INSTALLATION.md').read_text(encoding='utf-8')
def inline(text):
    text=html.escape(text)
    text=re.sub(r'`([^`]+)`',r'<code>\1</code>',text)
    text=re.sub(r'\*\*([^*]+)\*\*',r'<strong>\1</strong>',text)
    return re.sub(r'(https://[^\s<>]+)',lambda m:'<a href="'+m.group(1)+'">'+m.group(1)+'</a>',text)
parts=[];listing=None
for line in source.splitlines():
    if line.startswith('#'):
        if listing:parts.append('</'+listing+'>');listing=None
        mark,text=line.split(' ',1);level=min(3,len(mark));slug=re.sub('[^a-z0-9 -]','',text.lower()).replace(' ','-')
        parts.append(f'<h{level} id="{slug}">'+inline(text)+f'</h{level}>')
    elif re.match(r'^(- |\d+\. )',line):
        ordered=not line.startswith('- ');kind='ol' if ordered else 'ul'
        if listing!=kind:
            if listing:parts.append('</'+listing+'>')
            parts.append('<'+kind+'>');listing=kind
        parts.append('<li>'+inline(re.sub(r'^(- |\d+\. )','',line))+'</li>')
    else:
        if listing:parts.append('</'+listing+'>');listing=None
        if line.strip():parts.append('<p>'+inline(line)+'</p>')
if listing:parts.append('</'+listing+'>')
style='body{font:16px/1.7 system-ui,sans-serif;background:#0d1623;color:#e4edf8;max-width:900px;margin:40px auto;padding:0 24px}h1,h2,h3{line-height:1.25;color:#fff}h2{margin-top:46px;border-top:1px solid #304056;padding-top:24px}a{color:#66e6c7}code{background:#1c2b3e;padding:2px 6px;border-radius:4px}li{margin:8px 0}nav{padding:14px;background:#172638;border-radius:12px}@media print{body{background:white;color:black}h1,h2,h3{color:black}a{color:#164f46}}'
output='<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>USB Relay — Installation guide</title><style>'+style+'</style><body><nav>USB Relay · <a href="#1-download-the-right-windows-package">Windows setup</a> · <a href="#4-enable-developer-options-on-the-phone">Phone setup</a> · <a href="#troubleshooting">Troubleshooting</a></nav>'+''.join(parts)+'</body></html>\n'
(root/'docs/START-HERE.html').write_text(output,encoding='utf-8')
