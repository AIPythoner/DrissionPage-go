"""Record source public members; write only the Go repository's docs directory."""
import ast
from pathlib import Path
import sys

source = Path(sys.argv[1]).resolve() / 'DrissionPage'
destination = Path(__file__).resolve().parent.parent / 'docs' / 'API_INVENTORY.md'
lines = ['# Python 公开成员目录', '', '基准：DrissionPage 5.0.0b1 / b46345b。由 AST 提取类中直接定义的公开方法/属性，不重复展开继承成员。此目录用于检查迁移范围，不是逐成员测试通过率。Go 对应见 [迁移清单](MIGRATION.md)。', '']
count = 0
for path in sorted(source.rglob('*.py')):
    classes = []
    for node in ast.parse(path.read_text(encoding='utf-8-sig')).body:
        if isinstance(node, ast.ClassDef):
            members = [item.name for item in node.body if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)) and not item.name.startswith('_')]
            if members:
                classes.append((node.name, members))
    if classes:
        lines.extend(['## ' + path.relative_to(source).as_posix(), ''])
    for name, members in classes:
        count += len(members)
        lines.extend(['### ' + name, '', ', '.join('`' + item + '`' for item in members), ''])
lines.extend([f'共 {count} 个直接定义的公开成员，包括内部模块中的公开命名类成员。此数不是 Go 实现数量或兼容性百分比。', ''])
destination.write_text('\n'.join(lines), encoding='utf-8')
print(f'Recorded {count} public members.')
