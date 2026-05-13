export const adminTokenStorageKey = 'labkit_admin_token';

export function readAdminToken(): string {
  return (
    window.sessionStorage.getItem(adminTokenStorageKey) ??
    window.localStorage.getItem(adminTokenStorageKey) ??
    ''
  ).trim();
}

/** Store token in sessionStorage (clears on tab close). Default for login. */
export function sessionToken(token: string): void {
  const value = token.trim();
  if (!value) return;
  window.sessionStorage.setItem(adminTokenStorageKey, value);
  window.localStorage.removeItem(adminTokenStorageKey);
}

/** Store token in localStorage (persists across browser restarts). */
export function rememberToken(token: string): void {
  const value = token.trim();
  if (!value) return;
  window.localStorage.setItem(adminTokenStorageKey, value);
  window.sessionStorage.removeItem(adminTokenStorageKey);
}

/** Remove token from both storages. */
export function clearAdminToken(): void {
  window.sessionStorage.removeItem(adminTokenStorageKey);
  window.localStorage.removeItem(adminTokenStorageKey);
}

export function authorizedAdminHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init);
  const token = readAdminToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  return headers;
}

export async function readAPIError(response: Response, fallback: string): Promise<string> {
  try {
    const payload = (await response.json()) as {
      error?: { message?: string };
      message?: string;
    };
    if (typeof payload?.error?.message === 'string' && payload.error.message.trim() !== '') {
      return payload.error.message.trim();
    }
    if (typeof payload?.message === 'string' && payload.message.trim() !== '') {
      return payload.message.trim();
    }
  } catch {
    try {
      const text = await response.text();
      if (text.trim() !== '') return text.trim();
    } catch {
      return fallback;
    }
  }
  return fallback;
}

export function fileNameFromDisposition(disposition: string | null, fallback: string): string {
  if (!disposition) return fallback;
  const match = disposition.match(/filename="?([^"]+)"?/i);
  return match?.[1]?.trim() || fallback;
}
