import React from 'react';
import { initialWindowMetrics, SafeAreaProvider } from 'react-native-safe-area-context';
import Application from './src/ui/Application';

export default function App() {
  return <SafeAreaProvider initialMetrics={initialWindowMetrics}><Application /></SafeAreaProvider>;
}
