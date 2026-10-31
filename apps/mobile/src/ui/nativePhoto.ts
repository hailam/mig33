import * as ImagePicker from 'expo-image-picker';
import { ImageManipulator, SaveFormat } from 'expo-image-manipulator';

export interface SelectedPhoto { bytes: Uint8Array; uri: string; }

/** Normalize modern camera formats to the original PNG/JPEG/2048px contract.
 * Keep headroom below the 1MiB frame limit for the original packet fields.
 */
export async function choosePhoto(source: 'camera' | 'library'): Promise<SelectedPhoto | undefined> {
  if (source === 'camera') {
    const permission = await ImagePicker.requestCameraPermissionsAsync();
    if (!permission.granted) throw new Error('Camera access is needed to take a photo. You can enable it in Settings.');
  }
  const options: ImagePicker.ImagePickerOptions = { mediaTypes: ['images'], allowsEditing: false,
    allowsMultipleSelection: false, quality: 1, exif: false };
  const result = source === 'camera' ? await ImagePicker.launchCameraAsync(options) : await ImagePicker.launchImageLibraryAsync(options);
  if (result.canceled || !result.assets[0]) return undefined;
  const asset = result.assets[0];
  const context = ImageManipulator.manipulate(asset.uri);
  try {
    if (Math.max(asset.width, asset.height) > 1600) context.resize(asset.width >= asset.height ? { width: 1600 } : { height: 1600 });
    const image = await context.renderAsync();
    try {
      for (const compress of [0.8, 0.6, 0.4]) {
        const saved = await image.saveAsync({ format: SaveFormat.JPEG, compress, base64: true });
        if (!saved.base64) throw new Error('The photo could not be prepared. Please choose another photo.');
        const binary = atob(saved.base64);
        if (binary.length <= 900 * 1024) return { bytes: Uint8Array.from(binary, (character) => character.charCodeAt(0)), uri: saved.uri };
      }
      throw new Error('This photo is too large to share. Choose a smaller photo.');
    } finally { image.release(); }
  } finally { context.release(); }
}
