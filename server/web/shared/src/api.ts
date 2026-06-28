export type ApiStatus = 'ok' | 'error';

export interface ApiErrorPayload {
  status: 'error';
  message: string;
  code?: string;
}

export type JsonRecord = Record<string, unknown>;

export class ApiError extends Error {
  readonly statusCode: number;
  readonly code?: string;

  constructor(message: string, statusCode: number, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.statusCode = statusCode;
    this.code = code;
  }
}

export interface ApiClientOptions {
  onUnauthorized?: () => void;
}

export class ApiClient {
  constructor(private readonly options: ApiClientOptions = {}) {}

  async get<T extends JsonRecord>(path: string): Promise<T> {
    return this.request<T>(path);
  }

  async post<T extends JsonRecord>(path: string, body?: JsonRecord | FormData): Promise<T> {
    return this.request<T>(path, body);
  }

  private async request<T extends JsonRecord>(path: string, body?: JsonRecord | FormData): Promise<T> {
    const init: RequestInit = { credentials: 'include', headers: {} };
    if (body instanceof FormData) {
      init.method = 'POST';
      init.body = body;
    } else if (body) {
      init.method = 'POST';
      init.headers = { 'Content-Type': 'application/json' };
      init.body = JSON.stringify(body);
    }

    const response = await fetch(path, init);
    if (response.status === 401) {
      this.options.onUnauthorized?.();
    }

    const text = await response.text();
    const payload = parseJson(text, response.status);
    if (isApiError(payload)) {
      throw new ApiError(payload.message, response.status, payload.code);
    }
    if (!response.ok) {
      throw new ApiError(`HTTP ${response.status}`, response.status);
    }
    return payload as T;
  }
}

function parseJson(text: string, statusCode: number): JsonRecord | ApiErrorPayload {
  try {
    return JSON.parse(text) as JsonRecord | ApiErrorPayload;
  } catch {
    throw new ApiError(`服务器响应异常 (HTTP ${statusCode})`, statusCode);
  }
}

function isApiError(payload: JsonRecord | ApiErrorPayload): payload is ApiErrorPayload {
  return payload.status === 'error' && typeof payload.message === 'string';
}
