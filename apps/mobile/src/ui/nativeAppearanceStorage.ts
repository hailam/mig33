import * as SecureStore from 'expo-secure-store';
import type { AppearanceMode } from './appearance';
import { parseAppearanceMode } from './appearanceTheme';

// Appearance is independent of credentials: Forget Login must not reset it.
const key = 'mig33.ui.appearance';
const options = { keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY };
let pending: Promise<void> = Promise.resolve();

export async function loadAppearance(): Promise<AppearanceMode> {
  await pending;
  const value = await SecureStore.getItemAsync(key, options);
  return parseAppearanceMode(value);
}

export function saveAppearance(mode: AppearanceMode): Promise<void> {
  // Rapid switches persist in selection order, including after a failed write.
  const operation = pending.then(() => SecureStore.setItemAsync(key, mode, options));
  pending = operation.catch(() => {});
  return operation;
}

export async function loadHomeTabOrder(): Promise<readonly string[]> {
  const saved = await SecureStore.getItemAsync('mig33.ui.home-tabs', options);
  if (!saved) return ['friends', 'rooms'];
  try {
    const order: unknown = JSON.parse(saved);
    if (Array.isArray(order) && order.length === 3 && new Set(order).size === 3
      && order.every((id) => ['friends', 'rooms', 'migworld'].includes(id))) return order;
  } catch { /* Ignore an invalid preference. */ }
  return ['friends', 'rooms'];
}

export function saveHomeTabOrder(order: readonly string[]): Promise<void> {
  const operation = pending.then(() => SecureStore.setItemAsync('mig33.ui.home-tabs', JSON.stringify(order), options));
  pending = operation.catch(() => {});
  return operation;
}
