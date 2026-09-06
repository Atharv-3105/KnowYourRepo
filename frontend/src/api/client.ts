const BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

// Every KnowYourRepo API error response is a flat { "error": "..." } object
// with a varying HTTP status (400/404/500/503) - see the backend's confirmed
// error convention (Docs/phase5_frontend). ApiError normalizes that plus the
// X-Request-ID response header (always present, set by RequestIDMiddleware)
// so callers can surface it for support/debugging without re-parsing responses.
export class ApiError extends Error {
  status: number;
  requestId: string | null;

  constructor(message: string, status: number, requestId: string | null) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.requestId = requestId;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  // Exposed via Access-Control-Expose-Headers by the backend's CORS middleware -
  // without that, this would read null on every cross-origin response.
  const requestId = res.headers.get("X-Request-ID");

  if (!res.ok) {
    let message = res.statusText;

    try {
      const body = (await res.json()) as { error?: string };
      if (typeof body.error === "string") {
        message = body.error;
      }
    } catch {
      // Non-JSON or empty error body - fall back to the status text.
    }

    throw new ApiError(message, res.status, requestId);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "POST",
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
};
