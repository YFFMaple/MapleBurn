import json, os, glob, hashlib
from datetime import datetime, timezone, timedelta

local = timezone(timedelta(hours=8))
today = '2026-10-09'
seen = set()
tot = {'in': 0, 'out': 0, 'cc': 0, 'cr': 0, 'req': 0}
base = os.path.expanduser(r'~/.claude/projects')
for path in glob.iglob(os.path.join(base, '**', '*.jsonl'), recursive=True):
    try:
        f = open(path, 'r', encoding='utf-8', errors='replace')
    except OSError:
        continue
    with f:
        for line in f:
            if '"usage"' not in line or '"assistant"' not in line:
                continue
            try:
                l = json.loads(line)
            except Exception:
                continue
            if l.get('type') != 'assistant':
                continue
            msg = l.get('message') or {}
            u = msg.get('usage')
            if not u or not msg.get('model'):
                continue
            ts = l.get('timestamp', '')
            try:
                d = datetime.fromisoformat(ts.replace('Z', '+00:00')).astimezone(local)
            except Exception:
                continue
            if d.strftime('%Y-%m-%d') != today:
                continue
            i = u.get('input_tokens', 0)
            o = u.get('output_tokens', 0)
            cc = u.get('cache_creation_input_tokens', 0)
            cr = u.get('cache_read_input_tokens', 0)
            if i + o + cc + cr == 0:
                continue
            h = hashlib.sha1(f"{msg.get('id','')}|{i}|{o}|{cc}|{cr}".encode()).hexdigest()
            if h in seen:
                continue
            seen.add(h)
            tot['in'] += i
            tot['out'] += o
            tot['cc'] += cc
            tot['cr'] += cr
            tot['req'] += 1

total = tot['in'] + tot['out'] + tot['cc'] + tot['cr']
print('requests:', tot['req'], 'input:', tot['in'], 'output:', tot['out'],
      'cache_create:', tot['cc'], 'cache_read:', tot['cr'], 'TOTAL:', total)
