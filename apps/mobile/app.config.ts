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
      // The preserved HTTP gateway is cleartext HTTP. Native TCP uses its
      // original raw socket protocol; neither transport adds a new API.
      NSAppTransportSecurity: {
        NSAllowsArbitraryLoads: true,
        NSAllowsLocalNetworking: true,
      },
    },
  },
  android: {
    package: 'com.hailam.mig33',
  },
  plugins: [
    'expo-dev-client',
    ['expo-build-properties', { android: { usesCleartextTraffic: true } }],
  ],
};

export default config;
