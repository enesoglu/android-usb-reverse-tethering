"""Render the public engineering guide using only the Python standard library."""
from pathlib import Path
from html import escape, unescape
from urllib.parse import urljoin, urlsplit
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / "docs/reverse-tethering-engineering.md"
OUTPUT = ROOT / "site/engineering.html"
REPOSITORY = "https://github.com/enesoglu/android-usb-reverse-tethering/blob/main/"
CANONICAL = "https://projects.enesoglu.com/android-usb-reverse-tethering/engineering.html"

def inline(text):
    text = escape(text)
    text = re.sub(r"`([^`]+)`", r"<code>\1</code>", text)
    text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
    def link(match):
        target = unescape(match.group(2))
        if not urlsplit(target).scheme and not target.startswith("#"):
            target = urljoin(REPOSITORY + "docs/", target)
        if urlsplit(target).scheme not in {"", "https", "http"}:
            raise ValueError("Unsupported article link scheme")
        return '<a href="' + escape(target, quote=True) + '">' + match.group(1) + '</a>'
    return re.sub(r"\[([^\]]+)\]\(([^)]+)\)", link, text)

parts, toc, paragraph = [], [], []
listing = False
code = None

def flush():
    if paragraph:
        parts.append("<p>" + inline(" ".join(paragraph)) + "</p>")
        paragraph.clear()

def close_list():
    global listing
    if listing:
        parts.append("</ul>")
        listing = False

for line in SOURCE.read_text(encoding="utf-8").splitlines():
    if line.startswith("```"):
        flush(); close_list()
        if code is None:
            code = []
        else:
            parts.append("<pre><code>" + escape("\n".join(code)) + "</code></pre>")
            code = None
        continue
    if code is not None:
        code.append(line)
        continue
    heading = re.match(r"^(#{1,3}) (.+)$", line)
    if heading:
        flush(); close_list()
        level, title = len(heading[1]), heading[2]
        slug = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
        parts.append(f'<h{level} id="{slug}">{inline(title)}</h{level}>')
        if level == 2:
            toc.append('<li><a href="#' + slug + '">' + escape(title) + '</a></li>')
    elif line.startswith("- "):
        flush()
        if not listing:
            parts.append("<ul>"); listing = True
        parts.append("<li>" + inline(line[2:]) + "</li>")
    elif not line.strip():
        flush(); close_list()
    else:
        close_list(); paragraph.append(line.strip())
flush(); close_list()
if code is not None:
    raise ValueError("Unclosed code block in engineering guide")
style = re.search(r"<style>(.*?)</style>", (ROOT/"site/index.html").read_text(encoding="utf-8"), re.S)[1]
style += " article{max-width:860px;margin:auto}article h1{font-size:clamp(2.2rem,5vw,3.8rem)}article h2{margin-top:48px;padding-top:24px;border-top:1px solid var(--line)}pre{overflow:auto;padding:20px;background:var(--panel);border-radius:10px}code{font:0.87em/1.5 ui-monospace,monospace;overflow-wrap:anywhere}pre code{overflow-wrap:normal}aside{background:var(--panel);padding:20px 24px;border-radius:10px}aside ul{margin:0;padding-left:20px}aside li{margin:5px 0}"
output = '<!doctype html>\n<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Engineering USB reverse tethering | USB Relay</title><meta name="description" content="Understand USB negotiation, Android VPN and TUN, ADB, accessory transport, packet sizing, TLS and flow control in a rootless reverse-tethering app."><link rel="canonical" href="' + CANONICAL + '"><meta property="og:type" content="article"><meta property="og:title" content="Engineering USB reverse tethering"><meta property="og:url" content="' + CANONICAL + '"><meta name="theme-color" content="#101b24"><link rel="icon" href="usb-relay.ico"><style>' + style + '</style></head><body><a class="skip" href="#main">Skip to content</a><nav aria-label="Main"><a class="brand" href="./">USB RELAY</a><div><a href="./#setup">Setup</a> · <a href="https://github.com/enesoglu/android-usb-reverse-tethering/releases">Downloads</a> · <a href="' + REPOSITORY + 'docs/reverse-tethering-engineering.md">Article source</a></div></nav><main id="main"><article><div class="eyebrow">Architecture &amp; design</div>' + parts[0] + '<aside aria-label="On this page"><strong>On this page</strong><ul>' + ''.join(toc) + '</ul></aside>' + ''.join(parts[1:]) + '</article></main><footer>USB Relay contributors · <a href="https://github.com/enesoglu/android-usb-reverse-tethering/blob/main/LICENSE">GPL-3.0-only</a> · <a href="./">Project home</a></footer></body></html>\n'
if "--check" in sys.argv:
    if not OUTPUT.exists() or OUTPUT.read_text(encoding="utf-8") != output:
        raise SystemExit("Engineering HTML is stale; run python scripts/render-engineering.py")
    print("PASS: engineering article matches its Markdown source")
else:
    OUTPUT.write_text(output, encoding="utf-8")
    print("Rendered public engineering article")
