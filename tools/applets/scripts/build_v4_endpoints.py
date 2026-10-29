#!/usr/bin/env python3
"""Build a classified, bytecode-linked v4 endpoint catalog without network access."""
import json
from pathlib import Path
from urllib.parse import urlsplit, parse_qs

ROOT = Path(__file__).resolve().parents[3]
DATA = ROOT / 'research/applets/v4/docs/data'
raw = json.loads((DATA / 'protocol-endpoints.json').read_text())
notes_path = DATA / 'endpoint-route-notes.json'
notes_document = json.loads(notes_path.read_text())
notes = notes_document.get('routes', notes_document)
if isinstance(notes, list):
    notes = {item['literal']: item for item in notes}

purposes = {
    ('account','get_history'): 'Account history',
    ('account','home'): 'Account overview',
    ('account','recharge_credit'): 'Recharge credit',
    ('account','transfer_credit'): 'Transfer credit page',
    ('avatar','home'): 'Avatar selection',
    ('bot','bot_list'): 'Games / bot list',
    ('buzz','home'): 'Buzz page',
    ('call','home'): 'Call setup page',
    ('chatroom','create'): 'Create chat room page',
    ('chatroom','search'): 'Search chat rooms',
    ('chatroom','setup'): 'Room information / settings',
    ('email','compose'): 'Compose mail',
    ('group','add_to_group'): 'Link / add room to a group',
    ('group','home'): 'Group home',
    ('group','list_chatrooms'): 'Group chat rooms',
    ('group','list_groups_home'): 'Discover groups',
    ('group','list_my_groups'): 'My groups',
    ('group','members'): 'Group members',
    ('group','view_forum'): 'Group forum',
    ('help','contact_us'): 'Contact support page',
    ('help','main'): 'Help home',
    ('im','setup_im'): 'External IM account setup',
    ('leaderboard','home'): 'Leaderboard',
    ('location','checkin'): 'Location check-in page',
    ('lookout','home'): 'Lookout page',
    ('merchant','dashboard'): 'Merchant dashboard',
    ('migworld','get_mobpage'): 'migWorld / discovery page',
    ('photo','home'): 'Photos / scrapbook page',
    ('photos','wall'): 'migPhotos wall',
    ('profile','edit'): 'Edit profile page',
    ('profile','home'): 'Own or selected user profile',
    ('profile','search'): 'Search people',
    ('profile','user_blog_posts'): 'User blog posts',
    ('report','report_abuse'): 'Report room / group-chat abuse page',
    ('settings','block_list'): 'Manage blocked users',
    ('settings','call_by_name'): 'Call-by-name settings',
    ('settings','change_password'): 'Change password page',
    ('settings','change_tab_order'): 'Tab-order settings',
    ('settings','chat_privacy'): 'Chat privacy settings',
    ('settings','profile_privacy_settings'): 'Profile privacy settings',
    ('settings','view_update_settings'): 'Activity update settings',
    ('settings','change_mobile'): 'Change mobile number page',
    ('sms','home'): 'Compose SMS page',
    ('store','home'): 'Gift or theme store',
    ('store','main'): 'Store home',
}

special = {
    'http://gateway.mig33.com/': ('binary_http_gateway', 'GET polling / POST packet bytes', 'Binary packet stream'),
    'http://www.mig33.com/sites/c=migworld&v=midlet&a=new_home': ('internal_browser_home_key', 'No network request in the recovered initialization call site', 'Local document/cache/history key; exact unusual URL-shaped string retained'),
    'http://www.mig33.com/sites/index.php?c=help&v=wap&a=apn': ('external_browser', 'platformRequest; handset browser chooses HTTP behavior', 'Connection help page'),
    'http://www.mig33.com/sites/index.php?c=help&v=wap&a=tc': ('external_browser', 'platformRequest; handset browser chooses HTTP behavior', 'Terms and conditions page'),
    'http://www.mig33.com/sites/index.php?c=registration_api&v=midlet&a=check_username&username=%s': ('registration_check', 'GET through local job 1005', 'Raw text consumed by the username suggestion parser'),
    'http://www.mig33.com/sites/lib/ga_midlet.php?': ('analytics_prefix', 'GET through separate analytics client', 'Response body 1 disables tracking'),
    'http://www.mig33.com/sites/resources/language/v4.6/l': ('language_catalog', 'GET through local job 1001', 'Binary packet 1002 containing language triples'),
}

records = []
excluded = []
for index, item in enumerate(raw, 1):
    literal = item['literal']
    if literal in ('http://', 'https://', 'socket://'):
        excluded.append({'literal': literal, 'reason': 'Scheme prefix, not a complete endpoint', 'references': item['references']})
        continue
    record = {'id': f'L{index:03d}', 'literal': literal, 'references': item['references']}
    if literal.startswith('/sites/index.php?'):
        parts = urlsplit(literal)
        query = parse_qs(parts.query, keep_blank_values=True)
        controller, action = query['c'][0], query['a'][0]
        record.update({
            'kind': 'embedded_page_template', 'controller': controller, 'action': action,
            'purpose': purposes[(controller, action)],
            'base': 'WEB_BASE derived from login response 203 field 26',
            'initial_application_method': 'GET',
            'delivery': '922/923 over gateway, or direct HTTP according to transport state',
            'response': 'Server-provided markup rendered by the embedded browser',
            'query_template_components': [{'name': chunk.partition('=')[0], 'value_template': chunk.partition('=')[2]}
                                          for chunk in parts.query.split('&')],
        })
    else:
        kind, method, response = special[literal]
        record.update({'kind': kind, 'initial_application_method': method, 'response': response})
    if literal in notes:
        record['construction_notes'] = notes[literal]
    records.append(record)

extras = [
    {'id': 'G001', 'template': 'socket://gateway.mig33.com:25', 'kind': 'binary_tcp_gateway',
     'construction': 'Default host and port; manual/saved settings can override.',
     'references': ['research/applets/v4/artifacts/normal-flow/bc.java:125', 'research/applets/v4/artifacts/normal-flow/u.java:119']},
    {'id': 'G002', 'template': 'socket://gateway.mig33.com:9119', 'kind': 'binary_tcp_gateway',
     'construction': 'Connection test switches between ports 25 and 9119 for its second attempt.',
     'references': ['research/applets/v4/artifacts/normal-flow/bc.java:459']},
    {'id': 'G003', 'template': '{HTTP_GATEWAY}{session}', 'kind': 'binary_http_session_gateway',
     'construction': 'Append session directly except requests 200, 100, 2, 904; base retained when session is null. GET polls.',
     'references': ['research/applets/v4/artifacts/normal-flow/u.java:505']},
    {'id': 'G004', 'template': '{ASSET_BASE}{assetId}?w={width}&h={height}&a=1&c=1', 'kind': 'dynamic_image',
     'construction': 'Base from login 203 tag 23, stored settings tag 9. IDs beginning http pass through unchanged. A second builder uses G for both dimensions.',
     'references': ['research/applets/v4/artifacts/normal-flow/aj.java:6306']},
    {'id': 'D001', 'template': 'http://www.mig33.com/midlet/member/c_info.php?awtrack=wap', 'kind': 'midlet_install_notification',
     'construction': 'MIDlet-Install-Notify descriptor value; handled by the device application manager, not app network code.',
     'references': ['research/applets/v4/artifacts/extracted/META-INF/MANIFEST.MF', 'research/applets/v4/artifacts/original/mig33-4.62.jad']},
    {'id': 'D002', 'template': 'http://www.mig33.com/wap2/v4_60/24x24/mig33v46.jar', 'kind': 'jar_distribution',
     'construction': 'MIDlet-Jar-URL in the JAD; installation/distribution URL, not a runtime app API.',
     'references': ['research/applets/v4/artifacts/original/mig33-4.62.jad']},
]

catalog = {
    'version': '4.62', 'build': '.000',
    'original_jar_sha256': 'dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c',
    'scope': 'Recovered client URL/path literals plus constructed transport/asset URLs and descriptor URLs. Not an exhaustive inventory of the historical server, or a statement of current availability.',
    'counts': {'raw_literals': len(raw), 'excluded_scheme_prefixes': len(excluded),
               'embedded_page_templates': sum(r['kind'] == 'embedded_page_template' for r in records),
               'classified_url_path_literals': len(records), 'constructed_or_descriptor_templates': len(extras)},
    'literals': records, 'constructed_and_descriptor_urls': extras, 'excluded': excluded,
}
assert catalog['counts']['raw_literals'] == 62
assert catalog['counts']['embedded_page_templates'] == 52
assert catalog['counts']['classified_url_path_literals'] == 59
assert len({r['literal'] for r in records}) == 59
assert {r['literal'] for r in records} | {r['literal'] for r in excluded} == {r['literal'] for r in raw}
assert all(r['literal'] in notes for r in records if '%s' in r['literal'])

(DATA/'endpoints-classified.json').write_text(json.dumps(catalog, indent=2, ensure_ascii=False)+'\n')

rows = []
for record in records:
    if record['kind'] != 'embedded_page_template':
        continue
    ref = record['references'][0]
    filename = Path(ref['path']).name
    evidence = f'[{filename}](../artifacts/bytecode/{filename}), line {ref["line"]}'
    rows.append(f'| {record["id"]} | {record["purpose"]} | `{record["literal"]}` | {evidence} |')
table = '\n'.join(['| ID | Page opened | Exact relative URL template | Original bytecode |',
                    '| --- | --- | --- | --- |', *rows])
document_path = ROOT/'research/applets/v4/docs/endpoints.md'
if document_path.exists():
    document = document_path.read_text()
    start = '<!-- BEGIN GENERATED PAGE ROUTES -->'
    end = '<!-- END GENERATED PAGE ROUTES -->'
    assert document.count(start) == 1 and document.count(end) == 1
    prefix, rest = document.split(start)
    _, suffix = rest.split(end)
    document_path.write_text(prefix+start+'\n\n'+table+'\n\n'+end+suffix)
print(json.dumps(catalog['counts'], indent=2))
