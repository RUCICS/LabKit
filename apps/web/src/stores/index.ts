import { createPinia } from 'pinia';

export { useAppStore } from './app';

export function createAppPinia() {
  return createPinia();
}
