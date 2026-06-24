import { computed, ref } from 'vue';

export type SessionUser = {
  user_id?: number;
  student_id: string;
  nickname?: string;
};

const user = ref<SessionUser | null>(null);
const loaded = ref(false);
let inflight: Promise<void> | null = null;

/** The signed-in user, or null when signed out. */
export const sessionUser = computed(() => user.value);
/** True once the session state has been resolved at least once. */
export const sessionLoaded = computed(() => loaded.value);
/** Whether a browser session is currently active. */
export const isAuthenticated = computed(() => user.value !== null);

/**
 * Resolve the current session by probing the read-only /api/profile endpoint.
 * Concurrent calls share one request; safe to call on every app mount.
 */
export function refreshSession(): Promise<void> {
  if (inflight) {
    return inflight;
  }
  inflight = (async () => {
    try {
      if (typeof fetch !== 'function') {
        user.value = null;
        return;
      }
      const response = await fetch('/api/profile', { credentials: 'include' });
      user.value = response.ok ? ((await response.json()) as SessionUser) : null;
    } catch {
      user.value = null;
    } finally {
      loaded.value = true;
      inflight = null;
    }
  })();
  return inflight;
}

/**
 * Begin sign-in: a full-page redirect to the school identity provider. After
 * authentication the user is returned to `next` (defaults to the current page).
 */
export function login(next?: string): void {
  if (typeof window === 'undefined') {
    return;
  }
  window.location.href = `/api/auth/login?next=${encodeURIComponent(safeNext(next))}`;
}

function safeNext(next?: string): string {
  const candidate = (next ?? '').trim();
  if (isSameOriginPath(candidate)) {
    return candidate;
  }
  if (typeof window !== 'undefined') {
    const current = window.location.pathname + window.location.search;
    if (isSameOriginPath(current)) {
      return current;
    }
  }
  return '/';
}

function isSameOriginPath(value: string): boolean {
  return value.startsWith('/') && !value.startsWith('//');
}
