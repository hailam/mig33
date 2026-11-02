import { useEffect, useMemo, useSyncExternalStore } from 'react';
import { OriginalSearch, type OriginalSearchKind } from '../../mig33/original-search';
import type { OriginalRequest } from '../../mig33/legacy-pages';

export function useOriginalSearch({ requestOriginal, siteURL, kind, online }: {
  requestOriginal: OriginalRequest; siteURL: string; kind: OriginalSearchKind; online: boolean;
}) {
  const search = useMemo(() => new OriginalSearch(requestOriginal, siteURL, kind), [requestOriginal, siteURL, kind]);
  const state = useSyncExternalStore(search.subscribe, search.getSnapshot, search.getSnapshot);
  useEffect(() => { if (!online) search.clear(); }, [search, online]);
  useEffect(() => () => search.close(), [search]);
  return { ...state, search: search.search, clear: search.clear };
}
