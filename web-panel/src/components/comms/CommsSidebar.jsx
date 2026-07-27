import React from 'react';
import DevicePicker from '../DevicePicker';
import Icon from '../ui/Icon';
import { COMMS_NAV, avatarClass } from '../../features/comms/nav';

export default function CommsSidebar({
    devices,
    deviceId,
    setDeviceId,
    loading,
    device,
    online,
    filter,
    setFilter,
    contacts,
    callLogs,
    inboxThreadCount,
    sentThreadCount,
    dbCounts,
    syncing,
    savingDb,
    exporting,
    bootstrapLoading,
    syncDevice,
    saveCurrentToDatabase,
    loadFromDatabase,
    bootstrap,
    fetchContactsLive,
    exportExcel,
    remoteContactSearch,
    setRemoteContactSearch,
    remoteSmsSearch,
    setRemoteSmsSearch,
    searchContactsRemote,
    searchSmsRemote,
}) {
    return (
        <aside className="cm-sidebar rp-col">
            <section className="cm-sidebar-section">
                <h5 className="cm-section-title">
                    <Icon name="devices" size={14} /> Device
                </h5>
                <DevicePicker
                    id="contacts-device-select"
                    variant="hybrid"
                    devices={devices}
                    value={deviceId}
                    onChange={setDeviceId}
                    loading={loading}
                    placeholder="Select device…"
                />
                <div className="cm-device-info">
                    {device && (
                        <div className="cm-device-card">
                            <div className={`cm-avatar cm-avatar-sm ${avatarClass(device.friendly_name)}`}>
                                {(device.friendly_name || 'D')[0]}
                            </div>
                            <div className="cm-device-meta">
                                <strong>{device.friendly_name || device.id}</strong>
                                <span className="cm-device-sub">{online ? 'Online' : 'Offline'}</span>
                            </div>
                        </div>
                    )}
                </div>
            </section>

            <section className="cm-sidebar-section">
                <h5 className="cm-section-title">
                    <Icon name="contacts" size={14} /> Contacts
                </h5>
                <nav className="cm-nav">
                    {COMMS_NAV.filter((n) => n.group === 'contacts').map((n) => (
                        <button
                            key={n.id}
                            type="button"
                            className={`cm-nav-item ${filter === n.id ? 'cm-nav-active' : ''}`}
                            onClick={() => setFilter(n.id)}
                        >
                            <Icon name={n.icon} size={16} /> {n.label}
                        </button>
                    ))}
                </nav>
            </section>

            <section className="cm-sidebar-section">
                <h5 className="cm-section-title">
                    <Icon name="message" size={14} /> Text messages
                </h5>
                <nav className="cm-nav">
                    {COMMS_NAV.filter((n) => n.group === 'sms').map((n) => (
                        <button
                            key={n.id}
                            type="button"
                            className={`cm-nav-item ${filter === n.id ? 'cm-nav-active' : ''}`}
                            onClick={() => setFilter(n.id)}
                        >
                            <Icon name={n.icon} size={16} /> {n.label}
                            {n.id === 'sms_inbox' && inboxThreadCount > 0 && (
                                <span className="cm-nav-badge">{inboxThreadCount}</span>
                            )}
                            {n.id === 'sms_sent' && sentThreadCount > 0 && (
                                <span className="cm-nav-badge cm-nav-badge-muted">{sentThreadCount}</span>
                            )}
                        </button>
                    ))}
                </nav>
            </section>

            <section className="cm-sidebar-section cm-sidebar-grow">
                <h5 className="cm-section-title">
                    <Icon name="phone" size={14} /> Calls
                </h5>
                <nav className="cm-nav">
                    {COMMS_NAV.filter((n) => n.group === 'calls').map((n) => (
                        <button
                            key={n.id}
                            type="button"
                            className={`cm-nav-item ${filter === n.id ? 'cm-nav-active' : ''}`}
                            onClick={() => setFilter(n.id)}
                        >
                            <Icon name={n.icon} size={16} /> {n.label}
                            {callLogs.length > 0 && (
                                <span className="cm-nav-badge cm-nav-badge-muted">{callLogs.length}</span>
                            )}
                        </button>
                    ))}
                </nav>
            </section>

            <footer className="cm-sidebar-footer">
                <button type="button" className="cm-btn cm-btn-accent" onClick={() => syncDevice(online)} disabled={!deviceId || !online || syncing}>
                    <Icon name="refresh" size={16} className={syncing ? 'spin' : ''} /> Sync from device
                </button>
                <button type="button" className="cm-btn" onClick={saveCurrentToDatabase} disabled={!deviceId || savingDb}>
                    <Icon name="server" size={16} className={savingDb ? 'spin' : ''} /> Save to database
                </button>
                <button type="button" className="cm-btn" onClick={loadFromDatabase} disabled={!deviceId || bootstrapLoading}>
                    <Icon name="cloud" size={16} /> Load from database
                </button>
                <button type="button" className="cm-btn" onClick={() => bootstrap(deviceId)} disabled={!deviceId}>
                    <Icon name="storage" size={16} /> Reload saved
                </button>
                <button type="button" className="cm-btn" onClick={fetchContactsLive} disabled={!deviceId || !online}>
                    <Icon name="download" size={16} /> Fetch contacts
                </button>
                <div className="cm-export-row">
                    <button type="button" className="cm-btn" onClick={() => exportExcel('all')} disabled={!deviceId || exporting}>
                        <Icon name="fileText" size={16} /> Export Excel
                    </button>
                    <button type="button" className="cm-icon-btn" title="Export contacts" onClick={() => exportExcel('contacts')} disabled={!deviceId || exporting}>
                        <Icon name="contacts" size={16} />
                    </button>
                    <button type="button" className="cm-icon-btn" title="Export messages" onClick={() => exportExcel('sms')} disabled={!deviceId || exporting}>
                        <Icon name="message" size={16} />
                    </button>
                    <button type="button" className="cm-icon-btn" title="Export call logs" onClick={() => exportExcel('calls')} disabled={!deviceId || exporting}>
                        <Icon name="phone" size={16} />
                    </button>
                </div>
                <div className="cm-sidebar-search-row">
                    <input
                        type="search"
                        className="cm-input-sm"
                        placeholder="Search on device…"
                        value={remoteContactSearch}
                        onChange={(e) => setRemoteContactSearch(e.target.value)}
                    />
                    <button
                        type="button"
                        className="cm-icon-btn"
                        title="Search contacts"
                        disabled={!online && !remoteContactSearch.trim()}
                        onClick={searchContactsRemote}
                    >
                        <Icon name="search" size={16} />
                    </button>
                </div>
                <div className="cm-sidebar-search-row">
                    <input
                        type="search"
                        className="cm-input-sm"
                        placeholder="Search SMS on device…"
                        value={remoteSmsSearch}
                        onChange={(e) => setRemoteSmsSearch(e.target.value)}
                    />
                    <button
                        type="button"
                        className="cm-icon-btn"
                        title="Search SMS"
                        disabled={!online || !remoteSmsSearch.trim()}
                        onClick={searchSmsRemote}
                    >
                        <Icon name="search" size={16} />
                    </button>
                </div>
                <p className="cm-sidebar-meta">
                    {contacts.length} contacts · {callLogs.length} calls
                </p>
                <p className="cm-sidebar-meta">
                    {inboxThreadCount} inbox threads / {sentThreadCount} sent threads
                </p>
                <p className="cm-sidebar-meta">
                    DB: {dbCounts.contacts} contacts · {dbCounts.messages} msgs · {dbCounts.calls} calls
                </p>
            </footer>
        </aside>
    );
}
