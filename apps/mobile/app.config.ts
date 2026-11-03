import type { ExpoConfig } from 'expo/config';

const config: ExpoConfig = {
  name: 'mig33',
  slug: 'mig33-mobile',
  version: '0.1.0',
  scheme: 'mig33',
  orientation: 'portrait',
  userInterfaceStyle: 'automatic',
  platforms: ['ios', 'android'],
  ios: {
    bundleIdentifier: 'com.hailam.mig33',
    supportsTablet: true,
    infoPlist: {
      NSLocalNetworkUsageDescription: 'Connect to your mig33 server on the local network.',
      NSAppTransportSecurity: {
        NSAllowsArbitraryLoads: false,
        NSAllowsLocalNetworking: false,
      },
    },
  },
  android: {
    package: 'com.hailam.mig33',
  },
  plugins: [
    ['expo-splash-screen', {
      backgroundColor: '#f5f9fa', image: './assets/mig33-logo.png', imageWidth: 176,
      dark: { backgroundColor: '#0b151c', image: './assets/mig33-logo.png' },
    }],
    'expo-dev-client',
    'expo-secure-store',
    ['expo-image-picker', {
      photosPermission: 'Choose a photo to share in mig33.',
      cameraPermission: 'Take a photo to share in mig33.',
      microphonePermission: false,
    }],
    ['expo-build-properties', { android: { usesCleartextTraffic: false } }],
    './plugins/withSecureNetworking.cjs',
    './plugins/withLaunchScreen.cjs',
  ],
};

export default config;
