import { api, getAdminWebSocketUrl } from '../api/client';

/** @type {WebSocket | null} */
let socket = null;
/** @type {string | null} */
let activeUrl = null;
/** @type {Set<(connected: boolean) => void>} */
const listeners = new Set();
/** @type {Set<(message: object) => void>} */
const deviceEventListeners = new Set();
let subscriberCount = 0;
/** @type {ReturnType<typeof setTimeout> | null} */
let disconnectTimer = null;

function isConnected() {
    return socket != null && socket.readyState === WebSocket.OPEN;
}

function notify() {
    const connected = isConnected();
    listeners.forEach((listener) => listener(connected));
}

function clearDisconnectTimer() {
    if (disconnectTimer != null) {
        clearTimeout(disconnectTimer);
        disconnectTimer = null;
    }
}

function connect() {
    clearDisconnectTimer();
    const url = getAdminWebSocketUrl();
    if (!url) {
        if (socket) {
            socket.close();
            socket = null;
            activeUrl = null;
        }
        notify();
        return;
    }

    if (socket && activeUrl === url && socket.readyState <= WebSocket.OPEN) {
        return;
    }

    if (socket) {
        socket.close();
        socket = null;
    }

    activeUrl = url;
    socket = new WebSocket(url);

    socket.onopen = () => notify();
    socket.onmessage = (event) => {
        try {
            let msg = JSON.parse(event.data);
            if (msg?.type === 'enc' && api.ckx1?.ready()) {
                const plain = api.ckx1.open(msg);
                msg = JSON.parse(new TextDecoder().decode(plain));
            }
            if (msg?.type === 'device_online' || msg?.type === 'device_offline') {
                deviceEventListeners.forEach((listener) => listener(msg));
            }
        } catch {
            // ignore non-JSON / decrypt failures
        }
    };
    socket.onclose = () => {
        notify();
        if (subscriberCount > 0 && getAdminWebSocketUrl() === activeUrl) {
            setTimeout(connect, 2000);
        }
    };
    socket.onerror = () => notify();
}

/**
 * Shared admin WebSocket (survives React Strict Mode double-mount in dev).
 * @param {(connected: boolean) => void} listener
 * @returns {() => void} unsubscribe
 */
export function subscribeAdminWebSocket(listener) {
    subscriberCount += 1;
    listeners.add(listener);
    listener(isConnected());
    connect();

    return () => {
        listeners.delete(listener);
        subscriberCount -= 1;
        if (subscriberCount <= 0) {
            subscriberCount = 0;
            clearDisconnectTimer();
            disconnectTimer = setTimeout(() => {
                if (subscriberCount === 0 && socket) {
                    socket.close();
                    socket = null;
                    activeUrl = null;
                }
                disconnectTimer = null;
            }, 150);
        }
    };
}

/**
 * @param {(message: object) => void} listener
 * @returns {() => void} unsubscribe
 */
export function subscribeAdminDeviceEvents(listener) {
    deviceEventListeners.add(listener);
    connect();
    return () => deviceEventListeners.delete(listener);
}
