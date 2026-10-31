import React, { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Keyboard, Pressable, ScrollView, StyleSheet, Text, View, type KeyboardTypeOptions } from 'react-native';
import type { Contact, ContactGroup, OwnPresence } from '../mig33/client';
import { useAppearance } from './appearance';
import { legacySelector } from './chatActions';
import { ChatActionSheet } from './screens/ChatActionsScreens';
import type { Mig33Session } from './session/useMig33Session';
import { V4Button, V4Checkbox, V4ChoiceGroup, V4Field, V4Icon, type V4MenuItem } from './v4/primitives';

export interface RosterConfirmation {
  title: string;
  message: string;
  label: string;
  run: () => void | Promise<void>;
}
export interface RosterActionsOptions {
  openPage: (url: string) => void;
  openChat: (username: string) => void;
  showMenu: (menu: { title: string; items: V4MenuItem[] }) => void;
  confirm: (confirmation: RosterConfirmation) => void;
  extendMenu?: (items: V4MenuItem[], category: number, context?: { groupId: number }) => V4MenuItem[];
}
interface Field {
  name: string;
  label: string;
  required?: boolean;
  readOnly?: boolean;
  multiline?: boolean;
  keyboard?: KeyboardTypeOptions;
  limit?: number;
}
interface Editor {
  id: number;
  title: string;
  fields: readonly Field[];
  values: Record<string, string>;
  submitLabel: string;
  groupId?: number;
  shareMobile?: boolean;
  error?: string;
  information?: string;
  successMessage?: () => string;
  returnTo?: Editor;
  createdGroup?: boolean;
  submit: (values: Readonly<Record<string, string>>, groupId?: number, shareMobile?: boolean) => Promise<boolean>;
}

/** Source forms/menu roles: v4 screens.md57/9, K/Q and group commands.
 * Touch adaptation adds a Contact Groups entry so every group operation stays
 * reachable without keypad focus or a hidden long-press gesture.
 */
export function useRosterActions(session: Mig33Session, options: RosterActionsOptions) {
  const { theme, metrics, modern, v3 } = useAppearance();
  const [editor, setEditor] = useState<Editor | null>(null);
  const latestEditor = useRef(editor);
  latestEditor.current = editor;
  const [expandedGroups, setExpandedGroups] = useState(false);
  const [saving, setSaving] = useState(false);
  const [createdFor, setCreatedFor] = useState<{ editorId: number; name: string } | null>(null);
  const sequence = useRef(0);
  const submitting = useRef(false);
  const mounted = useRef(true);
  const latestSession = useRef(session);
  latestSession.current = session;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => { if (!session.online) { setEditor(null); setCreatedFor(null); } }, [session.online]);
  useEffect(() => {
    if (!createdFor) return;
    const group = session.groups.find((entry) => entry.name === createdFor.name);
    if (!group) return;
    setEditor((current) => current?.id === createdFor.editorId ? { ...current, groupId: group.id } : current);
    setCreatedFor(null);
  }, [createdFor, session.groups]);
  const styles = useMemo(() => StyleSheet.create({
    content: { paddingHorizontal: modern ? 18 : 8, paddingTop: 8, paddingBottom: 16 },
    label: { color: theme.label, fontSize: metrics.font, marginBottom: 5 },
    choice: { minHeight: metrics.touchTarget, borderWidth: 1, borderColor: theme.border,
      backgroundColor: theme.body, borderRadius: modern ? 12 : 0, padding: 10,
      flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 10 },
    choices: { marginBottom: 12, borderWidth: 1, borderColor: theme.groupBorder, borderRadius: modern ? 12 : 0, overflow: 'hidden' },
    choiceRow: { minHeight: metrics.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 10,
      paddingVertical: 10, paddingHorizontal: 12, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: theme.groupBorder },
    selected: { backgroundColor: theme.selected }, text: { flex: 1, color: theme.text, fontSize: metrics.font },
    selectedText: { color: theme.selectedText }, error: { color: theme.text, fontSize: metrics.smallFont, lineHeight: 20,
      borderLeftWidth: 3, borderColor: theme.focus, paddingLeft: 10, marginVertical: 12 },
    actions: { marginTop: 10, flexDirection: 'row', justifyContent: 'flex-end' },
  }), [theme, metrics, modern]);

  function closePrompt() { setEditor(null); setExpandedGroups(false); setCreatedFor(null); Keyboard.dismiss(); }
  function begin(form: Omit<Editor, 'id'>) {
    Keyboard.dismiss(); setExpandedGroups(false); setEditor({ ...form, id: ++sequence.current });
  }
  function change(name: string, value: string) {
    setEditor((current) => current ? { ...current, values: { ...current.values, [name]: value }, error: undefined } : null);
  }
  async function submit() {
    if (!editor || submitting.current || session.roomAction) return;
    const current = editor;
    const missing = current.fields.find((field) => field.required && !current.values[field.name]?.trim());
    if (missing) {
      setEditor((value) => value?.id === current.id ? { ...value, error: `${missing.label} is required.` } : value);
      return;
    }
    submitting.current = true; setSaving(true); setExpandedGroups(false); Keyboard.dismiss();
    try {
      const saved = await current.submit(current.values, current.groupId, current.shareMobile);
      if (!mounted.current) return;
      if (!saved) {
        setEditor((value) => value?.id === current.id ? { ...value, error: 'The change was not saved. Review the message and try again.' } : value);
        return;
      }
      const information = current.successMessage?.();
      const next: Editor | null = information ? { id: ++sequence.current, title: current.title, fields: [], values: {},
        submitLabel: 'OK', information, submit: async () => true } : current.returnTo ?? null;
      setEditor((value) => value?.id === current.id ? next : value);
      if (current.returnTo && current.createdGroup) {
        setCreatedFor({ editorId: current.returnTo.id, name: current.values.name.trim() });
      }
      setExpandedGroups(false);
    } catch (error) {
      if (mounted.current) setEditor((value) => value?.id === current.id
        ? { ...value, error: error instanceof Error ? error.message : 'The change could not be saved.' } : value);
    } finally {
      submitting.current = false;
      if (mounted.current) setSaving(false);
    }
  }
  function createContactGroup(returnTo?: Editor) {
    begin({ title: 'Create Contact Group', fields: [{ name: 'name', label: 'Group Name', required: true, limit: 320 }],
      values: { name: '' }, submitLabel: 'Continue', returnTo, createdGroup: true,
      submit: (values) => session.createContactGroup(values.name.trim()) });
  }
  function addFriend() {
    begin({ title: 'Add a Friend', fields: [
      { name: 'username', label: 'mig33 Username', required: true, limit: 320 },
      { name: 'display', label: 'Display Name (optional)', limit: 320 },
    ], values: { username: '', display: '' }, groupId: -1, shareMobile: false, submitLabel: 'Add',
    submit: (values, groupId, shareMobile) => session.addContact(values.username.trim(), {
      ...(values.display.trim() && { display: values.display.trim() }), groupId, shareMobile,
    }) });
  }
  function openInviteToMig33() {
    let acknowledgement = '';
    begin({ title: 'Invite 2 mig33', fields: [
      { name: 'phone', label: 'Mobile Number', required: true, keyboard: 'phone-pad', limit: 320 },
      { name: 'name', label: 'Your Name', required: true, limit: 320 },
    ], values: { phone: '', name: session.username }, submitLabel: 'Invite',
    // Native900 is an invitation request. Preserve its actual acknowledgment;
    // a local provider request does not establish delivery to a real handset.
    submit: async (values) => {
      const [ack] = await session.requestOriginal(900,
        (packet) => { packet.setText(1, values.phone.trim()).setText(2, values.name.trim()); }, [1], 1);
      acknowledgement = ack.text(1) || 'Invitation request accepted.';
      return true;
    }, successMessage: () => acknowledgement });
  }
  function openStatus() {
    begin({ title: 'Update Status Msg', fields: [{ name: 'status', label: 'Status Message', multiline: true, limit: 320 }],
      // Source starts with "is ". Preserve a known status when reopening.
      values: { status: session.ownStatus || 'is ' }, submitLabel: 'Continue', submit: (values) => session.setStatus(values.status) });
  }
  function openCall(target = '') { options.openPage(legacySelector('call', 'home', { f: 'call', number_to_call: target })); }
  function openSMS(target = '') { options.openPage(legacySelector('sms', 'home', { f: 'sms', to_number: target })); }
  function editContact(contact: Contact) {
    begin({ title: 'Contact Details', fields: [
      { name: 'display', label: 'Display Name', required: true, limit: 320 },
      { name: 'username', label: 'mig33 Username', readOnly: true },
      { name: 'mobile', label: 'Mobile Number', keyboard: 'phone-pad', limit: 320 },
    ], values: { display: contact.display, username: contact.username, mobile: contact.packet.text(8) ?? '' },
    submitLabel: 'Update', submit: (values) => session.updateContact(contact.id, { display: values.display.trim(), mobile: values.mobile.trim() }) });
  }
  function moveContact(contact: Contact) {
    begin({ title: 'Move Contact To', fields: [], values: {}, groupId: contact.groupId, submitLabel: 'Move',
      submit: (_values, groupId) => session.moveContact(contact.id, groupId ?? -1) });
  }
  function settingsItems(): V4MenuItem[] {
    const page = (id: string, label: string, action: string): V4MenuItem => ({ id, label, testID: `roster.settings.${id}`,
      onPress: () => options.openPage(legacySelector('settings', action)) });
    return [
      page('password', 'Change Password', 'change_password'), page('tabs', 'Tab Order', 'change_tab_order'),
      { id: 'privacy', label: 'Privacy', icon: 'arrowRight', testID: 'roster.settings.privacy', onPress: () => options.showMenu({ title: 'Privacy', items: [
        page('profile', 'Profile', 'profile_privacy_settings'), page('updates', 'My Updates', 'view_update_settings'),
        page('calls', 'Call by Name', 'call_by_name'), page('chat', 'Private Chat', 'chat_privacy'), page('blocks', 'Manage Block List', 'block_list'),
      ] }) },
      page('mobile', 'Change Mobile #', 'change_mobile'),
    ];
  }
  function contactExtraItems(contact: Contact): V4MenuItem[] {
    return [
      { id: 'contact-details', label: v3 ? 'View Details' : 'Contact Details', testID: 'roster.contact.details', onPress: () => editContact(contact) },
      { id: 'contact-move', label: 'Move Contact To', icon: 'arrowRight', testID: 'roster.contact.move', onPress: () => moveContact(contact) },
      { id: 'contact-name', label: 'Change Display Name', testID: 'roster.contact.rename', onPress: () => begin({
        title: 'Change Display Name', fields: [{ name: 'display', label: 'Display Name', required: true, limit: 320 }],
        values: { display: contact.display }, submitLabel: 'Continue',
        submit: (values) => session.updateContact(contact.id, { display: values.display.trim() }),
      }) },
      { id: 'contact-delete', label: v3 ? 'Delete Contact' : 'Delete Friend', testID: 'roster.contact.delete', onPress: () => options.confirm({
        title: 'Delete Friend', message: `You are about to delete ${contact.display || contact.username} from your friend list.`, label: 'Delete',
        run: async () => { if (await session.deleteContact(contact.id) && contact.username) session.closeConversation(contact.username); },
      }) },
    ];
  }
  function groupItems(group: ContactGroup): V4MenuItem[] {
    // Include hidden/offline/external records in the original empty-group rule.
    const nonempty = session.contacts.some((contact) => contact.groupId === group.id);
    const items: V4MenuItem[] = [
      { id: 'group-rename', label: 'Rename', testID: 'roster.group.rename', onPress: () => begin({
        title: 'Rename Group', fields: [{ name: 'name', label: 'New Group Name', required: true, limit: 320 }],
        values: { name: group.name }, submitLabel: 'Continue', submit: (values) => session.renameContactGroup(group.id, values.name.trim()),
      }) },
      { id: 'group-create', label: 'Create Contact Group', testID: 'roster.group.create', onPress: () => createContactGroup() },
      { id: 'group-delete', label: 'Delete', disabled: nonempty, testID: 'roster.group.delete', onPress: () => options.confirm({
        title: 'Delete Group', message: `Delete ${group.name}? Only an empty group can be deleted.`, label: 'Delete',
        run: async () => { await session.deleteContactGroup(group.id); },
      }) },
    ];
    return options.extendMenu?.(items, 5, { groupId: group.id }) ?? items;
  }
  function friendsItems(): V4MenuItem[] {
    return [
      { id: 'add', label: 'Add Friends', testID: 'roster.add', icon: 'arrowRight', onPress: () => options.showMenu({ title: 'Add Friends', items: [
        { id: 'username', label: 'By Username', testID: 'roster.add.username', onPress: addFriend },
        { id: 'phone', label: 'By Phone Number', testID: 'roster.add.phone', onPress: openInviteToMig33 },
      ] }) },
      { id: 'create-contact-group', label: 'Create Contact Group', testID: 'roster.group.create', onPress: () => createContactGroup() },
      { id: 'contact-groups', label: 'Contact Groups', icon: 'arrowRight', testID: 'roster.groups', onPress: () => options.showMenu({ title: 'Contact Groups', items: [
        ...latestSession.current.groups.map((group): V4MenuItem => ({ id: `group:${group.id}`, label: group.name,
          testID: `roster.group.${group.id}`, icon: 'arrowRight',
          onPress: () => options.showMenu({ title: group.name, items: groupItems(group) }),
        })),
        { id: 'create', label: 'Create Contact Group', testID: 'roster.group.create', onPress: () => createContactGroup() },
      ] }) },
      { id: 'refresh', label: 'Refresh Contact List', testID: 'roster.refresh', disabled: session.syncing,
        onPress: () => { void session.syncContacts(); } },
    ];
  }
  function selfItems(): V4MenuItem[] {
    const presence: readonly { value: OwnPresence; label: string; icon: 'online' | 'away' | 'busy' | 'offline' }[] = [
      { value: 1, label: 'Online', icon: 'online' }, { value: 4, label: 'Away', icon: 'away' },
      { value: 3, label: 'Busy', icon: 'busy' }, { value: 99, label: 'Appear Offline', icon: 'offline' },
    ];
    return [
      { id: 'self-status', label: 'Update Status Msg', testID: 'roster.status', onPress: openStatus },
      { id: 'presence', label: 'Set Presence', icon: 'arrowRight', testID: 'roster.presence', onPress: () => options.showMenu({ title: 'Set Presence',
        items: presence.map((item) => ({ id: String(item.value), label: `${item.label}${session.presence === item.value ? ' *' : ''}`,
          icon: item.icon, testID: `roster.presence.${item.value}`, disabled: !!session.roomAction,
          onPress: () => { void session.setPresence(item.value); },
        })),
      }) },
      { id: 'create-contact-group', label: 'Create Contact Group', testID: 'roster.group.create', onPress: () => createContactGroup() },
      { id: 'settings', label: 'Settings', icon: 'arrowRight', testID: 'roster.settings',
        onPress: () => { const items = settingsItems(); options.showMenu({ title: 'Settings', items: options.extendMenu?.(items, 6) ?? items }); } },
    ];
  }

  const busy = saving || !!session.roomAction;
  const groupOptions: { id: number; name: string; disabled?: boolean }[] = [
    { id: -1, name: '<none>' }, { id: -2, name: 'Create Contact Group' }, ...session.groups,
  ];
  if (modern && editor?.groupId !== undefined && !groupOptions.some((group) => group.id === editor.groupId)) {
    // Preserve the captured wire value if a group disappears while this
    // editor is open; the native Picker still needs a matching selection tag.
    groupOptions.unshift({ id: editor.groupId, name: 'Unknown group', disabled: true });
  }
  function chooseGroup(id: number) {
    if (!editor || latestEditor.current?.id !== editor.id || submitting.current || latestSession.current.roomAction) return;
    Keyboard.dismiss();
    if (id === -2) { createContactGroup(editor); return; }
    if (!groupOptions.some((group) => group.id === id && !group.disabled)) return;
    setEditor((value) => value?.id === editor.id ? { ...value, groupId: id } : value);
    setExpandedGroups(false);
  }
  function dismissPrompt() {
    if (!editor) return;
    Keyboard.dismiss(); setExpandedGroups(false); setEditor(editor.returnTo ?? null);
  }
  const promptContent: ReactNode | null = editor ? <ScrollView testID="roster.prompt" keyboardShouldPersistTaps="handled" contentContainerStyle={styles.content}>
      {editor.information && <Text testID="roster.information" style={[styles.text, { flex: 0, marginVertical: 12 }]}>{editor.information}</Text>}
      {editor.fields.map((field) => <V4Field key={field.name} testID={`roster.field.${field.name}`} label={field.label}
        value={editor.values[field.name] ?? ''} editable={!field.readOnly && !busy} multiline={field.multiline}
        maxLength={field.limit} keyboardType={field.keyboard} autoCapitalize="none" autoCorrect={field.multiline === true}
        onChangeText={(value) => change(field.name, value)} style={field.multiline ? { minHeight: 100 } : undefined} />)}
      {editor.groupId !== undefined && (modern ? <V4ChoiceGroup key={`group:${editor.id}`} testID="roster.group.choose"
        label={editor.title === 'Move Contact To' ? 'Group' : 'Add to Group'} value={String(editor.groupId)} variant="menu" disabled={busy}
        options={groupOptions.map((group) => ({ value: String(group.id), label: group.name, disabled: group.disabled,
          testID: `roster.group.option.${group.id === -2 ? 'create' : group.id}` }))}
        onChange={(value) => chooseGroup(Number(value))} /> : <>
        <Text style={styles.label}>{editor.title === 'Move Contact To' ? 'Group' : 'Add to Group'}</Text>
        <Pressable testID="roster.group.choose" accessibilityRole="button" accessibilityState={{ expanded: expandedGroups, disabled: busy }}
          disabled={busy} onPress={() => { Keyboard.dismiss(); setExpandedGroups(!expandedGroups); }} style={styles.choice}>
          <Text style={styles.text}>{editor.groupId === -1 ? '<none>' : session.groups.find((group) => group.id === editor.groupId)?.name ?? 'Unknown group'}</Text>
          <V4Icon name={expandedGroups ? 'arrowDown' : 'arrowRight'} size={12} />
        </Pressable>
        {expandedGroups && <View style={styles.choices}>
          {groupOptions.map((group) => <Pressable key={group.id}
            testID={`roster.group.option.${group.id === -2 ? 'create' : group.id}`} accessibilityRole="button"
            accessibilityState={{ selected: editor.groupId === group.id, disabled: busy }} disabled={busy}
            onPress={() => chooseGroup(group.id)}
            style={[styles.choiceRow, editor.groupId === group.id && styles.selected]}>
            {group.id === -2 && <V4Icon name="plus" size={16} />}
            <Text style={[styles.text, editor.groupId === group.id && styles.selectedText]}>{group.name}</Text>
            {editor.groupId === group.id && <V4Icon name="check" size={14} color={theme.selectedText} />}
          </Pressable>)}
        </View>}
      </>)}
      {editor.shareMobile !== undefined && <V4Checkbox testID="roster.share-mobile" label="Share my mobile number"
        checked={editor.shareMobile} disabled={busy} onChange={(checked) => setEditor((value) => value ? { ...value, shareMobile: checked } : value)} />}
      {editor.error && <Text testID="roster.error" accessibilityRole="alert" style={styles.error}>{editor.error}</Text>}
      <View style={styles.actions}><V4Button testID="roster.submit" label={editor.submitLabel} busy={busy} onPress={() => { void submit(); }} /></View>
    </ScrollView> : null;
  const prompt: ReactNode | null = editor ? <ChatActionSheet embedded visible title={editor.title} onClose={dismissPrompt}>
    {promptContent}
  </ChatActionSheet> : null;
  return { contactExtraItems, friendsItems, selfItems, groupItems, settingsItems, prompt, closePrompt,
    nativePrompt: editor ? { key: `roster:${editor.id}`, title: editor.title, content: promptContent,
      onClose: dismissPrompt, dismissible: !busy } : null,
    openStatus, openInviteToMig33, openCall, openSMS };
}
