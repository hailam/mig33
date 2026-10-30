import React, { useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator, AppState, FlatList, KeyboardAvoidingView, Platform,
  Pressable, SafeAreaView, ScrollView, StatusBar, StyleSheet, Text, TextInput, View,
} from 'react-native';

import { Mig33Client, type ClientState, type Contact } from './src/mig33/client';
import { HTTPTransport } from './src/mig33/transport/http';
import { createTCPTransport } from './src/mig33/transport/tcp.native';

interface Row { id: string; sender: string; body: string; peer: string; own: boolean; action: boolean; }

export default function App() {
  const [username, setUsername] = useState('alice');
  const [password, setPassword] = useState('');
  const [mode, setMode] = useState<'http' | 'tcp'>('http');
  const [httpURL, setHTTPURL] = useState('http://127.0.0.1:8080');
  const [host, setHost] = useState('127.0.0.1');
  const [port, setPort] = useState('9119');
  const [state, setState] = useState<ClientState>('idle');
  const [error, setError] = useState('');
  const [contacts, setContacts] = useState<readonly Contact[]>([]);
  const [recipient, setRecipient] = useState('bob');
  const [draft, setDraft] = useState('');
  const [rows, setRows] = useState<Row[]>([]);
  const [sending, setSending] = useState(false);
  const [settings, setSettings] = useState(false);
  const client = useRef<Mig33Client | null>(null);
  const unsubscribe = useRef<(() => void) | undefined>(undefined);
  const counter = useRef(0);
  const connecting = useRef(false);
  const sendingClient = useRef<Mig33Client | null>(null);
  const disconnecting = useRef<Mig33Client | null>(null);
  const online = state === 'online';
  const busy = state === 'connecting' || state === 'authenticating';

  useEffect(() => {
    const listener = AppState.addEventListener('change', (next) => {
      // This first slice deliberately ends the live session on backgrounding.
      // It does not silently replay an ambiguous send after a mobile suspend.
      if (next === 'background' && client.current) client.current.close();
    });
    return () => {
      listener.remove();
      unsubscribe.current?.();
      const active = client.current;
      client.current = null;
      sendingClient.current = null;
      active?.close();
    };
  }, []);

  function append(row: Omit<Row, 'id'>) {
    setRows((previous) => [...previous.slice(-199), { ...row, id: String(++counter.current) }]);
  }

  async function connect() {
    if (connecting.current) return;
    connecting.current = true;
    setError('');
    unsubscribe.current?.();
    client.current?.close();
    let next: Mig33Client | undefined;
    try {
      const transport = mode === 'http' ? new HTTPTransport(httpURL) : createTCPTransport(host, Number(port));
      next = new Mig33Client(transport);
      const active = next;
      client.current = next;
      sendingClient.current = null;
      setSending(false);
      unsubscribe.current = next.subscribe((event) => {
        if (client.current !== active) return;
        if (event.type === 'state') {
          setState(event.state);
          if (event.error) setError(event.error.message);
        } else if (event.type === 'contacts') setContacts(event.contacts);
        else if (event.type === 'message') {
          const message = event.message;
          append({
            sender: message.sender, body: message.body,
            peer: message.sender === active.username ? message.target : message.sender,
            own: message.content !== 6 && message.sender === active.username,
            action: message.content === 6,
          });
        }
      });
      setRows([]);
      setContacts([]);
      await next.login({ username, password, platform: `Expo ${Platform.OS}` });
      if (client.current !== next || next.state !== 'online') return;
      setPassword('');
      await next.syncContacts();
    } catch (cause) {
      if (!next || (client.current === next && next.state !== 'closed')) {
        setError(cause instanceof Error ? cause.message : String(cause));
      }
    } finally { connecting.current = false; }
  }

  async function disconnect() {
    const active = client.current;
    if (!active || disconnecting.current === active) return;
    disconnecting.current = active;
    try { await active.logout(); }
    catch (cause) {
      if (client.current === active && active.state !== 'closed') {
        setError(cause instanceof Error ? cause.message : String(cause));
      }
    } finally {
      if (disconnecting.current === active) disconnecting.current = null;
    }
  }

  async function send() {
    const active = client.current;
    if (!draft.trim() || sendingClient.current || !active) return;
    sendingClient.current = active;
    const text = draft;
    const peer = recipient.trim().toLowerCase();
    setSending(true);
    setError('');
    try {
      await active.sendMessage(peer, text);
      if (client.current !== active || active.state !== 'online') return;
      // Supported slash commands return a server-rendered action. Their
      // literal request must not become a second local chat row.
      if (!text.trimStart().startsWith('/')) {
        append({ sender: active.username, body: text, peer, own: true, action: false });
      }
      setDraft('');
    } catch (cause) {
      // Retain the draft. An acknowledgment timeout does not prove the message
      // was undelivered, so there is no automatic resend.
      if (client.current === active && active.state !== 'closed') {
        setError(cause instanceof Error ? cause.message : String(cause));
      }
    } finally {
      if (sendingClient.current === active) {
        sendingClient.current = null;
        setSending(false);
      }
    }
  }

  return (
    <SafeAreaView style={styles.screen}>
      <StatusBar barStyle="dark-content" />
      <KeyboardAvoidingView style={styles.screen} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
        <View style={styles.header}>
          <View><Text style={styles.logo}>mig33</Text><Text style={styles.caption}>A familiar place to connect</Text></View>
          {online && <Pressable accessibilityRole="button" onPress={disconnect}><Text style={styles.link}>Sign out</Text></Pressable>}
        </View>
        {error !== '' && <Text accessibilityRole="alert" style={styles.error}>{error}</Text>}
        {!online ? (
          <ScrollView contentContainerStyle={styles.form} keyboardShouldPersistTaps="handled">
            <Text style={styles.title}>Welcome back</Text>
            <Text style={styles.caption}>Sign in to your local mig33 server.</Text>
            <Text style={styles.label}>Username</Text>
            <TextInput accessibilityLabel="Username" style={styles.input} value={username} onChangeText={setUsername}
              autoCapitalize="none" autoCorrect={false} editable={!busy} textContentType="username" />
            <Text style={styles.label}>Password</Text>
            <TextInput accessibilityLabel="Password" style={styles.input} value={password} onChangeText={setPassword}
              secureTextEntry autoCapitalize="none" editable={!busy} textContentType="password" onSubmitEditing={connect} />
            <Pressable accessibilityRole="button" onPress={() => setSettings(!settings)} disabled={busy}>
              <Text style={styles.settingsLink}>{settings ? 'Hide connection settings' : 'Connection settings'}</Text>
            </Pressable>
            {settings && <View style={styles.settings}>
              <Text style={styles.label}>Connection</Text>
              <View style={styles.segment}>
                {(['http', 'tcp'] as const).map((value) => <Pressable key={value} accessibilityRole="button"
                  accessibilityState={{ selected: mode === value }} onPress={() => setMode(value)} disabled={busy}
                  style={[styles.segmentItem, mode === value && styles.selected]}>
                  <Text style={mode === value ? styles.selectedText : styles.segmentText}>{value.toUpperCase()}</Text>
                </Pressable>)}
              </View>
              {mode === 'http' ? <TextInput accessibilityLabel="HTTP server" style={styles.input}
                value={httpURL} onChangeText={setHTTPURL} autoCapitalize="none" autoCorrect={false} editable={!busy} /> :
                <><TextInput accessibilityLabel="TCP host" style={styles.input} value={host} onChangeText={setHost}
                  autoCapitalize="none" autoCorrect={false} editable={!busy} />
                  <TextInput accessibilityLabel="TCP port" style={styles.input} value={port} onChangeText={setPort}
                    keyboardType="number-pad" editable={!busy} /></>}
            </View>}
            <Pressable accessibilityRole="button" style={[styles.button, busy && styles.disabled]} onPress={connect} disabled={busy}>
              {busy ? <ActivityIndicator color="white" /> : <Text style={styles.buttonText}>Sign in</Text>}
            </Pressable>
            <Text style={styles.note}>Local demo accounts: alice / alice-demo and bob / bob-demo.</Text>
            {state === 'closed' && <Text style={styles.note}>Your connection is closed. Sign in to reconnect.</Text>}
          </ScrollView>
        ) : (
          <View style={styles.chat}>
            <View style={styles.contactHeader}>
              <Text style={styles.label}>Friends</Text>
              <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.friends}>
                {contacts.filter((contact) => contact.username).map((contact) => (
                  <Pressable accessibilityRole="button" key={contact.id} onPress={() => setRecipient(contact.username)}
                    style={[styles.friend, recipient === contact.username && styles.friendSelected]}>
                    <Text style={styles.friendText}>{contact.display || contact.username}</Text>
                    <Text style={styles.presence}>{contact.presence === 99 ? 'Offline' : 'Online'}</Text>
                  </Pressable>
                ))}
              </ScrollView>
              <TextInput accessibilityLabel="Message recipient" style={styles.recipient} value={recipient}
                onChangeText={setRecipient} autoCapitalize="none" autoCorrect={false} placeholder="Friend's username" />
            </View>
            <FlatList data={rows.filter((row) => row.peer === recipient.trim().toLowerCase())}
              keyExtractor={(item) => item.id} contentContainerStyle={styles.messages}
              ListEmptyComponent={<Text style={styles.empty}>Say hello to {recipient || 'a friend'}.</Text>}
              renderItem={({ item }) => <View style={[styles.bubble, item.own && styles.ownBubble, item.action && styles.actionBubble]}>
                {!item.action && <Text style={styles.sender}>{item.own ? 'You' : item.sender}</Text>}
                <Text style={styles.message}>{item.body}</Text>
              </View>} />
            <View style={styles.composer}>
              <TextInput accessibilityLabel="Message" style={styles.draft} value={draft} onChangeText={setDraft}
                placeholder="Write a message…" multiline maxLength={2000} editable={!sending} />
              <Pressable accessibilityRole="button" accessibilityLabel="Send message" onPress={send}
                disabled={sending || !draft.trim() || !recipient.trim()} style={styles.send}>
                {sending ? <ActivityIndicator color="white" /> : <Text style={styles.buttonText}>Send</Text>}
              </Pressable>
            </View>
          </View>
        )}
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#f7f9fc' },
  header: { padding: 22, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', backgroundColor: '#fff' },
  logo: { color: '#175ccb', fontWeight: '800', fontSize: 30, letterSpacing: -1 },
  caption: { color: '#66758b', fontSize: 14, marginTop: 5 },
  link: { color: '#175ccb', fontWeight: '600', padding: 8 },
  error: { backgroundColor: '#fff0ee', color: '#9e3025', padding: 14, margin: 14, borderRadius: 10 },
  form: { padding: 24, gap: 8, width: '100%', maxWidth: 480, alignSelf: 'center' },
  title: { fontSize: 26, fontWeight: '700', color: '#172942', marginTop: 20 },
  label: { color: '#344861', fontSize: 14, fontWeight: '600', marginTop: 14, marginBottom: 4 },
  input: { backgroundColor: '#fff', borderWidth: 1, borderColor: '#dbe2ed', borderRadius: 12, padding: 14, fontSize: 16, color: '#172942', marginBottom: 6 },
  button: { backgroundColor: '#175ccb', borderRadius: 12, padding: 17, alignItems: 'center', marginTop: 18 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '700' },
  disabled: { opacity: 0.6 },
  settingsLink: { color: '#175ccb', paddingVertical: 10 },
  settings: { paddingBottom: 10 },
  segment: { flexDirection: 'row', gap: 6, marginBottom: 10 },
  segmentItem: { paddingVertical: 10, paddingHorizontal: 24, backgroundColor: '#e5ebf4', borderRadius: 9 },
  segmentText: { color: '#344861', fontWeight: '600' },
  selected: { backgroundColor: '#175ccb' },
  selectedText: { color: '#fff', fontWeight: '600' },
  note: { color: '#66758b', fontSize: 13, lineHeight: 19, marginTop: 14 },
  chat: { flex: 1 },
  contactHeader: { paddingHorizontal: 18, paddingBottom: 12, backgroundColor: '#fff' },
  friends: { gap: 8, paddingVertical: 7 },
  friend: { paddingVertical: 9, paddingHorizontal: 14, borderRadius: 12, backgroundColor: '#eef2f8' },
  friendSelected: { backgroundColor: '#dce9ff' },
  friendText: { color: '#263e60', fontWeight: '600' },
  presence: { fontSize: 11, marginTop: 3, color: '#66758b' },
  recipient: { borderBottomWidth: 1, borderColor: '#dbe2ed', paddingVertical: 9, color: '#172942', fontSize: 16 },
  messages: { padding: 18, gap: 10, flexGrow: 1 },
  empty: { color: '#75859a', textAlign: 'center', marginTop: 50 },
  bubble: { maxWidth: '86%', alignSelf: 'flex-start', padding: 13, borderRadius: 16, backgroundColor: '#fff' },
  ownBubble: { alignSelf: 'flex-end', backgroundColor: '#dce9ff' },
  actionBubble: { alignSelf: 'center', backgroundColor: '#edf1f7' },
  sender: { fontSize: 11, fontWeight: '700', color: '#536b8c', marginBottom: 4 },
  message: { color: '#172942', fontSize: 16, lineHeight: 23 },
  composer: { flexDirection: 'row', gap: 10, alignItems: 'flex-end', padding: 14, backgroundColor: '#fff' },
  draft: { flex: 1, minHeight: 44, maxHeight: 130, backgroundColor: '#f0f3f8', borderRadius: 14, padding: 12, color: '#172942', fontSize: 16 },
  send: { backgroundColor: '#175ccb', padding: 13, borderRadius: 12, minWidth: 68, alignItems: 'center' },
});
