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
