import React, { useMemo, useRef, useState } from 'react';
import { LayoutAnimation, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useAppearance } from '../appearance';
import { ModernLogo } from '../ModernLogo';
import { V4Button, V4Checkbox, V4ChoiceGroup, V4Field, V4Icon, V4Link, V4Logo, type V4FieldRef } from '../v4/primitives';

export interface LoginForm {
  username: string;
  password: string;
  remember: boolean;
  auto: boolean;
  invisible: boolean;
}

export function WelcomeScreen({ onLogin }: { onLogin: () => void }) {
  const { modern, v3 } = useAppearance();
  const styles = useStyles();
  // v3's welcome43 follows a connection test. Keep its available choices here
  // without claiming that a successful test occurred merely by changing theme.
  if (v3) return <ScrollView contentContainerStyle={styles.form}>
    <Text style={styles.text}>Please select:</Text>
    <V4Button label="Login" onPress={onLogin} testID="welcome.login" />
    <Text style={styles.text}>Or</Text>
    <V4Button label="Registration" onPress={() => {}} disabled />
  </ScrollView>;
  if (modern) return <ScrollView contentContainerStyle={[styles.form, styles.welcome]}>
    <View style={styles.welcomeIntro}>
      <View style={styles.welcomeBrand}><ModernLogo width={176} /></View>
      <Text accessibilityRole="header" style={styles.welcomeTitle}>Find your people.</Text>
      <Text style={styles.subtitle}>Friends and conversations, together.</Text>
    </View>
    <V4Button label="Login" onPress={onLogin} testID="welcome.login" style={styles.primaryButton} />
  </ScrollView>;
  return <ScrollView contentContainerStyle={styles.form}>
    <View style={styles.logo}><V4Logo /></View>
    <Text style={styles.text}>Join mig33 and meet new friends, chat and play games and much more!</Text>
    <V4Button label="Login" onPress={onLogin} testID="welcome.login" />
    <V4Button label="Registration" onPress={() => {}} disabled />
    <V4Button label="Change Language" onPress={() => {}} disabled />
  </ScrollView>;
}

/** Form15: original en-US labels and row order; unsupported flows stay disabled. */
export function LoginScreen({ form, onChange, busy, onLogin, onConnection, onAppearance, showOffline, onShowOfflineChange }: {
  form: LoginForm; onChange: (change: Partial<LoginForm>) => void;
  busy: boolean; onLogin: () => void; onConnection: () => void; onAppearance: () => void;
  showOffline: boolean; onShowOfflineChange: (show: boolean) => void;
}) {
  const { modern, v3, reduceMotion } = useAppearance();
  const styles = useStyles();
  const passwordField = useRef<V4FieldRef>(null);
  const [optionsExpanded, setOptionsExpanded] = useState(false);
  const optionsSummary = `Auto Login ${form.auto ? 'on' : 'off'} · Invisible sign-in ${form.invisible ? 'on' : 'off'}`;
  function toggleOptions() {
    if (busy) return;
    if (!reduceMotion) LayoutAnimation.configureNext({ duration: 180, update: { type: LayoutAnimation.Types.easeInEaseOut } });
    setOptionsExpanded((expanded) => !expanded);
  }
  const credentials = <>
    <V4Field testID="login.username" label="Username" value={form.username}
      onChangeText={(username) => onChange({ username })} autoCapitalize="none" autoCorrect={false}
      editable={!busy} textContentType="username" autoComplete="username" maxLength={320}
      {...(modern && { returnKeyType: 'next' as const, submitBehavior: 'submit' as const,
        onSubmitEditing: () => { if (!busy) passwordField.current?.focus(); } })} />
    <V4Field ref={passwordField} testID="login.password" label="Password" value={form.password}
      onChangeText={(password) => onChange({ password })} secureTextEntry autoCapitalize="none"
      autoCorrect={false} editable={!busy} textContentType="password" autoComplete="current-password"
      onSubmitEditing={onLogin} returnKeyType="go" maxLength={320} />
  </>;
  // Original v3 t constructor case15, preserved in screens-form-rows.json.
  // External IM, registration and recovery remain explicitly unavailable.
  if (v3) return <ScrollView testID="login.screen" contentContainerStyle={styles.form} keyboardShouldPersistTaps="handled">
    {credentials}
    <V4Checkbox testID="login.remember" label="Save Password" checked={form.remember}
      onChange={(remember) => onChange({ remember, ...(!remember ? { auto: false } : {}) })} disabled={busy} />
    <V4Checkbox testID="login.invisible" label="Sign in as Invisible" checked={form.invisible}
      onChange={(invisible) => onChange({ invisible })} disabled={busy} />
    <View style={styles.v3Spacer} />
    <V4Button testID="login.submit" label="Login" onPress={onLogin} busy={busy} />
    <View style={styles.v3Spacer} />
    <V4Button label="Register" onPress={() => {}} disabled />
    <View style={styles.v3Spacer} />
    <V4Button label="Forgot Details" onPress={() => {}} disabled />
    <View style={styles.v3Spacer} />
    <V4Button testID="login.connection" label="Connection Type" onPress={onConnection} disabled={busy} />
    <View style={styles.v3Spacer} />
    <Text style={styles.text}>Connect to:</Text>
    {['MSN', 'Yahoo!', 'Google Talk', 'AIM'].map((label) => <V4Checkbox key={label} label={label} checked={false} disabled />)}
    <View style={styles.v3Spacer} /><View style={styles.v3Spacer} />
    <V4Checkbox testID="login.showOffline" label="Show offline contacts" checked={showOffline}
      onChange={onShowOfflineChange} disabled={busy} />
    <V4Link testID="appearance.open" label="Appearance" onPress={onAppearance} disabled={busy} />
  </ScrollView>;
  const rememberPreference = <V4Checkbox testID="login.remember" label="Remember Login" checked={form.remember}
    onChange={(remember) => onChange({ remember, ...(!remember ? { auto: false } : {}) })} disabled={busy} />;
  const optionalPreferences = <>
    <V4Checkbox testID="login.auto" label="Auto Login" checked={form.auto}
      onChange={(auto) => onChange({ auto, ...(auto ? { remember: true } : {}) })} disabled={busy} />
    <V4Checkbox testID="login.invisible" label="Login As Invisible" checked={form.invisible}
      onChange={(invisible) => onChange({ invisible })} disabled={busy} />
  </>;
  if (modern) return <ScrollView testID="login.screen" contentContainerStyle={[styles.form, styles.login]}
    keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    <View style={styles.intro}>
      <View style={styles.brand}><ModernLogo /></View>
      <Text accessibilityRole="header" style={styles.title}>Sign in</Text>
      <Text style={styles.subtitle}>Friends and conversations, together.</Text>
    </View>
    <View style={styles.credentials}>{credentials}</View>
    <View style={styles.preferences}>
      <Text style={styles.sectionLabel}>Sign-in preferences</Text>
      {rememberPreference}
      <Pressable testID="login.options" accessibilityRole="button" accessibilityLabel="More sign-in options"
        accessibilityState={{ expanded: optionsExpanded, disabled: busy }}
        accessibilityValue={{ text: `${optionsExpanded ? '' : 'Collapsed. '}${optionsSummary}` }}
        disabled={busy} onPress={toggleOptions}
        style={({ pressed }) => [styles.optionsDisclosure, pressed && styles.optionsPressed]}>
        <View style={styles.optionsIdentity}>
          <Text style={styles.optionsTitle}>More sign-in options</Text>
          {!optionsExpanded && <Text testID="login.options.summary" style={styles.optionsSummary}>{optionsSummary}</Text>}
        </View>
        <V4Icon name={optionsExpanded ? 'arrowDown' : 'arrowRight'} size={13} />
      </Pressable>
      {optionsExpanded && <View style={styles.optionalPreferences}>{optionalPreferences}</View>}
    </View>
    <View style={styles.actions}>
      <V4Button testID="login.submit" label="Login" onPress={onLogin} busy={busy} style={styles.primaryButton} />
      <View style={styles.utilities}>
        <V4Link testID="login.connection" label="Connection Settings" onPress={onConnection} disabled={busy} />
        <V4Link testID="appearance.open" label="Appearance" onPress={onAppearance} disabled={busy} />
      </View>
    </View>
  </ScrollView>;
  return <ScrollView testID="login.screen" contentContainerStyle={styles.form} keyboardShouldPersistTaps="handled">
    {credentials}
    {rememberPreference}{optionalPreferences}
    <V4Checkbox label="Show Chatroom Avatars" checked disabled />
    <V4Button testID="login.submit" label="Login" onPress={onLogin} busy={busy} />
    <V4Link label="Forgot password?" disabled />
    <V4Link label="Register as a new user." disabled />
    <V4Link testID="login.connection" label="Connection Settings" onPress={onConnection} disabled={busy} />
    <V4Link label="Change Language" disabled />
    <V4Link testID="appearance.open" label="Appearance" onPress={onAppearance} disabled={busy} />
  </ScrollView>;
}

export function ConnectionChooser({ onChoose, onManual, busy }: {
  onChoose: (mode: 'http' | 'tcp') => void; onManual: () => void; busy: boolean;
}) {
  const { modern, v3 } = useAppearance();
  const styles = useStyles();
  return <ScrollView contentContainerStyle={styles.form}>
    {v3 && <View pointerEvents="none" style={styles.watermark}><V4Logo width={118} /></View>}
    {modern && <Text accessibilityRole="header" style={styles.title}>Your connection</Text>}
    <Text style={styles.text}>Please select your connection type below. If you are unsure, try TCP first.</Text>
    <V4Button label="TCP" variant="secondary" onPress={() => onChoose('tcp')} disabled={busy} style={modern ? styles.connectionAction : undefined} />
    <V4Button label="HTTP" variant="secondary" onPress={() => onChoose('http')} disabled={busy} style={modern ? styles.connectionAction : undefined} />
    <V4Button testID="connection.manual" label="Manual" variant="secondary" onPress={onManual} disabled={busy} style={modern ? styles.connectionAction : undefined} />
  </ScrollView>;
}

export interface ConnectionForm { host: string; port: string; mode: 'http' | 'tcp'; }
/** Original manual form13. Local server addresses replace the historical host. */
export function ManualConnectionScreen({ form, onChange, onConnect, busy }: {
  form: ConnectionForm; onChange: (change: Partial<ConnectionForm>) => void;
  onConnect: () => void; busy: boolean;
}) {
  const { modern } = useAppearance();
  const styles = useStyles();
  const portField = useRef<V4FieldRef>(null);
  return <ScrollView contentContainerStyle={styles.form} keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    {modern && <View style={styles.intro}>
      <Text accessibilityRole="header" style={styles.title}>Connection settings</Text>
      <Text style={styles.subtitle}>Choose your server and connection type.</Text>
    </View>}
    <V4Field testID="connection.host" label="Server" value={form.host}
      onChangeText={(host) => onChange({ host })} autoCapitalize="none" autoCorrect={false} editable={!busy}
      {...(modern && { returnKeyType: 'next' as const, submitBehavior: 'submit' as const,
        onSubmitEditing: () => { if (!busy) portField.current?.focus(); } })} />
    <V4Field ref={portField} testID="connection.port" label="Port" value={form.port}
      onChangeText={(port) => onChange({ port })} keyboardType="number-pad" editable={!busy} />
    {/* Explicit transport selection extends v3 form13 for arbitrary local endpoints. */}
    {modern ? <V4ChoiceGroup label="Connection type" variant="segmented" value={form.mode} disabled={busy}
      options={[{ value: 'tcp', label: 'TCP', testID: 'connection.tcp' }, { value: 'http', label: 'HTTP', testID: 'connection.http' }]}
      onChange={(mode) => { if (!busy && (mode === 'http' || mode === 'tcp')) onChange({ mode }); }} /> : <>
    <V4Checkbox testID="connection.tcp" label="TCP" checked={form.mode === 'tcp'}
      onChange={() => onChange({ mode: 'tcp' })} disabled={busy} />
    <V4Checkbox testID="connection.http" label="HTTP" checked={form.mode === 'http'}
      onChange={() => onChange({ mode: 'http' })} disabled={busy} />
    </>}
    <V4Button testID="connection.save" label="Connect" onPress={onConnect} busy={busy} style={modern ? styles.connectionAction : undefined} />
  </ScrollView>;
}

function useStyles() {
  const { theme, metrics, modern, v3 } = useAppearance();
  return useMemo(() => StyleSheet.create({
    form: { paddingHorizontal: metrics.pageInset, paddingTop: modern ? 22 : 4,
      paddingBottom: modern ? 28 : 12, alignItems: modern ? 'stretch' : 'flex-start',
      ...(modern && { width: '100%', maxWidth: 520, alignSelf: 'center', flexGrow: 1 }),
      ...(v3 && { flexGrow: 1 }) },
    text: { color: theme.text, fontSize: metrics.font, lineHeight: metrics.lineHeight, marginBottom: modern ? 20 : 6 },
    logo: { alignSelf: 'center', paddingVertical: 6 },
    v3Spacer: { height: 3 },
    watermark: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center' },
    login: { paddingTop: 16 },
    intro: { marginBottom: 25 },
    brand: { alignSelf: 'flex-start', marginLeft: -17, marginBottom: 17 },
    title: { color: theme.text, fontSize: 36, lineHeight: 43, fontWeight: '700', letterSpacing: -1.1, marginBottom: 8 },
    subtitle: { color: theme.secondaryText, fontSize: 16, lineHeight: 25 },
    credentials: { marginBottom: 2 },
    preferences: { paddingVertical: 12, marginBottom: 13,
      borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: theme.groupBorder },
    sectionLabel: { color: theme.secondaryText, fontSize: 13, lineHeight: 19, fontWeight: '500', marginBottom: 7 },
    optionsDisclosure: { minHeight: 44, flexDirection: 'row', alignItems: 'center', gap: 12,
      paddingVertical: 10, borderRadius: 12 },
    optionsPressed: { backgroundColor: theme.selected },
    optionsIdentity: { flex: 1, minWidth: 0 },
    optionsTitle: { color: theme.text, fontSize: metrics.font, lineHeight: metrics.lineHeight },
    optionsSummary: { color: theme.secondaryText, fontSize: 13, lineHeight: 19, marginTop: 3 },
    optionalPreferences: { paddingTop: 4 },
    actions: { marginTop: 'auto', paddingTop: 6 },
    utilities: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'space-between',
      columnGap: 20, rowGap: 2, paddingTop: 9 },
    primaryButton: { alignSelf: 'stretch' },
    connectionAction: { alignSelf: 'stretch', marginTop: 12 },
    welcome: { paddingBottom: 32 }, welcomeIntro: { flex: 1, justifyContent: 'center', paddingVertical: 36 },
    welcomeBrand: { alignSelf: 'flex-start', marginLeft: -21, marginBottom: 32 },
    welcomeTitle: { color: theme.text, fontSize: 44, lineHeight: 51, fontWeight: '700', letterSpacing: -1.4, marginBottom: 12 },
  }), [theme, metrics, modern, v3]);
}
