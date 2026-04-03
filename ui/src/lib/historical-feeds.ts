import { useSyncExternalStore } from "react";

const STORAGE_KEY = "update-ipsets:show-historical-feeds";
const CHANGE_EVENT = "update-ipsets:show-historical-feeds-change";

function readSnapshot(): boolean {
  if (typeof window === "undefined") return false;
  return window.localStorage.getItem(STORAGE_KEY) === "true";
}

function subscribe(callback: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  const onStorage = (event: Event) => {
    if (
      event instanceof StorageEvent &&
      event.key &&
      event.key !== STORAGE_KEY
    ) {
      return;
    }
    callback();
  };
  window.addEventListener("storage", onStorage);
  window.addEventListener(CHANGE_EVENT, onStorage);
  return () => {
    window.removeEventListener("storage", onStorage);
    window.removeEventListener(CHANGE_EVENT, onStorage);
  };
}

export function useHistoricalFeedsToggle(): [boolean, (next: boolean) => void] {
  const value = useSyncExternalStore(subscribe, readSnapshot, () => false);
  return [value, setHistoricalFeedsVisible];
}

export function setHistoricalFeedsVisible(next: boolean): void {
  if (typeof window === "undefined") return;
  if (next) {
    window.localStorage.setItem(STORAGE_KEY, "true");
  } else {
    window.localStorage.removeItem(STORAGE_KEY);
  }
  window.dispatchEvent(new Event(CHANGE_EVENT));
}
