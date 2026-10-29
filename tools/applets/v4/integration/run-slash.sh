#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root = Path('data/applets/integration/slash-out')
for folder in [root] + [root / name for name in ['alice', 'bob', 'carol', 'returning-bob']]:
    folder.mkdir(parents=True, exist_ok=True)
    for path in folder.iterdir():
        if path.is_file(): path.unlink()
PY
slash_pids=
slash_cleanup() {
 for pid in $slash_pids; do kill "$pid" 2>/dev/null || true; done
 for pid in $slash_pids; do wait "$pid" 2>/dev/null || true; done
}
trap slash_cleanup EXIT
trap 'exit 130' INT TERM
for slash_user in alice bob carol; do
 slash_transport=tcp
 if [ "$slash_user" = bob ]; then slash_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4SlashIntegration \
  sh tools/applets/v4/integration/run.sh "$slash_user" "$slash_transport" >"data/applets/integration/slash-out/$slash_user/runtime.log" 2>&1 &
 slash_pids="$slash_pids $!"
done
slash_status=0
for pid in $slash_pids; do if ! wait "$pid"; then slash_status=1; fi; done
slash_pids=
if [ "$slash_status" != 0 ]; then
 for slash_user in alice bob carol; do cat "data/applets/integration/slash-out/$slash_user/runtime.log"; done
 exit 1
fi
for slash_user in alice bob carol; do cat "data/applets/integration/slash-out/$slash_user/result.txt"; done
if ! SLASH_PHASE=returning INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4SlashIntegration \
 sh tools/applets/v4/integration/run.sh bob http >data/applets/integration/slash-out/returning-bob/runtime.log 2>&1; then
 cat data/applets/integration/slash-out/returning-bob/runtime.log
 exit 1
fi
cat data/applets/integration/slash-out/returning-bob/result.txt
python3 - <<'PY'
from pathlib import Path
from datetime import datetime, timezone
import hashlib, json
root = Path('data/applets/integration/slash-out')
summary = {'date': datetime.now(timezone.utc).date().isoformat(),
    'jar_sha256': hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),
    'clients': {}, 'total_screenshots': 0, 'total_binary_frames': 0}
for name, transport in [('alice', 'tcp'), ('bob', 'http'), ('carol', 'tcp'), ('returning-bob', 'http')]:
    folder = root / name
    events = [line.split('\t') for line in (folder / 'events.tsv').read_text().splitlines()]
    wire = [event for event in events if event[0] in ('TX', 'RX')]
    logout = [event for event in wire if event[:2] == ['TX', '300']][-1]
    assert any(event[:3] == ['RX', '1', logout[2]] for event in wire)
    result = (folder / 'result.txt').read_text().strip()
    assert result.startswith('PASS ')
    screenshots, frames = len(list(folder.glob('*.png'))), len(list(folder.glob('*.bin')))
    summary['clients'][name] = {'transport': transport, 'screenshots': screenshots, 'binary_frames': frames,
        'logout_transaction': int(logout[2]), 'correlated_logout_ack': True, 'result': result}
    summary['total_screenshots'] += screenshots
    summary['total_binary_frames'] += frames
(root / 'evidence-summary.json').write_text(json.dumps(summary, indent=2) + '\n')
print('Saved evidence summary:', summary['total_screenshots'], 'screenshots,', summary['total_binary_frames'], 'binary frames')
PY
