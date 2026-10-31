import * as SecureStore from 'expo-secure-store';
import { LoginStorage } from './loginStorage';

const key = 'mig33.v4.remembered-login';
const options = { keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY };
export const loginStorage = new LoginStorage({
  read: () => SecureStore.getItemAsync(key, options),
  write: (value) => SecureStore.setItemAsync(key, value, options),
  remove: () => SecureStore.deleteItemAsync(key, options),
});
