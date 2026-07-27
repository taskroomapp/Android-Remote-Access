import { useCallback, useEffect, useState } from 'react';
import { api } from '../api/client';
import { runCommand, parseCommandData } from '../lib/commandRunner';
import { fetchServerMirror, loadLocalMirror, saveLocalMirror } from '../lib/mirror';
import { downloadBlob } from '../lib/download';
import { storedContactsToUi } from '../features/comms/contacts';
import {
    buildAllConversations,
    smsTypeToNumber,
    storedSmsToUi,
} from '../features/comms/messages';
import { storedCallsToUi } from '../features/comms/records';
import { buildContactIndex, resolveContact } from '../features/comms/phones';
import { itemsFromSnapshot } from '../features/comms/nav';

function pickLatest(local, server) {
    if (!local && !server) return null;
    if (!local) return server;
    if (!server) return local;
    const lt = new Date(local.updated_at || 0).getTime();
    const st = new Date(server.updated_at || 0).getTime();
    return st > lt ? server : local;
}

/**
 * Device communications data: contacts, SMS, call logs — bootstrap, sync, DB persist/export.
 */
export function useDeviceComms(deviceId) {
    const [contacts, setContacts] = useState([]);
    const [smsInbox, setSmsInbox] = useState([]);
    const [smsSent, setSmsSent] = useState([]);
    const [conversations, setConversations] = useState([]);
    const [callLogs, setCallLogs] = useState([]);
    const [status, setStatus] = useState('');
    const [sourceLabel, setSourceLabel] = useState('local mirror');
    const [syncing, setSyncing] = useState(false);
    const [savingDb, setSavingDb] = useState(false);
    const [exporting, setExporting] = useState(false);
    const [bootstrapLoading, setBootstrapLoading] = useState(false);
    const [dbCounts, setDbCounts] = useState({ contacts: 0, messages: 0, calls: 0 });

    const refreshDbCounts = useCallback(async (id) => {
        if (!id) return;
        try {
            const res = await api.listDeviceComms(id, 'all', 1);
            const c = res.counts || {};
            setDbCounts({
                contacts: c.contacts || 0,
                messages: c.messages || 0,
                calls: c.calls || 0,
            });
        } catch {
            /* optional */
        }
    }, []);

    const bootstrap = useCallback(async (id) => {
        setBootstrapLoading(true);
        setStatus('Loading saved mirrors…');
        try {
            const localContacts = loadLocalMirror(id, 'contacts');
            const localInbox = loadLocalMirror(id, 'sms_inbox');
            const localCalls = loadLocalMirror(id, 'call_logs');
            const serverContacts = await fetchServerMirror(id, 'contacts');
            const serverInbox = await fetchServerMirror(id, 'sms_inbox');
            const serverCalls = await fetchServerMirror(id, 'call_logs');
            const contactsSnap = pickLatest(localContacts, serverContacts);
            const inboxSnap = pickLatest(localInbox, serverInbox);
            const callsSnap = pickLatest(localCalls, serverCalls);
            if (contactsSnap) setContacts(itemsFromSnapshot(contactsSnap));
            if (inboxSnap) setSmsInbox(itemsFromSnapshot(inboxSnap));
            if (callsSnap) setCallLogs(itemsFromSnapshot(callsSnap));
            setSmsSent(itemsFromSnapshot(loadLocalMirror(id, 'sms_sent')));
            setConversations(itemsFromSnapshot(loadLocalMirror(id, 'sms_conversations')));
            await refreshDbCounts(id);
            const any = contactsSnap || inboxSnap || callsSnap;
            setSourceLabel(!any ? 'none' : 'local/server mirror');
            setStatus(!any ? 'No data — sync when online' : 'Loaded from saved data');
        } finally {
            setBootstrapLoading(false);
        }
    }, [refreshDbCounts]);

    useEffect(() => {
        if (deviceId) bootstrap(deviceId);
    }, [deviceId, bootstrap]);

    const persistToDatabase = useCallback(async ({
        contactList = contacts,
        inboxList = smsInbox,
        sentList = smsSent,
        callList = callLogs,
    } = {}) => {
        const index = buildContactIndex(contactList);
        const messages = [...(inboxList || []), ...(sentList || [])].map((m) => {
            const contact = resolveContact(index, m.address || m.phone);
            return {
                ...m,
                name: m.name || contact?.name || '',
                person: m.person || contact?.id || '',
            };
        });
        const calls = (callList || []).map((c) => {
            const contact = resolveContact(index, c.number);
            return {
                ...c,
                name: c.name || contact?.name || '',
                contact_id: c.contact_id || contact?.id || '',
            };
        });
        const res = await api.saveDeviceComms(deviceId, {
            contacts: contactList,
            messages,
            calls,
        });
        await refreshDbCounts(deviceId);
        return res?.saved || {};
    }, [contacts, smsInbox, smsSent, callLogs, deviceId, refreshDbCounts]);

    const fetchLiveSmsExtras = useCallback(async (inboxList = smsInbox, contactList = contacts) => {
        const sent = await runCommand(deviceId, 'get_sms_messages', { box: 'sent', limit: 500 });
        let sentList = smsSent;
        if (sent.status === 'success') {
            const data = parseCommandData(sent.data);
            sentList = data.messages || [];
            setSmsSent(sentList);
            saveLocalMirror(deviceId, 'sms_sent', { items: sentList, updated_at: new Date().toISOString() });
        }
        const conv = buildAllConversations(inboxList, sentList, buildContactIndex(contactList));
        setConversations(conv);
        saveLocalMirror(deviceId, 'sms_conversations', {
            items: conv,
            updated_at: new Date().toISOString(),
        });
        return sentList;
    }, [deviceId, smsInbox, contacts, smsSent]);

    const syncDevice = useCallback(async (online) => {
        if (!online) {
            alert('Device must be online to sync.');
            return;
        }
        setSyncing(true);
        setStatus('Syncing contacts, SMS & call logs…');
        try {
            let contactList = [];
            let inboxList = [];
            let sentList = [];
            let callList = [];
            const cStatus = await runCommand(deviceId, 'get_contacts', {});
            if (cStatus.status === 'success') {
                const data = parseCommandData(cStatus.data);
                contactList = data.contacts || data || [];
                if (!Array.isArray(contactList)) contactList = [];
                setContacts(contactList);
                saveLocalMirror(deviceId, 'contacts', { updated_at: new Date().toISOString(), items: contactList, source: 'device' });
            }
            const sStatus = await runCommand(deviceId, 'get_sms_messages', { limit: 1000 });
            if (sStatus.status === 'success') {
                const data = parseCommandData(sStatus.data);
                inboxList = data.messages || data || [];
                if (!Array.isArray(inboxList)) inboxList = [];
                setSmsInbox(inboxList);
                saveLocalMirror(deviceId, 'sms_inbox', { updated_at: new Date().toISOString(), items: inboxList, source: 'device' });
            }
            const callStatus = await runCommand(deviceId, 'get_call_logs', { limit: 1000 });
            if (callStatus.status === 'success') {
                const data = parseCommandData(callStatus.data);
                callList = data.calls || data || [];
                if (!Array.isArray(callList)) callList = [];
                setCallLogs(callList);
                saveLocalMirror(deviceId, 'call_logs', { updated_at: new Date().toISOString(), items: callList, source: 'device' });
            }
            sentList = await fetchLiveSmsExtras(inboxList, contactList);
            try {
                await api.mirrorUpdate(deviceId, {
                    types: ['contacts', 'sms_inbox', 'sms_sent', 'call_logs'],
                    snapshots: {
                        contacts: { updated_at: new Date().toISOString(), items: contactList, source: 'device' },
                        sms_inbox: { updated_at: new Date().toISOString(), items: inboxList, source: 'device' },
                        sms_sent: { updated_at: new Date().toISOString(), items: sentList, source: 'device' },
                        call_logs: { updated_at: new Date().toISOString(), items: callList, source: 'device' },
                    },
                });
            } catch {
                /* optional */
            }
            try {
                const saved = await persistToDatabase({ contactList, inboxList, sentList, callList });
                setSourceLabel('device + database');
                setStatus(
                    `Sync complete — saved ${saved.contacts_saved || 0} contacts, ` +
                    `${saved.messages_saved || 0} messages, ${saved.calls_saved || 0} calls`
                );
            } catch (dbErr) {
                setSourceLabel('device + mirror');
                setStatus(`Sync complete (DB save failed: ${dbErr.message || 'error'})`);
            }
        } catch (err) {
            setStatus(err.message || 'Sync failed');
        } finally {
            setSyncing(false);
        }
    }, [deviceId, fetchLiveSmsExtras, persistToDatabase]);

    const saveCurrentToDatabase = useCallback(async () => {
        if (!deviceId) return;
        setSavingDb(true);
        setStatus('Saving to database…');
        try {
            const saved = await persistToDatabase();
            setSourceLabel('database');
            setStatus(
                `Saved ${saved.contacts_saved || 0} contacts, ` +
                `${saved.messages_saved || 0} messages, ${saved.calls_saved || 0} calls`
            );
        } catch (err) {
            setStatus(err.message || 'Database save failed');
        } finally {
            setSavingDb(false);
        }
    }, [deviceId, persistToDatabase]);

    const loadFromDatabase = useCallback(async () => {
        if (!deviceId) return;
        setBootstrapLoading(true);
        setStatus('Loading from database…');
        try {
            const res = await api.listDeviceComms(deviceId, 'all', 10000);
            const contactList = storedContactsToUi(res.contacts);
            const allSms = storedSmsToUi(res.messages);
            const callList = storedCallsToUi(res.calls);
            const inboxList = allSms.filter((m) => smsTypeToNumber(m.type) !== 2);
            const sentList = allSms.filter((m) => smsTypeToNumber(m.type) === 2);
            setContacts(contactList);
            setSmsInbox(inboxList);
            setSmsSent(sentList);
            setCallLogs(callList);
            setConversations(buildAllConversations(inboxList, sentList, buildContactIndex(contactList)));
            const c = res.counts || {};
            setDbCounts({
                contacts: c.contacts ?? contactList.length,
                messages: c.messages ?? allSms.length,
                calls: c.calls ?? callList.length,
            });
            setSourceLabel('database');
            setStatus(
                `Loaded from DB — ${contactList.length} contacts, ${allSms.length} messages, ${callList.length} calls`
            );
        } catch (err) {
            setStatus(err.message || 'Failed to load from database');
        } finally {
            setBootstrapLoading(false);
        }
    }, [deviceId]);

    const exportExcel = useCallback(async (type = 'all') => {
        if (!deviceId) return;
        setExporting(true);
        setStatus(`Exporting ${type} to Excel…`);
        try {
            try {
                await persistToDatabase();
            } catch {
                /* export whatever is already stored */
            }
            const blob = await api.exportDeviceComms(deviceId, type);
            downloadBlob(blob, `device-comms-${type}-${new Date().toISOString().slice(0, 10)}.xlsx`);
            setStatus(`Excel export ready (${type})`);
        } catch (err) {
            setStatus(err.message || 'Excel export failed');
        } finally {
            setExporting(false);
        }
    }, [deviceId, persistToDatabase]);

    const fetchContactsLive = useCallback(async () => {
        const statusRes = await runCommand(deviceId, 'get_contacts', {});
        if (statusRes.status === 'success') {
            const data = parseCommandData(statusRes.data);
            const list = data.contacts || data || [];
            setContacts(Array.isArray(list) ? list : []);
            saveLocalMirror(deviceId, 'contacts', { items: list, source: 'direct', updated_at: new Date().toISOString() });
        }
    }, [deviceId]);

    return {
        contacts,
        setContacts,
        smsInbox,
        setSmsInbox,
        smsSent,
        setSmsSent,
        conversations,
        setConversations,
        callLogs,
        setCallLogs,
        status,
        setStatus,
        sourceLabel,
        syncing,
        savingDb,
        exporting,
        bootstrapLoading,
        dbCounts,
        bootstrap,
        syncDevice,
        saveCurrentToDatabase,
        loadFromDatabase,
        exportExcel,
        fetchContactsLive,
    };
}
