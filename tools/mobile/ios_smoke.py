#!/usr/bin/env -S uv run --no-project python
"""Run native iOS UI smoke tests against isolated original-protocol listeners.

Requires the app's generated iOS workspace, installed Ruby/CocoaPods profile,
a built Go server, and an existing Metro server. No packages are installed.
Only this runner's backend/peer processes are stopped. The ignored Xcode project
is restored afterward; test logs, screenshots and xcresult stay under data/.
"""
import argparse
import contextlib
import json
import os
from pathlib import Path
import signal
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
PROJECT = ROOT / 'apps/mobile/ios/mig33.xcodeproj'
PROJECT_FILE = PROJECT / 'project.pbxproj'
SCHEME = PROJECT / 'xcshareddata/xcschemes/Mig33SmokeUITests.xcscheme'


def unused_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def wait_until(predicate, children, description, seconds=30):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if any(child.poll() is not None for child in children):
            raise RuntimeError(f'Process exited while waiting for {description}; inspect run logs')
        try:
            if predicate():
                return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.1)
    raise TimeoutError(f'Timed out waiting for {description}')


def healthy(url):
    with urllib.request.urlopen(url, timeout=1) as response:
        return response.status == 200


def stop(child):
    if child.poll() is not None:
        return
    os.killpg(child.pid, signal.SIGTERM)
    try:
        child.wait(timeout=12)
    except subprocess.TimeoutExpired:
        os.killpg(child.pid, signal.SIGKILL)
        child.wait()


def main():
    def interrupted(signum, _frame):
        raise SystemExit(128 + signum)
    signal.signal(signal.SIGTERM, interrupted)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--device', required=True, help='Installed simulator UDID')
    parser.add_argument('--server-binary', type=Path, default=ROOT / 'data/mobile-integration/server')
    parser.add_argument('--metro-url', default='http://127.0.0.1:8081')
    parser.add_argument('--appearance', choices=('light', 'dark'), help='Temporarily set simulator appearance; restore it afterward')
    content_sizes = ('extra-small', 'small', 'medium', 'large', 'extra-large', 'extra-extra-large', 'extra-extra-extra-large',
        'accessibility-medium', 'accessibility-large', 'accessibility-extra-large', 'accessibility-extra-extra-large', 'accessibility-extra-extra-extra-large')
    parser.add_argument('--content-size', choices=content_sizes, help='Temporarily set simulator Dynamic Type size; restore it afterward')
    scope = parser.add_mutually_exclusive_group()
    scope.add_argument('--visual-only', action='store_true', help='Capture Modern HTTP login, Friends and chat; skip theme/Keychain lifecycle checks')
    scope.add_argument('--v3-only', action='store_true', help='Test Classic v3 over HTTP and TCP, appearance restart and live switching across all three themes')
    scope.add_argument('--rooms-only', action='store_true', help='Test room catalog, favorites, participants, messaging, drafts, all three themes and leave over HTTP/TCP')
    scope.add_argument('--features-only', action='store_true', help='Test original room moderation/votes, private blocks, groups and linked page forms over Modern HTTP and Classic v3 TCP')
    scope.add_argument('--unread-only', action='store_true', help='Test whole-tab unread phases, theme retention, clearing and real system Reduce Motion over HTTP/TCP')
    scope.add_argument('--modern-only', action='store_true', help='Test Modern native glass/navigation/menus/alerts/sheet detents, private/room chat and unread over HTTP')
    scope.add_argument('--commands-only', action='store_true', help='Test slash suggestions, retained draft/focus, gift completion and explicit private/room action sends over Modern HTTP and Classic v3 TCP')
    args = parser.parse_args()
    metro = urllib.parse.urlparse(args.metro_url)
    if metro.scheme != 'http' or metro.hostname not in ('127.0.0.1', 'localhost', '::1'):
        parser.error('--metro-url must be the local Metro HTTP origin')
    binary = args.server_binary.resolve()
    if not binary.is_file() or not PROJECT_FILE.is_file():
        parser.error('Build cmd/omni and generate/install the iOS project first; see tools/mobile/README.md')
    if not healthy(args.metro_url.rstrip('/') + '/status'):
        parser.error('Start the existing app-local Metro server before running the smoke test')
    if SCHEME.exists():
        parser.error('Mig33SmokeUITests scheme already exists; review/restore the generated project before continuing')
    runs = ROOT / 'data/mobile-ui/runs'
    runs.mkdir(parents=True, exist_ok=True)
    output = Path(tempfile.mkdtemp(prefix='run-', dir=runs))
    print(f'Native UI evidence: {output}', flush=True)
    http_port = unused_port()
    while http_port in (8080, 9119):
        http_port = unused_port()
    tcp_port = unused_port()
    while tcp_port == http_port or tcp_port in (8080, 9119):
        tcp_port = unused_port()
    origin = f'http://127.0.0.1:{http_port}'
    control_port = unused_port()
    while control_port in (http_port, tcp_port, 8080, 9119):
        control_port = unused_port()
    control_token = secrets.token_hex(24)
    config = {
        'test_environment': {
            'MIG33_SMOKE_HTTP_PORT': str(http_port),
            'MIG33_SMOKE_TCP_PORT': str(tcp_port),
            'MIG33_SMOKE_DEEP_LINK': 'exp+mig33-mobile://?' + urllib.parse.urlencode({
                '__expo_url': args.metro_url,
                '__expo_disable_fab': '1',
                '__expo_disable_auto_launch': '1',
            }),
            'MIG33_SMOKE_VISUAL_ONLY': '1' if args.visual_only else '0',
            'MIG33_SMOKE_V3_ONLY': '1' if args.v3_only else '0',
            'MIG33_SMOKE_ROOMS_ONLY': '1' if args.rooms_only else '0',
            'MIG33_SMOKE_FEATURES_ONLY': '1' if args.features_only else '0',
            'MIG33_SMOKE_UNREAD_ONLY': '1' if args.unread_only else '0',
            'MIG33_SMOKE_MODERN_ONLY': '1' if args.modern_only else '0',
            'MIG33_SMOKE_COMMANDS_ONLY': '1' if args.commands_only else '0',
            'MIG33_SMOKE_CONTROL_URL': f'http://127.0.0.1:{control_port}/' if args.features_only or args.unread_only or args.modern_only else '',
            'MIG33_SMOKE_CONTROL_TOKEN': control_token if args.features_only or args.unread_only or args.modern_only else '',
        },
        'device': args.device,
        'server_binary': str(binary),
        'appearance': args.appearance,
        'content_size': args.content_size,
        'scope': ('Modern HTTP and Classic v3 TCP slash commands: context filtering, dismiss/reopen, insertion without execution, retained draft/caret/keyboard responder, gift subcommands and explicit private/room action sends; Modern picker geometry. No system accessibility settings changes.'
            if args.commands_only else 'Modern HTTP: original login mark, stable arrival/native glass navigation, private and room round trips, native menus/alerts, medium-to-large sheet drag, text editing, retained browse state and unread on/off/clear; real Reduce Transparency and Increase Contrast passes with restoration. Software-keyboard layout is not verified unless a visible-keyboard geometry capture is recorded. Appearance follows --appearance.'
            if args.modern_only else 'All three themes over HTTP/TCP: real incoming unread messages, complete-tab on/off phases, steady Reduce Motion rendering, theme retention and clear-on-select. System Reduce Motion is changed through Settings and restored.'
            if args.unread_only else 'Modern HTTP and Classic v3 TCP original feature flows: room creation/Unicode forms, moderator mute/unmute/kick, ordinary706 votes, personal block/unblock, private-group creation/invite/participants/message/leave; isolated demo data and real gateway peers.'
            if args.features_only else 'Modern HTTP and Classic v3 TCP rooms: catalog/filter, favorites, join welcome, participants, room round trip, two-room drafts, live switching across all three themes and leave; isolated demo data.'
            if args.rooms_only else 'Modern HTTP login, native glass Friends, inline chat round trip, menu and logout captures; isolated demo data.'
            if args.visual_only else 'Classic v3 HTTP/TCP Write Message, restart persistence, live conversation and draft across all three themes; isolated demo data.'
            if args.v3_only else 'Modern and Classic v3 HTTP, Classic v4 TCP; native glass, theme persistence and live chat/draft preservation; Friends, menus, logout; Keychain remember/auto-login/delete across restarts; isolated demo data.'),
    }
    config_path = output / 'config.json'
    config_path.write_text(json.dumps(config, indent=2) + '\n')
    env = {key: value for key, value in os.environ.items() if not key.startswith('MIG33_')}
    env['EXPO_NO_TELEMETRY'] = '1'
    backup = PROJECT_FILE.read_bytes()
    (output / 'project.pbxproj.before').write_bytes(backup)
    children = []
    changed_project = False
    previous_appearance = None
    previous_content_size = None
    result = output / 'Smoke.xcresult'
    try:
        if args.appearance:
            current = subprocess.check_output(['xcrun', 'simctl', 'ui', args.device, 'appearance'], text=True).strip()
            if current not in ('light', 'dark'):
                raise RuntimeError(f'Cannot safely restore simulator appearance: {current}')
            previous_appearance = current
            subprocess.run(['xcrun', 'simctl', 'ui', args.device, 'appearance', args.appearance], check=True)
            (output / 'appearance.json').write_text(json.dumps({'before': current, 'requested': args.appearance}, indent=2) + '\n')
        if args.content_size:
            current = subprocess.check_output(['xcrun', 'simctl', 'ui', args.device, 'content_size'], text=True).strip()
            if current not in content_sizes:
                raise RuntimeError(f'Cannot safely restore simulator content size: {current}')
            previous_content_size = current
            subprocess.run(['xcrun', 'simctl', 'ui', args.device, 'content_size', args.content_size], check=True)
            (output / 'content-size.json').write_text(json.dumps({'before': current, 'requested': args.content_size}, indent=2) + '\n')
        with contextlib.ExitStack() as stack:
            def spawn(command, name):
                log = stack.enter_context((output / name).open('w'))
                child = subprocess.Popen(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
                children.append(child)
                return child
            server = spawn([str(binary), '-config-dir', str(ROOT / 'configs'), '-env', '',
                '-http', f'127.0.0.1:{http_port}', '-tcp', f'127.0.0.1:{tcp_port}',
                '-public-url', origin, '-db', ':memory:', '-operators', ''], 'server.log')
            wait_until(lambda: healthy(origin + '/healthz'), [server], 'isolated Go listeners')
            ready = output / 'peer-ready'
            peer_command = ['mise', '-C', str(ROOT / 'apps/mobile'), 'exec', '--', 'node',
                str(ROOT / 'tools/mobile/smoke-peer.mjs'), '--origin', origin,
                '--ready', str(ready), '--events', str(output / 'peer-events.jsonl')]
            if args.rooms_only or args.modern_only or args.commands_only:
                peer_command.append('--rooms')
            if args.commands_only:
                peer_command.append('--commands')
            if args.features_only:
                peer_command.extend(['--features', '--control-port', str(control_port), '--control-token', control_token])
            elif args.unread_only or args.modern_only:
                peer_command.extend(['--unread', '--control-port', str(control_port), '--control-token', control_token])
            peer = spawn(peer_command, 'peer.log')
            wait_until(ready.is_file, [server, peer], 'Bob echo peer')
            print(f'Isolated HTTP {http_port}, TCP {tcp_port}; Bob is ready', flush=True)
            changed_project = True
            # mise resolves the pinned Ruby directly. `bundle exec ruby` adds
            # a second PATH lookup that can select macOS Ruby under Bundler's
            # modern load path when system bins precede mise's bins.
            with (output / 'prepare.log').open('w') as log:
                preparation = subprocess.run(['mise', '-C', str(ROOT / 'apps/mobile'), '-E', 'ios', 'exec', '--',
                    'ruby', '-rbundler/setup', str(ROOT / 'tools/mobile/prepare_ios_smoke.rb'),
                    str(PROJECT), str(config_path)], cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
            if preparation.returncode:
                raise RuntimeError(f'XCTest target preparation failed; inspect {output / "prepare.log"}')
            print('Building/running XCTest; progress is in xcodebuild.log', flush=True)
            build = spawn(['mise', '-C', str(ROOT / 'apps/mobile'), '-E', 'ios', 'exec', '--',
                'xcodebuild', 'test', '-workspace', str(ROOT / 'apps/mobile/ios/mig33.xcworkspace'),
                '-scheme', 'Mig33SmokeUITests', '-configuration', 'Debug',
                '-destination', 'platform=iOS Simulator,id=' + args.device,
                '-parallel-testing-enabled', 'NO', '-maximum-concurrent-test-simulator-destinations', '1',
                '-derivedDataPath', str(ROOT / 'data/mobile-ui/DerivedData'),
                '-resultBundlePath', str(result), 'CODE_SIGNING_ALLOWED=YES',
                'CODE_SIGN_IDENTITY=-'], 'xcodebuild.log')
            returncode = build.wait(timeout=1200)
            if returncode:
                raise RuntimeError(f'XCTest failed ({returncode}); inspect {output / "xcodebuild.log"}')
            events = [json.loads(line) for line in (output / 'peer-events.jsonl').read_text().splitlines()]
            transports = ['http'] if args.visual_only or args.modern_only else ['http', 'tcp']
            if args.commands_only:
                # One explicit private wave and one room wave per transport.
                # Extra action deliveries would expose completion auto-sending.
                actions = [event for event in events if event.get('event') == 'command-action']
                for kind in (1, 3):
                    matching = [event for event in actions if event.get('kind') == kind
                        and event.get('body') == 'alice waves.' and (kind != 3 or event.get('target') == 'Lobby')]
                    if len(matching) != 2:
                        raise AssertionError(f'Expected exactly two explicit original500/content6 actions for kind {kind}; received {len(matching)}')
            for transport in transports:
                if args.commands_only:
                    continue
                if args.unread_only:
                    suffixes = ['pulsing', 'steady'] if transport == 'http' else ['pulsing']
                    for suffix in suffixes:
                        if not any(event.get('event') == 'feature-private-send'
                            and event.get('body') == 'Unread ' + transport.upper() + ' ' + suffix for event in events):
                            raise AssertionError('Missing real incoming unread delivery for ' + transport + ' ' + suffix)
                    continue
                body = ('Hello room from iOS ' if args.rooms_only else 'Hello from iOS ') + transport.upper()
                if not any(event.get('event') == 'replied' and event.get('body') == body
                    and event.get('kind') == (3 if args.rooms_only else 1)
                    and (not args.rooms_only or event.get('target') == 'Lobby') for event in events):
                    raise AssertionError('Missing acknowledged round-trip peer reply for ' + transport)
                if args.rooms_only and not any(event.get('event') == 'replied' and event.get('kind') == 1
                    and event.get('body') == 'Hello from iOS ' + transport.upper() for event in events):
                    raise AssertionError('Missing private round trip opened from room participants for ' + transport)
                if args.modern_only:
                    if not any(event.get('event') == 'replied' and event.get('kind') == 3
                        and event.get('body') == 'Hello room from iOS HTTP' for event in events):
                        raise AssertionError('Missing Modern room round trip')
                    if not any(event.get('event') == 'feature-private-send' and event.get('body') == 'Modern unread HTTP' for event in events):
                        raise AssertionError('Missing Modern unread fixture delivery')
                if args.features_only:
                    if not any(event.get('event') == 'feature-photo-send' and event.get('transport') == transport for event in events):
                        raise AssertionError('Missing original500 photo delivery for ' + transport)
                    if not any(event.get('event') == 'replied' and event.get('kind') == 2
                        and event.get('body') == 'Hello group from iOS ' + transport.upper() for event in events):
                        raise AssertionError('Missing acknowledged private-group round trip for ' + transport)
                    for accepted, prefix in [(False, 'Muted probe '), (True, 'Unmuted probe ')]:
                        if not any(event.get('event') == 'feature-room-send' and event.get('accepted') is accepted
                            and event.get('body') == prefix + transport.upper() for event in events):
                            raise AssertionError('Missing real moderator enforcement probe: ' + prefix + transport)
                    for actor in ['carol', 'hailam']:
                        if not any(event.get('event') == 'feature-vote' and event.get('actor') == actor
                            and event.get('room') == 'Vote ' + transport.upper() for event in events):
                            raise AssertionError('Missing original706 vote from ' + actor + ' for ' + transport)
            keyboard_layout_verified = args.modern_only and "Added attachment named 'modern-private-software-keyboard'" in (output / 'xcodebuild.log').read_text()
            (output / 'result.json').write_text(json.dumps({'passed': True, 'transports': transports,
                'appearance': args.appearance, 'visual_only': args.visual_only, 'v3_only': args.v3_only,
                'content_size': args.content_size,
                'rooms_only': args.rooms_only, 'native_glass': True,
                'features_only': args.features_only,
                'unread_only': args.unread_only,
                'modern_only': args.modern_only,
                'commands_only': args.commands_only,
                'software_keyboard_layout': 'verified' if keyboard_layout_verified else 'not_verified',
                'themes': ({'http': 'modern', 'tcp': 'classic-v3', 'checks': ['slash-context-filtering', 'dismiss-reopen', 'completion-not-execution',
                    'draft-caret-keyboard-responder', 'gift-subcommands', 'picker-geometry', 'original500-content6-explicit-sends']} if args.commands_only else
                    {'http': 'modern', 'checks': ['authentic-login', 'glass-arrival', 'native-menus-alerts', 'sheet-detents', 'text-editing', 'browse-state-retention', 'unread',
                    'reduce-transparency-solid', 'increase-contrast-controls'] + (['software-keyboard-layout'] if keyboard_layout_verified else [])} if args.modern_only else
                    {'http': ['modern', 'classic', 'classic-v3'], 'tcp': ['modern', 'classic', 'classic-v3'],
                    'checks': ['unread-on-off', 'unread-theme-retention', 'unread-selected-clear', 'system-reduce-motion-steady']} if args.unread_only else
                    {'http': 'modern', 'tcp': 'classic-v3', 'checks': ['native-feature-actions', 'original-page-forms']} if args.features_only else
                    {'http': 'modern', 'tcp': 'classic-v3', 'checks': ['live-room-conversation', 'per-room-unsent-drafts', 'all-three-themes']} if args.rooms_only else
                    {'http': 'modern', 'checks': []} if args.visual_only else
                    {'http': 'classic-v3', 'tcp': 'classic-v3', 'checks': ['restart-persistence', 'live-conversation', 'unsent-draft', 'all-three-themes']} if args.v3_only else
                    {'http': ['modern', 'classic-v3'], 'tcp': 'classic', 'checks': ['restart-persistence', 'live-conversation', 'unsent-draft', 'all-three-themes']}),
                'rooms': ['categories', 'local-filter', 'favorite-add-remove', 'join-welcome', 'participants', 'participant-private-chat', 'round-trip', 'leave'] if args.rooms_only else [],
                'features': ['room-create-unicode', 'room-info', 'mute-send-rejected', 'unmute-send-accepted', 'kick-membership-removed',
                    'ordinary-vote-kick', 'block-suppresses-delivery', 'unblock-restores-delivery', 'photo-preview', 'composer-color-emoticon', 'group-create-invite-participants-round-trip-leave'] if args.features_only else [],
                'keychain': [] if args.visual_only or args.v3_only or args.rooms_only or args.features_only or args.unread_only or args.modern_only or args.commands_only else ['remember', 'restore', 'auto-login', 'delete'],
                'screenshots': 'attachments/', 'xcresult': result.name}, indent=2) + '\n')
            print('PASS Modern HTTP and Classic v3 TCP slash completion, retained draft/focus, context filtering and explicit original private/room actions' if args.commands_only else
                ('PASS Modern HTTP native presentation, text editing, retained browse state, private/room chat, unread and system accessibility settings; software-keyboard layout '
                + ('verified with visible-keyboard geometry capture' if keyboard_layout_verified else 'NOT VERIFIED')) if args.modern_only else
                'PASS native HTTP/TCP whole-tab unread, all themes, clearing and system Reduce Motion' if args.unread_only else
                'PASS native HTTP/TCP moderation, votes, blocks, groups and original page forms' if args.features_only else
                'PASS native HTTP/TCP rooms, favorites, participants, drafts and all three themes' if args.rooms_only else
                'PASS Modern HTTP visual check' if args.visual_only else 'PASS Classic v3 HTTP/TCP and all three theme transitions' if args.v3_only
                else 'PASS native HTTP/TCP UI, all three themes and Keychain lifecycle', flush=True)
    finally:
        try:
            for child in reversed(children):
                stop(child)
            if changed_project:
                PROJECT_FILE.write_bytes(backup)
                SCHEME.unlink(missing_ok=True)
        finally:
            try:
                if previous_appearance is not None:
                    subprocess.run(['xcrun', 'simctl', 'ui', args.device, 'appearance', previous_appearance], check=True)
                    (output / 'appearance.json').write_text(json.dumps({'before': previous_appearance,
                        'requested': args.appearance, 'restored': previous_appearance}, indent=2) + '\n')
            finally:
                if previous_content_size is not None:
                    subprocess.run(['xcrun', 'simctl', 'ui', args.device, 'content_size', previous_content_size], check=True)
                    (output / 'content-size.json').write_text(json.dumps({'before': previous_content_size,
                        'requested': args.content_size, 'restored': previous_content_size}, indent=2) + '\n')
        if result.exists():
            subprocess.run(['xcrun', 'xcresulttool', 'export', 'attachments', '--path', str(result),
                '--output-path', str(output / 'attachments')], cwd=ROOT, check=False)
        print(f'Evidence: {output}', flush=True)


if __name__ == '__main__':
    main()
