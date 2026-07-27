import { useCallback, useMemo, useState } from 'react';
import { api } from '../api/client';
import { downloadBlob } from '../lib/download';

/**
 * Shared capture gallery session: selection, DB save/load/export, clear/remove/download.
 */
export function useMediaSession(deviceId) {
    const [captures, setCaptures] = useState([]);
    const [activeId, setActiveId] = useState(null);
    const [tab, setTab] = useState('camera');
    const [timelineFilter, setTimelineFilter] = useState('all');
    const [lightboxOpen, setLightboxOpen] = useState(false);
    const [dbStatus, setDbStatus] = useState('');
    const [dbBusy, setDbBusy] = useState(false);

    const sliderItems = useMemo(() => {
        if (timelineFilter === 'photo') return captures.filter((c) => c.kind === 'camera');
        if (timelineFilter === 'audio') return captures.filter((c) => c.kind === 'audio');
        return captures;
    }, [captures, timelineFilter]);

    const active = sliderItems.find((c) => c.id === activeId) || sliderItems[0] || null;
    const activeIndex = active ? sliderItems.findIndex((c) => c.id === active.id) : -1;

    const selectCapture = useCallback((entry) => {
        if (!entry) return;
        setActiveId(entry.id);
        setTab(entry.kind === 'audio' ? 'audio' : 'camera');
    }, []);

    const goToIndex = useCallback((index) => {
        if (index < 0 || index >= sliderItems.length) return;
        selectCapture(sliderItems[index]);
    }, [sliderItems, selectCapture]);

    const goPrev = useCallback(() => goToIndex(activeIndex - 1), [goToIndex, activeIndex]);
    const goNext = useCallback(() => goToIndex(activeIndex + 1), [goToIndex, activeIndex]);

    const addCapture = useCallback((entry, preferTab) => {
        setCaptures((prev) => [entry, ...prev]);
        setActiveId(entry.id);
        setTab(preferTab || (entry.kind === 'audio' ? 'audio' : 'camera'));
        setTimelineFilter('all');
    }, []);

    const clearSession = useCallback(() => {
        setCaptures([]);
        setActiveId(null);
        setLightboxOpen(false);
    }, []);

    const removeActive = useCallback(() => {
        if (!active) return;
        setCaptures((prev) => {
            const next = prev.filter((c) => c.id !== active.id);
            const fallback = next[0] || null;
            setActiveId(fallback?.id ?? null);
            if (fallback) setTab(fallback.kind === 'audio' ? 'audio' : 'camera');
            return next;
        });
        setLightboxOpen(false);
    }, [active]);

    const downloadActive = useCallback(() => {
        if (!active?.previewUrl) return;
        const a = document.createElement('a');
        a.href = active.previewUrl;
        a.download = active.fileName || 'capture.bin';
        document.body.appendChild(a);
        a.click();
        a.remove();
    }, [active]);

    const saveSessionToDatabase = useCallback(async () => {
        if (!deviceId || !captures.length) return;
        setDbBusy(true);
        setDbStatus('Saving session to database…');
        try {
            const media = captures
                .filter((c) => c.previewUrl)
                .map((c) => ({
                    file_name: c.fileName || `${c.kind}_${c.id}`,
                    file_type: c.kind === 'audio' ? 'audio' : 'image',
                    source: c.kind === 'audio' ? 'mic_stop' : 'camera_snapshot',
                    camera: c.camera || '',
                    mime_type: c.kind === 'audio' ? 'audio/mp4' : 'image/jpeg',
                    data_url: c.previewUrl,
                }));
            const res = await api.saveDeviceArtifacts(deviceId, { media });
            setDbStatus(`Saved ${res?.saved?.media_saved || 0} media item(s)`);
        } catch (err) {
            setDbStatus(err.message || 'Database save failed');
        } finally {
            setDbBusy(false);
        }
    }, [deviceId, captures]);

    const loadFromDatabase = useCallback(async () => {
        if (!deviceId) return;
        setDbBusy(true);
        setDbStatus('Loading media from database…');
        try {
            const res = await api.listDeviceArtifacts(deviceId, 'media', 100);
            const items = res.media || [];
            const loaded = [];
            for (const m of items) {
                try {
                    const blob = await api.downloadMedia(m.id);
                    const previewUrl = URL.createObjectURL(blob);
                    loaded.push({
                        id: m.id,
                        kind: m.file_type === 'audio' ? 'audio' : 'camera',
                        camera: m.camera || '',
                        previewUrl,
                        fileName: m.file_name,
                        size: m.file_size || 0,
                        timestamp: m.data_entry_date || m.created_at || new Date().toISOString(),
                        fromDb: true,
                    });
                } catch {
                    /* skip broken blob */
                }
            }
            if (loaded.length) {
                setCaptures(loaded);
                setActiveId(loaded[0].id);
                setTab(loaded[0].kind === 'audio' ? 'audio' : 'camera');
            }
            setDbStatus(`Loaded ${loaded.length} media item(s) from database`);
        } catch (err) {
            setDbStatus(err.message || 'Failed to load from database');
        } finally {
            setDbBusy(false);
        }
    }, [deviceId]);

    const exportExcel = useCallback(async (type = 'media') => {
        if (!deviceId) return;
        setDbBusy(true);
        try {
            const blob = await api.exportDeviceArtifacts(deviceId, type);
            downloadBlob(blob, `device-${type}-${new Date().toISOString().slice(0, 10)}.xlsx`);
            setDbStatus('Excel export ready');
        } catch (err) {
            setDbStatus(err.message || 'Excel export failed');
        } finally {
            setDbBusy(false);
        }
    }, [deviceId]);

    return {
        captures,
        activeId,
        setActiveId,
        tab,
        setTab,
        timelineFilter,
        setTimelineFilter,
        lightboxOpen,
        setLightboxOpen,
        dbStatus,
        setDbStatus,
        dbBusy,
        sliderItems,
        active,
        activeIndex,
        selectCapture,
        goPrev,
        goNext,
        addCapture,
        clearSession,
        removeActive,
        downloadActive,
        saveSessionToDatabase,
        loadFromDatabase,
        exportExcel,
    };
}
