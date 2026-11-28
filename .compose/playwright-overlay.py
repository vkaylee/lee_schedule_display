#!/usr/bin/env python3
"""Create an HTTP-only authentication overlay for the disposable UI stack."""
import json
from pathlib import Path

source = Path('/workspace')
output = Path('/tmp/ui-overlay')
output.mkdir()
replacements = {
    'main.go': [('if !validOrigin(origin, dev) {', 'if origin != "http://ui-audit:8080" && !validOrigin(origin, dev) {', 1)],
}
overlay = {}
for filename, changes in replacements.items():
    original = source / filename
    text = original.read_text()
    for old, new, count in changes:
        if text.count(old) != count:
            raise SystemExit(f'Unexpected {filename} structure: {old}')
        text = text.replace(old, new)
    target = output / filename
    target.write_text(text)
    overlay[str(original)] = str(target)
(output / 'overlay.json').write_text(json.dumps({'Replace': overlay}))
