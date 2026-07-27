import React, { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useDevices, isDeviceOnline } from '../hooks/useDevices';
import { useDebouncedValue } from '../hooks/useDebouncedValue';
import { useDeviceComms } from '../hooks/useDeviceComms';
import { runCommand, parseCommandData } from '../lib/commandRunner';
import { saveLocalMirror } from '../lib/mirror';
import { enrichContactItem, loadContactPrefs, touchRecentContact } from '../features/comms/contacts';
import {
    buildAllConversations,
    buildInboxConversations,
    buildSentConversations,
    enrichSmsItem,
} from '../features/comms/messages';
import { enrichCallItem, formatDurationSec } from '../features/comms/records';
import {
    buildContactIndex,
    formatPhoneDisplay,
    normalizePhone,
    primaryPhone,
    resolveContact,
} from '../features/comms/phones';
import { COMMS_NAV, itemSearchText } from '../features/comms/nav';
import CommsSidebar from '../components/comms/CommsSidebar';
import CommsList from '../components/comms/CommsList';
import ContactDetail from '../components/comms/ContactDetail';
import SmsThread from '../components/comms/SmsThread';

export default function ContactsSmsPage() {
    const [searchParams, setSearchParams] = useSearchParams();
    const { devices, loading } = useDevices();
    const [deviceId, setDeviceId] = useState(() => searchParams.get('device') || '');
    const [filter, setFilter] = useState('contacts_all');
    const [selectedContact, setSelectedContact] = useState(null);
    const [selectedMessage, setSelectedMessage] = useState(null);
    const [activePhone, setActivePhone] = useState('');
    const [thread, setThread] = useState([]);
    const [listSearch, setListSearch] = useState('');
    const [remoteContactSearch, setRemoteContactSearch] = useState('');
    const [remoteSmsSearch, setRemoteSmsSearch] = useState('');
    const [threadLoading, setThreadLoading] = useState(false);
    const [detailTab, setDetailTab] = useState('info');

    const device = devices.find((d) => d.id === deviceId);
    const online = isDeviceOnline(device);

    const comms = useDeviceComms(deviceId);
    const {
        contacts, smsInbox, setSmsInbox, smsSent, conversations, setConversations, callLogs,
        status, setStatus, sourceLabel, syncing, savingDb, exporting, bootstrapLoading, dbCounts,
        bootstrap, syncDevice, saveCurrentToDatabase, loadFromDatabase, exportExcel, fetchContactsLive,
    } = comms;

    const contactsByPhone = useMemo(() => buildContactIndex(contacts), [contacts]);

    useEffect(() => {
        if (deviceId) {
            setSearchParams({ device: deviceId }, { replace: true });
        }
    }, [deviceId, setSearchParams]);

    useEffect(() => {
        if (!deviceId || !contacts.length) return;
        if (!smsInbox.length && !smsSent.length) return;
        const next = buildAllConversations(smsInbox, smsSent, contactsByPhone);
        setConversations(next);
        saveLocalMirror(deviceId, 'sms_conversations', {
            items: next,
            updated_at: new Date().toISOString(),
        });
        // eslint-disable-next-line react-hooks/exhaustive-deps -- intentionally keyed on contactsByPhone
    }, [contactsByPhone, deviceId]);

    const searchContactsRemote = async () => {
        const q = remoteContactSearch.trim();
        if (!q) return;
        if (!online) {
            setListSearch(q);
            setStatus('Offline — filtering saved contacts');
            return;
        }
        setStatus('Refreshing contacts from device…');
        await fetchContactsLive();
        setListSearch(q);
        setStatus('Contacts updated — list filtered');
    };

    const searchSmsRemote = async () => {
        const q = remoteSmsSearch.trim();
        if (!q || !online) return;
        setStatus('Searching SMS on device…');
        const res = await runCommand(deviceId, 'get_sms_messages', { query: q, box: 'all', limit: 200 });
        if (res.status === 'success') {
            const data = parseCommandData(res.data);
            const list = data.messages || [];
            setSmsInbox(list);
            setFilter('sms_inbox');
            setListSearch('');
            setStatus(`Found ${list.length} message(s)`);
        } else {
            setStatus(res.error || 'SMS search failed');
        }
    };

    const listItems = useMemo(() => {
        const prefs = loadContactPrefs(deviceId);
        switch (filter) {
            case 'sms_inbox':
                return buildInboxConversations(smsInbox, contactsByPhone).map((m) =>
                    enrichSmsItem(m, contactsByPhone)
                );
            case 'sms_sent':
                return buildSentConversations(smsSent, contactsByPhone).map((m) =>
                    enrichSmsItem(m, contactsByPhone)
                );
            case 'sms_conv': {
                const base = conversations.length
                    ? conversations
                    : buildAllConversations(smsInbox, smsSent, contactsByPhone);
                return base.map((m) => enrichSmsItem(m, contactsByPhone));
            }
            case 'calls_all':
                return (callLogs || []).map((c) => enrichCallItem(c, contactsByPhone));
            case 'contacts_fav': {
                const fav = new Set(prefs.favorites || []);
                return contacts.filter((c) => fav.has(c.id)).map(enrichContactItem);
            }
            case 'contacts_recent': {
                const map = new Map(contacts.map((c) => [c.id, c]));
                return (prefs.recent || []).map((id) => map.get(id)).filter(Boolean).map(enrichContactItem);
            }
            default:
                return contacts.map(enrichContactItem);
        }
    }, [filter, contacts, smsInbox, smsSent, conversations, callLogs, deviceId, contactsByPhone]);

    const debouncedListSearch = useDebouncedValue(listSearch, 200);

    const filtered = useMemo(() => {
        if (!debouncedListSearch.trim()) return listItems;
        const q = debouncedListSearch.toLowerCase();
        return listItems.filter((item) => itemSearchText(item).includes(q));
    }, [listItems, debouncedListSearch]);

    const listTitle = COMMS_NAV.find((n) => n.id === filter)?.label || 'Contacts';

    const inboxThreadCount = useMemo(
        () => buildInboxConversations(smsInbox, contactsByPhone).length,
        [smsInbox, contactsByPhone]
    );
    const sentThreadCount = useMemo(
        () => buildSentConversations(smsSent, contactsByPhone).length,
        [smsSent, contactsByPhone]
    );

    const detailPerson = useMemo(() => {
        if (selectedContact?.kind === 'contact' || selectedContact?.phones) {
            return enrichContactItem(selectedContact);
        }
        if (activePhone) {
            const contact = resolveContact(contactsByPhone, activePhone);
            if (contact) return enrichContactItem(contact);
            return {
                displayName: '',
                displayPhone: formatPhoneDisplay(activePhone),
                title: formatPhoneDisplay(activePhone) || 'Unknown',
                subtitle: '',
                name: '',
                phones: [{ number: activePhone }],
            };
        }
        return null;
    }, [selectedContact, activePhone, contactsByPhone]);

    const openThread = async (phone, { contact = null, message = null, tab = 'messages' } = {}) => {
        const resolved = contact || resolveContact(contactsByPhone, phone);
        setActivePhone(phone || '');
        setSelectedMessage(message || null);
        setSelectedContact(resolved || (phone ? { name: '', phones: [{ number: phone }], address: phone } : null));
        setDetailTab(tab);
        if (!phone) {
            setThread([]);
            return;
        }
        const phoneKey = normalizePhone(phone);
        if (!online) {
            const local = [...smsInbox, ...smsSent].filter((m) => {
                const addr = m.address || m.phone;
                return addr === phone || normalizePhone(addr) === phoneKey;
            });
            setThread(local.sort((a, b) => (a.date || 0) - (b.date || 0)));
            return;
        }
        setThreadLoading(true);
        try {
            const res = await runCommand(deviceId, 'get_sms_messages', { address: phone, box: 'all', limit: 100 });
            if (res.status === 'success') {
                const data = parseCommandData(res.data);
                const list = (data.messages || []).sort((a, b) => (a.date || 0) - (b.date || 0));
                setThread(list);
            }
        } finally {
            setThreadLoading(false);
        }
    };

    const selectListItem = (item) => {
        if (filter.startsWith('sms')) {
            openThread(item.address || item.phone, {
                contact: item.contact,
                message: null,
                tab: 'messages',
            });
            return;
        }
        if (filter.startsWith('calls') || item.kind === 'call') {
            setSelectedContact({
                name: item.displayName || item.name || '',
                phones: item.number ? [{ number: item.number }] : [],
                kind: 'contact',
            });
            setSelectedMessage({
                body: `${item.type || 'call'} · ${formatDurationSec(item.duration)}`,
                date: item.timestamp,
                id: item.id,
            });
            setActivePhone(item.number || '');
            setDetailTab('info');
            setThread([]);
            return;
        }
        touchRecentContact(deviceId, item.id);
        setSelectedContact(item);
        setSelectedMessage(null);
        setActivePhone(primaryPhone(item));
        setDetailTab('info');
        setThread([]);
    };

    return (
        <div className="hybrid-op-shell">
            <div className="cm-page">
                <div className="cm-shell rp-shell">
                    <CommsSidebar
                        devices={devices}
                        deviceId={deviceId}
                        setDeviceId={setDeviceId}
                        loading={loading}
                        device={device}
                        online={online}
                        filter={filter}
                        setFilter={setFilter}
                        contacts={contacts}
                        callLogs={callLogs}
                        inboxThreadCount={inboxThreadCount}
                        sentThreadCount={sentThreadCount}
                        dbCounts={dbCounts}
                        syncing={syncing}
                        savingDb={savingDb}
                        exporting={exporting}
                        bootstrapLoading={bootstrapLoading}
                        syncDevice={syncDevice}
                        saveCurrentToDatabase={saveCurrentToDatabase}
                        loadFromDatabase={loadFromDatabase}
                        bootstrap={bootstrap}
                        fetchContactsLive={fetchContactsLive}
                        exportExcel={exportExcel}
                        remoteContactSearch={remoteContactSearch}
                        setRemoteContactSearch={setRemoteContactSearch}
                        remoteSmsSearch={remoteSmsSearch}
                        setRemoteSmsSearch={setRemoteSmsSearch}
                        searchContactsRemote={searchContactsRemote}
                        searchSmsRemote={searchSmsRemote}
                    />

                    <CommsList
                        listTitle={listTitle}
                        listSearch={listSearch}
                        setListSearch={setListSearch}
                        deviceId={deviceId}
                        bootstrapLoading={bootstrapLoading}
                        filtered={filtered}
                        selectedMessage={selectedMessage}
                        selectedContact={selectedContact}
                        activePhone={activePhone}
                        onSelectItem={selectListItem}
                    />

                    <aside className="cm-details-col rp-col">
                        <header className="cm-details-header">
                            <div className="cm-details-tabs">
                                {['info', 'messages', 'statistics'].map((tab) => (
                                    <button
                                        key={tab}
                                        type="button"
                                        className={`cm-tab ${detailTab === tab ? 'cm-tab-active' : ''}`}
                                        onClick={() => {
                                            setDetailTab(tab);
                                            if (tab === 'messages' && activePhone && thread.length === 0) {
                                                openThread(activePhone, {
                                                    contact: selectedContact,
                                                    message: selectedMessage,
                                                    tab: 'messages',
                                                });
                                            }
                                        }}
                                    >
                                        {tab}
                                    </button>
                                ))}
                            </div>
                        </header>
                        <div className="cm-details-body cm-scroll-hide">
                            {!detailPerson ? (
                                <p className="cm-empty">Select a contact or conversation</p>
                            ) : detailTab === 'messages' ? (
                                <SmsThread
                                    detailPerson={detailPerson}
                                    thread={thread}
                                    threadLoading={threadLoading}
                                />
                            ) : detailTab === 'statistics' ? (
                                <div className="cm-stat-cards">
                                    <div className="cm-stat-card">
                                        <span className="cm-stat-num">{contacts.length}</span>
                                        <span className="cm-stat-label">Contacts</span>
                                    </div>
                                    <div className="cm-stat-card">
                                        <span className="cm-stat-num">{smsInbox.length}</span>
                                        <span className="cm-stat-label">Inbox</span>
                                    </div>
                                    <div className="cm-stat-card">
                                        <span className="cm-stat-num">{smsSent.length}</span>
                                        <span className="cm-stat-label">Sent</span>
                                    </div>
                                    <div className="cm-stat-card">
                                        <span className="cm-stat-num">{callLogs.length}</span>
                                        <span className="cm-stat-label">Call logs</span>
                                    </div>
                                    <div className="cm-stat-card">
                                        <span className="cm-stat-num">{dbCounts.contacts + dbCounts.messages + dbCounts.calls}</span>
                                        <span className="cm-stat-label">DB rows</span>
                                    </div>
                                </div>
                            ) : (
                                <ContactDetail
                                    detailPerson={detailPerson}
                                    activePhone={activePhone}
                                    selectedMessage={selectedMessage}
                                    openThread={openThread}
                                />
                            )}
                        </div>
                    </aside>
                </div>
                <footer className="cm-statusbar suite-statusbar">
                    <span className="suite-statusbar-item">{status}</span>
                    <span className="suite-statusbar-item suite-statusbar-right">
                        Source: {sourceLabel}
                    </span>
                </footer>
            </div>
        </div>
    );
}
