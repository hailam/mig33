#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root = Path('data/applets/integration/friend-moderation-out')
for folder in [root] + [root / user for user in ['hailam', 'alice', 'bob']]:
    folder.mkdir(parents=True, exist_ok=True)
    for path in folder.iterdir():
        if path.is_file(): path.unlink()
PY
if ! INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4FriendModerationIntegration \
 sh tools/applets/v4/integration/run.sh hailam tcp >data/applets/integration/friend-moderation-out/hailam/runtime.log 2>&1; then
 cat data/applets/integration/friend-moderation-out/hailam/runtime.log
 exit 1
fi
cat data/applets/integration/friend-moderation-out/hailam/result.txt
friend_pids=
friend_cleanup() {
 for pid in $friend_pids; do kill "$pid" 2>/dev/null || true; done
 for pid in $friend_pids; do wait "$pid" 2>/dev/null || true; done
}
trap friend_cleanup EXIT
trap 'exit 130' INT TERM
for friend_user in alice bob; do
 friend_transport=tcp
 if [ "$friend_user" = bob ]; then friend_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4FriendModerationIntegration \
  sh tools/applets/v4/integration/run.sh "$friend_user" "$friend_transport" >"data/applets/integration/friend-moderation-out/$friend_user/runtime.log" 2>&1 &
 friend_pids="$friend_pids $!"
done
friend_status=0
for pid in $friend_pids; do if ! wait "$pid"; then friend_status=1; fi; done
friend_pids=
if [ "$friend_status" != 0 ]; then
 for friend_user in alice bob; do cat "data/applets/integration/friend-moderation-out/$friend_user/runtime.log"; done
 exit 1
fi
for friend_user in alice bob; do cat "data/applets/integration/friend-moderation-out/$friend_user/result.txt"; done
python3 - <<'PY'
from pathlib import Path
from datetime import datetime, timezone
import hashlib, json
root = Path('data/applets/integration/friend-moderation-out')
summary = {'date': datetime.now(timezone.utc).date().isoformat(),
    'jar_sha256': hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),
    'clients': {}, 'total_screenshots': 0, 'total_binary_frames': 0}
for user, transport in [('hailam', 'tcp'), ('alice', 'tcp'), ('bob', 'http')]:
    folder = root / user
    events = [line.split('\t') for line in (folder / 'events.tsv').read_text().splitlines()]
    wire = [event for event in events if event[0] in ('TX', 'RX')]
    logout = [event for event in wire if event[:2] == ['TX', '300']][-1]
    # The receiver can record an ACK before the sending thread records its TX.
    assert any(event[:3] == ['RX', '1', logout[2]] for event in wire)
    assert not any(event[:2] == ['TX', '417'] for event in events)
    result = (folder / 'result.txt').read_text().strip()
    assert result.startswith('PASS ')
    screenshots, frames = len(list(folder.glob('*.png'))), len(list(folder.glob('*.bin')))
    summary['clients'][user] = {'transport': transport, 'screenshots': screenshots, 'binary_frames': frames,
        'logout_transaction': int(logout[2]), 'correlated_logout_ack': True, 'personal_block_packets': 0, 'result': result}
    summary['total_screenshots'] += screenshots
    summary['total_binary_frames'] += frames
(root / 'evidence-summary.json').write_text(json.dumps(summary, indent=2) + '\n')
print('Saved evidence summary:', summary['total_screenshots'], 'screenshots,', summary['total_binary_frames'], 'binary frames')
PY
