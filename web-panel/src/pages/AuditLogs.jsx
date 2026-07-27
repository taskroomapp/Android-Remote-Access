import React, { useState, useEffect } from 'react';
import { api } from '../api/client';
import Icon from '../components/ui/Icon';

const COMMAND_TYPES = [
    { value: '', label: 'All Commands' },
    { value: 'file_list', label: 'File List' },
    { value: 'file_read', label: 'File Read' },
    { value: 'file_delete', label: 'File Delete' },
    { value: 'get_contacts', label: 'Get Contacts' },
    { value: 'get_call_logs', label: 'Get Call Logs' },
    { value: 'camera_snapshot', label: 'Camera Snapshot' },
    { value: 'mic_start', label: 'Microphone Recording' },
    { value: 'get_device_info', label: 'Device Info' },
    { value: 'get_location', label: 'Location' },
];

const STATUS_TYPES = [
    { value: '', label: 'All Status' },
    { value: 'success', label: 'Success' },
    { value: 'failed', label: 'Failed' },
    { value: 'pending', label: 'Pending' },
    { value: 'queued', label: 'Queued' },
    { value: 'timeout', label: 'Timeout' },
];

export default function AuditLogs() {
    const [logs, setLogs] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [filters, setFilters] = useState({
        query: '',
        command_type: '',
        status: '',
        start_date: '',
        end_date: '',
    });
    const [pagination, setPagination] = useState({
        page: 1,
        page_size: 50,
        total: 0,
        total_pages: 0,
    });
    const [searchInput, setSearchInput] = useState({
        query: '',
        command_type: '',
        status: '',
        start_date: '',
        end_date: '',
    });

    useEffect(() => {
        searchLogs();
    }, [pagination.page]);

    const searchLogs = async () => {
        try {
            setLoading(true);
            setError(null);

            const params = {
                ...searchInput,
                page: pagination.page,
                page_size: pagination.page_size,
            };

            // Remove empty filters
            Object.keys(params).forEach(key => {
                if (params[key] === '' || params[key] === null) {
                    delete params[key];
                }
            });

            const data = await api.searchAuditLogs(params);
            setLogs(data.results || []);
            setPagination(prev => ({
                ...prev,
                total: data.total,
                total_pages: data.total_pages,
            }));
        } catch (err) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    };

    const handleFilterChange = (key, value) => {
        setSearchInput(prev => ({ ...prev, [key]: value }));
    };

    const applyFilters = () => {
        setFilters(searchInput);
        setPagination(prev => ({ ...prev, page: 1 }));
        searchLogs();
    };

    const clearFilters = () => {
        setSearchInput({
            query: '',
            command_type: '',
            status: '',
            start_date: '',
            end_date: '',
        });
        setFilters({
            query: '',
            command_type: '',
            status: '',
            start_date: '',
            end_date: '',
        });
        setPagination(prev => ({ ...prev, page: 1 }));
        searchLogs();
    };

    const goToPage = (page) => {
        setPagination(prev => ({ ...prev, page }));
    };

    return (
        <div className="audit-logs-page">
            <header className="page-header">
                <h1><Icon name="audit" size={24} /> Audit Logs</h1>
                <p className="subtitle">Complete forensic chain of custody for all system operations</p>
            </header>

            {/* Filters */}
            <div className="filters-section">
                <div className="filter-row">
                    <div className="filter-group">
                        <label>Search</label>
                        <input
                            type="text"
                            placeholder="Search logs..."
                            value={searchInput.query}
                            onChange={(e) => handleFilterChange('query', e.target.value)}
                            onKeyPress={(e) => e.key === 'Enter' && applyFilters()}
                        />
                    </div>
                    <div className="filter-group">
                        <label>Command Type</label>
                        <select
                            value={searchInput.command_type}
                            onChange={(e) => handleFilterChange('command_type', e.target.value)}
                        >
                            {COMMAND_TYPES.map(type => (
                                <option key={type.value} value={type.value}>{type.label}</option>
                            ))}
                        </select>
                    </div>
                    <div className="filter-group">
                        <label>Status</label>
                        <select
                            value={searchInput.status}
                            onChange={(e) => handleFilterChange('status', e.target.value)}
                        >
                            {STATUS_TYPES.map(status => (
                                <option key={status.value} value={status.value}>{status.label}</option>
                            ))}
                        </select>
                    </div>
                    <div className="filter-group">
                        <label>From Date</label>
                        <input
                            type="date"
                            value={searchInput.start_date}
                            onChange={(e) => handleFilterChange('start_date', e.target.value)}
                        />
                    </div>
                    <div className="filter-group">
                        <label>To Date</label>
                        <input
                            type="date"
                            value={searchInput.end_date}
                            onChange={(e) => handleFilterChange('end_date', e.target.value)}
                        />
                    </div>
                </div>
                <div className="filter-actions">
                    <button className="btn-secondary" onClick={clearFilters}>Clear</button>
                    <button className="btn-primary" onClick={applyFilters}>Apply Filters</button>
                </div>
            </div>

            {/* Results */}
            <div className="logs-results">
                <div className="results-summary">
                    <span>Showing {logs.length} of {pagination.total} entries</span>
                </div>

                {error && (
                    <div className="error-banner">
                        {error}
                        <button onClick={searchLogs}>Retry</button>
                    </div>
                )}

                {loading ? (
                    <div className="loading-state">
                        <span className="spinner"></span>
                        <p>Loading audit logs...</p>
                    </div>
                ) : logs.length === 0 ? (
                    <div className="empty-state">
                        <span className="empty-icon"><Icon name="audit" size={40} /></span>
                        <h3>No Logs Found</h3>
                        <p>No audit logs match your search criteria.</p>
                    </div>
                ) : (
                    <>
                        <table className="audit-table">
                            <thead>
                                <tr>
                                    <th>Timestamp</th>
                                    <th>Administrator</th>
                                    <th>Device</th>
                                    <th>Command</th>
                                    <th>Status</th>
                                    <th>IP Address</th>
                                    <th>Transaction ID</th>
                                </tr>
                            </thead>
                            <tbody>
                                {logs.map(log => (
                                    <tr key={log.id} className={`status-${log.status}`}>
                                        <td className="timestamp-cell">
                                            {formatTimestamp(log.timestamp)}
                                        </td>
                                        <td className="admin-cell">
                                            <span className="admin-name">{log.administrator_name}</span>
                                        </td>
                                        <td className="device-cell">
                                            <span className="device-name">{log.device_name}</span>
                                            <code className="device-id">{log.device_id.substring(0, 8)}...</code>
                                        </td>
                                        <td className="command-cell">
                                            <span className="command-type">{formatCommandType(log.command_type)}</span>
                                            {log.command_payload && (
                                                <span className="command-payload">{log.command_payload}</span>
                                            )}
                                        </td>
                                        <td className="status-cell">
                                            <span className={`status-badge ${log.status}`}>
                                                {log.status}
                                            </span>
                                        </td>
                                        <td className="ip-cell">
                                            <code>{log.ip_address}</code>
                                        </td>
                                        <td className="txn-cell">
                                            <code className="transaction-id">{log.transaction_id}</code>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>

                        {/* Pagination */}
                        {pagination.total_pages > 1 && (
                            <div className="pagination">
                                <button
                                    className="page-btn"
                                    disabled={pagination.page === 1}
                                    onClick={() => goToPage(pagination.page - 1)}
                                >
                                    Previous
                                </button>
                                <span className="page-info">
                                    Page {pagination.page} of {pagination.total_pages}
                                </span>
                                <button
                                    className="page-btn"
                                    disabled={pagination.page === pagination.total_pages}
                                    onClick={() => goToPage(pagination.page + 1)}
                                >
                                    Next
                                </button>
                            </div>
                        )}
                    </>
                )}
            </div>
        </div>
    );
}

function formatTimestamp(timestamp) {
    const date = new Date(timestamp);
    return (
        <span className="timestamp">
            <span className="date">{date.toLocaleDateString()}</span>
            <span className="time">{date.toLocaleTimeString()}</span>
        </span>
    );
}

function formatCommandType(type) {
    if (!type) return '-';
    return type
        .replace(/_/g, ' ')
        .replace(/\b\w/g, c => c.toUpperCase());
}
