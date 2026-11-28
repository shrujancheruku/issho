import AsyncStorage from '@react-native-async-storage/async-storage';
import { getRandomBytes } from 'expo-crypto';
import * as SecureStore from 'expo-secure-store';

import { createEncryptedStorage } from './encrypted-storage';

/**
 * Where a phone keeps the session so it survives a restart: encrypted in
 * AsyncStorage, with the key in the OS's secure store (encrypted-storage.ts).
 */
export const sessionStorage = createEncryptedStorage({
  keys: {
    getItem: (k) => SecureStore.getItemAsync(k),
    setItem: (k, v) => SecureStore.setItemAsync(k, v),
    removeItem: (k) => SecureStore.deleteItemAsync(k),
  },
  bulk: AsyncStorage,
  randomBytes: getRandomBytes,
});
