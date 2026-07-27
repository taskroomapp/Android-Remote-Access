import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import DevicePicker from '../components/DevicePicker';
import Icon from '../components/ui/Icon';
import TruncatedText from '../components/hybrid/TruncatedText';
import CameraControls from '../components/live/CameraControls';
import MicControls from '../components/live/MicControls';
import CaptureStage from '../components/live/CaptureStage';
import CaptureStrip from '../components/live/CaptureStrip';
import CaptureLightbox from '../components/live/CaptureLightbox';
import { useDevices, isDeviceOnline } from '../hooks/useDevices';
import { useMediaSession } from '../hooks/useMediaSession';
import { useCameraCapture } from '../hooks/useCameraCapture';
import { useMicRecording } from '../hooks/useMicRecording';
import { runCommand, parseCommandData } from '../lib/commandRunner';
import { formatBytes, formatSessionClock, formatStorage } from '../features/live/formatters';

export default function LiveViewPage() {
    const [searchParams, setSearchParams] = useSearchParams();
    const { devices, loading } = useDevices({ onlineOnly: true });
    const [deviceId, setDeviceId] = useState(() => searchParams.get('device') || '');
    const [sessionStart, setSessionStart] = useState(null);
    const [elapsed, setElapsed] = useState(0);
    const [deviceInfo, setDeviceInfo] = useState(null);
    const tickRef = useRef(null);
    const stripRef = useRef(null);
    const touchStartX = useRef(null);

    const device = devices.find((d) => d.id === deviceId);

    const session = useMediaSession(deviceId);
    const {
        captures, activeId, tab, setTab, timelineFilter, setTimelineFilter,
        lightboxOpen, setLightboxOpen, dbStatus, setDbStatus, dbBusy,
        sliderItems, active, activeIndex, selectCapture, goPrev, goNext,
        addCapture, clearSession, removeActive, downloadActive,
        saveSessionToDatabase, loadFromDatabase, exportExcel,
    } = session;

    const onSessionStart = useCallback(() => {
        setSessionStart((prev) => prev || Date.now());
    }, []);

    const camera = useCameraCapture({ deviceId, addCapture, setDbStatus, onSessionStart });
    const mic = useMicRecording({ deviceId, addCapture, setDbStatus, onSessionStart, elapsed });
    const busy = camera.busy || mic.busy;

    useEffect(() => {
        if (deviceId) {
            setSearchParams({ device: deviceId }, { replace: true });
        }
    }, [deviceId, setSearchParams]);

    useEffect(() => {
        if (deviceId && isDeviceOnline(device)) {
            let cancelled = false;
            runCommand(deviceId, 'get_device_info', {})
                .then((s) => {
                    if (!cancelled && s.status === 'success') {
                        setDeviceInfo(parseCommandData(s.data));
                    }
                })
                .catch(() => {
                    /* Live info is optional; avoid uncaught CKX1/network races */
                });
            return () => { cancelled = true; };
        }
        return undefined;
    }, [deviceId, device]);

    useEffect(() => {
        if (mic.recording && sessionStart) {
            tickRef.current = setInterval(() => {
                setElapsed(Date.now() - sessionStart);
            }, 500);
        }
        return () => clearInterval(tickRef.current);
    }, [mic.recording, sessionStart]);

    useEffect(() => {
        if (!stripRef.current || activeIndex < 0) return;
        const el = stripRef.current.querySelector(`[data-lv-index="${activeIndex}"]`);
        if (el) el.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
    }, [activeIndex, sliderItems.length]);

    useEffect(() => {
        const onKey = (e) => {
            if (e.target?.tagName === 'INPUT' || e.target?.tagName === 'SELECT' || e.target?.tagName === 'TEXTAREA') return;
            if (e.key === 'ArrowLeft') {
                e.preventDefault();
                goPrev();
            } else if (e.key === 'ArrowRight') {
                e.preventDefault();
                goNext();
            } else if (e.key === 'Escape') {
                setLightboxOpen(false);
            } else if (e.key === 'f' || e.key === 'F') {
                if (active) setLightboxOpen(true);
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [goPrev, goNext, active, setLightboxOpen]);

    const handleClearSession = () => {
        clearSession();
        setSessionStart(null);
        setElapsed(0);
    };

    const onTouchStart = (e) => {
        touchStartX.current = e.changedTouches?.[0]?.clientX ?? null;
    };

    const onTouchEnd = (e) => {
        const start = touchStartX.current;
        touchStartX.current = null;
        if (start == null) return;
        const end = e.changedTouches?.[0]?.clientX ?? start;
        const dx = end - start;
        if (Math.abs(dx) < 48) return;
        if (dx > 0) goPrev();
        else goNext();
    };

    const cameraCount = captures.filter((c) => c.kind === 'camera').length;
    const audioCount = captures.filter((c) => c.kind === 'audio').length;
    const canPrev = activeIndex > 0;
    const canNext = activeIndex >= 0 && activeIndex < sliderItems.length - 1;

    return (
        <div className="hybrid-op-shell">
            <div className="lv-page">
                <div className="suite-topbar">
                    <span className="suite-topbar-title">Capture Studio</span>
                    <span className="suite-badge">
                        <span className={`suite-badge-dot ${mic.recording ? 'recording' : 'online'}`} />
                        Live
                    </span>
                    <span className="suite-clock">{formatSessionClock(elapsed)}</span>
                    <span className="suite-topbar-spacer" />
                    {device && <span className="suite-badge">{device.friendly_name || device.device_name}</span>}
                </div>

                <div className="lv-shell rp-shell">
                    <aside className="lv-controls rp-col">
                        <div className="lv-section">
                            <div className="suite-section-title">Device</div>
                            <DevicePicker
                                id="live-device-select"
                                variant="hybrid"
                                devices={devices}
                                value={deviceId}
                                onChange={setDeviceId}
                                loading={loading}
                                onlineOnly
                                placeholder="Select a device…"
                            />
                        </div>
                        <CameraControls deviceId={deviceId} busy={busy} onCapture={camera.capturePhoto} />
                        <MicControls
                            deviceId={deviceId}
                            busy={busy}
                            recording={mic.recording}
                            onStart={mic.startAudio}
                            onStop={mic.stopAudio}
                        />
                        <div className="lv-section">
                            <div className="suite-section-title">Database</div>
                            <button type="button" className="suite-btn" disabled={!deviceId || dbBusy || !captures.length} onClick={saveSessionToDatabase}>
                                <Icon name="server" size={16} /> Save session
                            </button>
                            <button type="button" className="suite-btn" disabled={!deviceId || dbBusy} onClick={loadFromDatabase} style={{ marginTop: '0.35rem' }}>
                                <Icon name="cloud" size={16} /> Load from database
                            </button>
                            <button type="button" className="suite-btn" disabled={!deviceId || dbBusy} onClick={() => exportExcel('media')} style={{ marginTop: '0.35rem' }}>
                                <Icon name="fileText" size={16} /> Export Excel
                            </button>
                            {dbStatus ? <p className="lv-hint">{dbStatus}</p> : null}
                        </div>
                    </aside>

                    <div className="lv-main rp-col-main">
                        <div className="lv-view-tabs">
                            <button
                                type="button"
                                className={`lv-view-tab ${tab === 'camera' ? 'lv-tab-active' : ''}`}
                                onClick={() => {
                                    setTab('camera');
                                    const first = captures.find((c) => c.kind === 'camera');
                                    if (first) session.setActiveId(first.id);
                                }}
                            >
                                Live capture
                                {cameraCount > 0 ? <span className="lv-tab-count">{cameraCount}</span> : null}
                            </button>
                            <button
                                type="button"
                                className={`lv-view-tab ${tab === 'audio' ? 'lv-tab-active' : ''}`}
                                onClick={() => {
                                    setTab('audio');
                                    const first = captures.find((c) => c.kind === 'audio');
                                    if (first) session.setActiveId(first.id);
                                }}
                            >
                                Audio
                                {audioCount > 0 ? <span className="lv-tab-count">{audioCount}</span> : null}
                            </button>
                        </div>

                        <div className="lv-viewer-bar">
                            <div className="lv-viewer-nav">
                                <button type="button" className="suite-btn lv-viewer-icon" disabled={!canPrev} onClick={goPrev} title="Previous (←)">
                                    <Icon name="chevronLeft" size={16} />
                                </button>
                                <span className="lv-viewer-position">
                                    {sliderItems.length ? `${activeIndex + 1} / ${sliderItems.length}` : '— / —'}
                                </span>
                                <button type="button" className="suite-btn lv-viewer-icon" disabled={!canNext} onClick={goNext} title="Next (→)">
                                    <Icon name="chevronRight" size={16} />
                                </button>
                            </div>
                            <div className="lv-viewer-tools">
                                <button type="button" className="suite-btn lv-viewer-icon" disabled={!active} onClick={() => setLightboxOpen(true)} title="Fullscreen (F)">
                                    <Icon name="maximize" size={16} />
                                </button>
                                <button type="button" className="suite-btn lv-viewer-icon" disabled={!active} onClick={downloadActive} title="Download">
                                    <Icon name="download" size={16} />
                                </button>
                                <button type="button" className="suite-btn lv-viewer-icon" disabled={!active} onClick={removeActive} title="Remove">
                                    <Icon name="trash" size={16} />
                                </button>
                            </div>
                        </div>

                        <CaptureStage
                            active={active}
                            recording={mic.recording}
                            elapsed={elapsed}
                            canPrev={canPrev}
                            canNext={canNext}
                            goPrev={goPrev}
                            goNext={goNext}
                            onOpenLightbox={() => setLightboxOpen(true)}
                            onTouchStart={onTouchStart}
                            onTouchEnd={onTouchEnd}
                            sliderItems={sliderItems}
                            selectCapture={selectCapture}
                        />

                        <CaptureStrip
                            captures={captures}
                            sliderItems={sliderItems}
                            active={active}
                            activeId={activeId}
                            timelineFilter={timelineFilter}
                            setTimelineFilter={setTimelineFilter}
                            selectCapture={selectCapture}
                            clearSession={handleClearSession}
                            stripRef={stripRef}
                        />
                    </div>

                    <aside className="lv-info rp-col">
                        <div className="lv-info-block">
                            <div className="suite-section-title">Active capture</div>
                            {!active ? (
                                <p className="suite-muted">No capture selected</p>
                            ) : (
                                <>
                                    <TruncatedText text={active.fileName} className="cm-list-name" lines={2} title={active.fileName} as="p" />
                                    <p className="suite-muted">{new Date(active.timestamp).toLocaleString()}</p>
                                    <p className="suite-muted">
                                        {active.kind === 'camera'
                                            ? `${active.camera === 'front' ? 'Front' : 'Back'} camera`
                                            : 'Microphone'}
                                        {active.size ? ` · ${formatBytes(active.size)}` : ''}
                                    </p>
                                    <div className="lv-info-capture-actions">
                                        <button type="button" className="suite-btn" disabled={!canPrev} onClick={goPrev}>
                                            <Icon name="chevronLeft" size={14} /> Prev
                                        </button>
                                        <button type="button" className="suite-btn" disabled={!canNext} onClick={goNext}>
                                            Next <Icon name="chevronRight" size={14} />
                                        </button>
                                    </div>
                                    <div className="lv-info-capture-actions">
                                        <button type="button" className="suite-btn" onClick={() => setLightboxOpen(true)}>
                                            <Icon name="maximize" size={14} /> Full
                                        </button>
                                        <button type="button" className="suite-btn" onClick={downloadActive}>
                                            <Icon name="download" size={14} /> Save
                                        </button>
                                    </div>
                                </>
                            )}
                        </div>
                        <div className="lv-info-block">
                            <div className="suite-section-title">Device status</div>
                            {!deviceInfo ? (
                                <p className="suite-muted">Select a device</p>
                            ) : (
                                <p className="suite-muted">
                                    Battery {deviceInfo.battery_level ?? '—'}% · Storage{' '}
                                    {formatStorage(deviceInfo.storage_available ?? deviceInfo.free_storage ?? deviceInfo.storage_free)}
                                </p>
                            )}
                        </div>
                        <div className="lv-info-block">
                            <div className="suite-section-title">Capture stats</div>
                            <div className="lv-stat-row">
                                <div className="lv-stat-cell">
                                    <strong>{cameraCount}</strong>
                                    <span>Photos</span>
                                </div>
                                <div className="lv-stat-cell">
                                    <strong>{audioCount}</strong>
                                    <span>Audio</span>
                                </div>
                            </div>
                            <p className="lv-browse-hint">← → browse · F fullscreen · swipe on photo</p>
                        </div>
                    </aside>
                </div>
            </div>

            {lightboxOpen && active ? (
                <CaptureLightbox
                    active={active}
                    activeIndex={activeIndex}
                    sliderItemsLength={sliderItems.length}
                    canPrev={canPrev}
                    canNext={canNext}
                    goPrev={goPrev}
                    goNext={goNext}
                    onClose={() => setLightboxOpen(false)}
                />
            ) : null}
        </div>
    );
}
