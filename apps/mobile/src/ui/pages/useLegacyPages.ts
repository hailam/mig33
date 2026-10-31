import { useEffect, useMemo, useRef, useSyncExternalStore } from 'react';
import { LegacyPages, type NativePageAction, type OriginalRequest } from '../../mig33/legacy-pages';

export function useLegacyPages({ requestOriginal, siteURL, online, onNativeAction }: {
  requestOriginal: OriginalRequest;
  siteURL: string;
  online: boolean;
  onNativeAction: (action: NativePageAction) => void | Promise<void>;
}) {
  const action = useRef(onNativeAction);
  action.current = onNativeAction;
  const browser = useMemo(() => new LegacyPages(requestOriginal, siteURL, (value) => action.current(value)), [requestOriginal, siteURL]);
  const state = useSyncExternalStore(browser.subscribe, browser.getSnapshot, browser.getSnapshot);
  useEffect(() => { if (!online) browser.close(); }, [browser, online]);
  useEffect(() => () => browser.close(), [browser]);
  return { ...state, open: browser.open, follow: browser.follow, submit: browser.submit, back: browser.back,
    close: browser.close, refresh: browser.refresh, loadImage: browser.loadImage };
}

export type LegacyBrowser = ReturnType<typeof useLegacyPages>;
