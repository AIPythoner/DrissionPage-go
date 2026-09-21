"""Record original Python behavior for the shared static compatibility corpus.

Usage: python -B scripts/python-reference.py E:/code/open-source/DrissionPage
Only writes the Go repository's testdata/reference.json.
"""
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
sys.path.insert(0, sys.argv[1])
from DrissionPage._elements.session_element import make_session_ele

root = Path(__file__).resolve().parent.parent
markup = '''<!doctype html><html><head><title>Reference</title></head><body>
<main id="root"><p id="a" class="item first" data-kind="alpha">hello <b>world</b></p>
<p id="b" class="item" data-kind="alphabet">hello again</p><p id="c" data-kind="beta">尾部结尾</p>
<div id="blocks">before<div>inside</div>after<br>last</div>
<pre id="pre">  line 1\n    line 2\tend</pre>
<table id="table"><tr><td>A</td><td>B</td></tr><tr><th>C</th><td>D</td></tr></table>
<div id="space"> a   b &amp; c&nbsp;d <span> e </span> f </div>
<div id="hidden">visible<script>ignored()</script><style>.a{}</style><span>tail</span></div>
<a id="link" href="/next">go</a><input id="disabled" disabled><input id="enabled">
<div id="quote" title="a'&quot;b">quoted</div><div id="empty"></div>
</main></body></html>'''
doc = make_session_ele(markup)
locators = ['#a', '.item', 'tag:p', 't:p', '@id=a', '@data-kind', '@data-kind=alpha', '@data-kind:alpha', '@data-kind^alpha', '@data-kind$bet', 'tag:p@@class:item@!id=b', '@|id=a@|id=c', 'text=hello again', 'text:hello', 'text^hello', 'text$结尾', 'tag:input@!disabled', 'xpath://p[2]', 'css:main > p.item', '@title=a\'"b', 'xpath://div[@id="empty"]', '#missing']
cases = []
for locator in locators:
    elements = doc.eles(locator)
    cases.append({'locator': locator, 'ids': [element.attr('id') or '' for element in elements]})
texts = [{'id': element_id, 'text': doc.ele('#' + element_id).text} for element_id in ['a', 'b', 'c', 'blocks', 'pre', 'table', 'space', 'hidden', 'empty']]
scalars = []
for expression in ["string-length('中文🙂')", "substring('甲乙丙丁', 2, 2)", "substring('12345', 0, 3)", "substring('12345', -2, 4)", "substring('12345', 1.5, 2.6)", "substring('12345', 0 div 0, 3)", "translate('甲乙甲丙', '甲乙', 'AB')", "translate('aba', 'aa', 'XY')", "count(//p)"]:
    value = doc.inner_ele.xpath(expression)
    scalars.append({'expression': expression, 'value': value})
output = {'source_version': '5.0.0b1', 'source_commit': 'b46345b', 'html': markup, 'locators': cases, 'texts': texts, 'scalars': scalars}
(root / 'testdata' / 'reference.json').write_text(json.dumps(output, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
print(f'Captured {len(cases)} locator and {len(texts)} text cases from Python.')
