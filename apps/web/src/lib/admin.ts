export const adminTokenStorageKey = 'labkit_admin_token';

export function readAdminToken() {
  return (window.sessionStorage.getItem(adminTokenStorageKey) ?? '').trim();
}

export function writeAdminToken(token: string) {
  const value = token.trim();
  if (!value) {
    return;
  }
  window.sessionStorage.setItem(adminTokenStorageKey, value);
}

export function authorizedAdminHeaders(init?: HeadersInit) {
  const headers = new Headers(init);
  const token = readAdminToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  return headers;
}

export async function readAPIError(response: Response, fallback: string) {
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
      if (text.trim() !== '') {
        return text.trim();
      }
    } catch {
      return fallback;
    }
  }
  return fallback;
}

export function fileNameFromDisposition(disposition: string | null, fallback: string) {
  if (!disposition) {
    return fallback;
  }
  const match = disposition.match(/filename="?([^"]+)"?/i);
  return match?.[1]?.trim() || fallback;
}
