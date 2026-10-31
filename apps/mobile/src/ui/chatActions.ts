/** Recovered aj.U() menus. Classic v3 uses the v4 wire contract underneath. */
export type ChatKind = 'private' | 'group' | 'room';
export interface ChatContext { kind: ChatKind; id: string; }
export type ChatAction = 'write' | 'profile' | 'private' | 'add' | 'lookout' | 'gift' | 'invite' | 'photo'
  | 'call' | 'sms' | 'credit' | 'kick' | 'mute' | 'unmute' | 'block' | 'report' | 'info';
export interface ChatActionItem { action: ChatAction; label: string; target?: boolean; }

export function chatActions(kind: ChatKind, v3: boolean): ChatActionItem[] {
  const item = (action: ChatAction, label: string, target = true): ChatActionItem => ({ action, label, target });
  if (v3) {
    const profile = item('profile', "View User's Profile");
    if (kind === 'room') return [profile, item('private', 'Private Chat'), item('kick', 'Kick User'), item('mute', 'Mute User')];
    return [profile, item('add', 'Add to Contact List'), item('invite', 'Invite User', false),
      ...(kind === 'private' ? [item('block', 'Block User')] : []), item('photo', 'Send Photo', false)];
  }
  if (kind === 'room') return [item('profile', 'User Profiles'), item('private', 'Private Chat'), item('add', 'Add as friend'),
    item('gift', 'Give a Gift'), item('call', 'Call User'), item('sms', 'SMS User'), item('credit', 'Transfer Credit'),
    item('kick', 'Kick User'), item('block', 'Block & Mute User'), item('report', 'Report Abuse', false), item('info', 'Room Info', false)];
  if (kind === 'group') return [item('profile', 'User Profiles'), item('invite', 'Add to Chat', false), item('private', 'Private Chat'),
    item('add', 'Add as friend'), item('photo', 'Share Photo', false), item('call', 'Call User'), item('credit', 'Transfer Credit'),
    item('gift', 'Give a Gift'), item('block', 'Block & Mute User'), item('report', 'Report Abuse', false)];
  return [item('add', 'Add as friend'), item('profile', 'User Profile'), item('lookout', 'Lookout'), item('gift', 'Give a Gift'),
    item('invite', 'Add to Chat', false), item('photo', 'Share Photo', false), item('call', 'Call User'), item('sms', 'SMS User'),
    item('credit', 'Transfer Credit'), item('block', 'Block & Mute User')];
}

export function legacySelector(controller: string, action: string, values: Record<string, string> = {}): string {
  return `/sites/index.php?${new URLSearchParams({ c: controller, a: action, v: 'midlet', ...values })}`;
}

/** Page selectors are the original selectors, including their original names. */
export function chatActionPage(action: ChatAction, context: ChatContext, username = ''): string | undefined {
  switch (action) {
    case 'profile': return legacySelector('profile', 'home', { username });
    case 'lookout': return legacySelector('lookout', 'home', { username });
    case 'gift': return legacySelector('store', 'home', { e: '3', ty: '1', username });
    case 'call': return legacySelector('call', 'home', { f: 'call', number_to_call: username });
    case 'sms': return legacySelector('sms', 'home', { f: 'sms', to_number: username });
    case 'credit': return legacySelector('account', 'transfer_credit', { recipient_username: username });
    case 'info': return legacySelector('chatroom', 'setup', { roomName: context.id });
    case 'report': return legacySelector('report', 'report_abuse', {
      type: '4', subject: context.kind === 'room' ? 'Chatroom' : 'Group Chat', subject_id: context.id,
    });
  }
}
