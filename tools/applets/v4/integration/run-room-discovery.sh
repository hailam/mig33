#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
from datetime import datetime,timezone
root=Path('data/applets/integration/room-discovery-out')
for folder in [root]+[root/p for p in ['creator','observer','returning']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file(): p.unlink()
(root/'room-name.txt').write_text('Discovery '+datetime.now(timezone.utc).strftime('%Y%m%d%H%M%S')+'\n')
PY
for discovery_phase in creator observer returning; do
 discovery_user=hailam
 discovery_transport=http
 if [ "$discovery_phase" = creator ]; then discovery_transport=tcp; fi
 if [ "$discovery_phase" = observer ]; then discovery_user=bob; fi
 if ! ROOM_DISCOVERY_PHASE="$discovery_phase" INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4RoomDiscoveryIntegration \
  sh tools/applets/v4/integration/run.sh "$discovery_user" "$discovery_transport" >"data/applets/integration/room-discovery-out/$discovery_phase/runtime.log" 2>&1; then
  cat "data/applets/integration/room-discovery-out/$discovery_phase/runtime.log"
  exit 1
 fi
 cat "data/applets/integration/room-discovery-out/$discovery_phase/result.txt"
done
python3 - <<'PY'
from pathlib import Path
from datetime import datetime, timezone
import hashlib, json
root = Path('data/applets/integration/room-discovery-out')
summary = {
    'date': datetime.now(timezone.utc).date().isoformat(),
    'jar_sha256': hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),
    'room_name': (root / 'room-name.txt').read_text().strip(),
    'phases': {}, 'total_screenshots': 0, 'total_binary_frames': 0,
}
for phase, user, transport in [('creator', 'hailam', 'tcp'), ('observer', 'bob', 'http'), ('returning', 'hailam', 'http')]:
    folder = root / phase
    events = [line.split('\t') for line in (folder / 'events.tsv').read_text().splitlines()]
    wire = [event for event in events if event[0] in ('TX', 'RX')]
    assert wire[-2][:2] == ['TX', '300'] and wire[-1][:2] == ['RX', '1'] and wire[-2][2] == wire[-1][2]
    result = (folder / 'result.txt').read_text().strip()
    assert result.startswith('PASS ')
    screenshots, frames = len(list(folder.glob('*.png'))), len(list(folder.glob('*.bin')))
    summary['phases'][phase] = {
        'user': user, 'transport': transport, 'screenshots': screenshots, 'binary_frames': frames,
        'tx': sum(event[0] == 'TX' for event in events), 'rx': sum(event[0] == 'RX' for event in events),
        'logout_transaction': int(wire[-1][2]), 'correlated_logout_ack': True, 'result': result,
    }
    summary['total_screenshots'] += screenshots
    summary['total_binary_frames'] += frames
(root / 'evidence-summary.json').write_text(json.dumps(summary, indent=2) + '\n')
print('Saved evidence summary:', summary['total_screenshots'], 'screenshots,', summary['total_binary_frames'], 'binary frames')
PY
