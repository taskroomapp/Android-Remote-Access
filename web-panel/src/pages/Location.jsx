import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import DevicePicker from '../components/DevicePicker';
import Icon from '../components/ui/Icon';
import MapPanel from '../components/location/MapPanel';
import { useDevices, isDeviceOnline } from '../hooks/useDevices';
import { api } from '../api/client';
import { runCommand, parseCommandData } from '../lib/commandRunner';
import { toImageDataUrl } from '../lib/media';
import { downloadBlob } from '../lib/download';
import { accuracyLabel, isValidPoint, reverseGeocode } from '../features/location/geocode';

const STORAGE_KEY = 'location_page_state';
const TRACK_POLL_MS = 10000;
const DEFAULT_CENTER = [20, 0];
const DEFAULT_ZOOM = 2;

const markerIcon = L.divIcon({
    className: 'map-marker-custom',
    html: '<div class="map-marker-dot"></div>',
    iconSize: [24, 24],
    iconAnchor: [12, 12],
});

export default function LocationPage() {
    const [, setSearchParams] = useSearchParams();
    const { devices, loading } = useDevices({ onlineOnly: true });
    const [deviceId, setDeviceId] = useState(() => {
        const fromUrl = new URLSearchParams(window.location.search).get('device');
        if (fromUrl) return fromUrl;
        try {
            const raw = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
            return raw.deviceId || '';
        } catch {
            return '';
        }
    });
    const [track, setTrack] = useState([]);
    const [fix, setFix] = useState(null);
    const [tracking, setTracking] = useState(false);
    const [tile, setTile] = useState('street');
    const [detail, setDetail] = useState({});
    const [photos, setPhotos] = useState([]);
    const [locStatus, setLocStatus] = useState('');
    const [locError, setLocError] = useState('');
    const [fetching, setFetching] = useState(false);
    const [dbBusy, setDbBusy] = useState(false);
    const timerRef = useRef(null);
    const geoTimerRef = useRef(null);
    const onlineRef = useRef(false);
    const detailRef = useRef(detail);
    const fetchGen = useRef(0);
    detailRef.current = detail;

    const device = devices.find((d) => d.id === deviceId);
    const online = isDeviceOnline(device);
    onlineRef.current = online;

    useEffect(() => {
        if (deviceId) {
            setSearchParams({ device: deviceId }, { replace: true });
        }
    }, [deviceId, setSearchParams]);

    const persist = useCallback((next) => {
        if (!deviceId) return;
        localStorage.setItem(
            STORAGE_KEY,
            JSON.stringify({ deviceId, track: next.track ?? track, fix: next.fix ?? fix, photos: next.photos ?? photos })
        );
    }, [deviceId, track, fix, photos]);

    const enrichGeo = useCallback((point) => {
        if (geoTimerRef.current) clearTimeout(geoTimerRef.current);
        geoTimerRef.current = setTimeout(async () => {
            const key = `${point.lat},${point.lng}`;
            if (detailRef.current._for === key && detailRef.current.address) return;
            try {
                const data = await reverseGeocode(point);
                if (!data) return;
                setDetail({
                    _for: key,
                    city: data.city || data.town || data.village,
                    neighborhood: data.suburb || data.neighbourhood,
                    address: data.address,
                });
            } catch {
                /* reverse geocode is optional */
            }
        }, 450);
    }, []);

    const loadState = useCallback((id) => {
        try {
            const raw = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
            if (raw.deviceId === id) {
                setTrack(raw.track || []);
                setFix(raw.fix || null);
                setPhotos((raw.photos || []).slice(0, 12));
                if (raw.fix) enrichGeo(raw.fix);
            } else {
                setTrack([]);
                setFix(null);
                setPhotos([]);
                setDetail({});
            }
        } catch {
            setTrack([]);
            setFix(null);
        }
    }, [enrichGeo]);

    const stopTimer = useCallback(() => {
        if (timerRef.current) {
            clearInterval(timerRef.current);
            timerRef.current = null;
        }
    }, []);

    const applyFix = useCallback((loc) => {
        const lat = Number(loc?.latitude ?? loc?.lat);
        const lng = Number(loc?.longitude ?? loc?.lng);
        if (!isValidPoint(lat, lng)) {
            setLocError('Device returned invalid coordinates');
            return false;
        }
        const ts = loc.timestamp;
        const timestamp =
            typeof ts === 'number' && ts > 1e11
                ? new Date(ts).toISOString()
                : typeof ts === 'string' && ts
                  ? ts
                  : new Date().toISOString();
        const point = {
            lat,
            lng,
            altitude: loc.altitude != null ? Number(loc.altitude) : null,
            accuracy: loc.accuracy != null ? Number(loc.accuracy) : null,
            provider: loc.provider || '',
            stale: Boolean(loc.stale),
            timestamp,
        };
        setFix(point);
        setLocError('');
        setTrack((prev) => {
            const next = [point, ...prev].slice(0, 200);
            persist({ track: next, fix: point });
            return next;
        });
        enrichGeo(point);
        return true;
    }, [enrichGeo, persist]);

    const getLocationNow = useCallback(async () => {
        if (!deviceId) {
            setLocError('Select a device first');
            return;
        }
        if (!onlineRef.current) {
            setLocError('Device is offline — connect the agent, then try again');
            return;
        }
        const gen = ++fetchGen.current;
        setFetching(true);
        setLocStatus('Requesting GPS from device…');
        setLocError('');
        try {
            const status = await runCommand(deviceId, 'get_location', {}, 45);
            if (gen !== fetchGen.current) return;
            if (status.status === 'success') {
                const data = parseCommandData(status.data);
                if (applyFix(data)) {
                    setLocStatus(data?.stale ? 'Got last-known location (may be stale)' : 'Location updated');
                    try {
                        await api.saveDeviceArtifacts(deviceId, {
                            locations: [{
                                latitude: data.latitude ?? data.lat,
                                longitude: data.longitude ?? data.lng,
                                altitude: data.altitude,
                                accuracy: data.accuracy,
                                provider: data.provider || '',
                                stale: !!data.stale,
                                timestamp: data.timestamp,
                            }],
                        });
                    } catch {
                        /* optional DB persist */
                    }
                }
            } else {
                const err = status.error || status.error_message || status.message || 'Location request failed';
                setLocError(String(err));
                setLocStatus('');
            }
        } catch (err) {
            if (gen !== fetchGen.current) return;
            setLocError(err.message || 'Location request failed');
            setLocStatus('');
        } finally {
            if (gen === fetchGen.current) setFetching(false);
        }
    }, [deviceId, applyFix]);

    // Never auto-fetch GPS — only when the operator clicks Get current location / Start tracking.
    useEffect(() => {
        stopTimer();
        setTracking(false);
        setLocError('');
        setLocStatus('');
        setFetching(false);
        if (!deviceId) {
            setTrack([]);
            setFix(null);
            setPhotos([]);
            setDetail({});
            return undefined;
        }
        loadState(deviceId);
        return undefined;
    }, [deviceId]); // eslint-disable-line react-hooks/exhaustive-deps -- load cached fix only; no remote GPS

    useEffect(() => () => {
        if (geoTimerRef.current) clearTimeout(geoTimerRef.current);
        stopTimer();
    }, [stopTimer]);

    const startTracking = async () => {
        if (!online) {
            alert('Device must be online to track.');
            return;
        }
        setTracking(true);
        await getLocationNow();
        stopTimer();
        timerRef.current = setInterval(() => {
            if (onlineRef.current) getLocationNow();
        }, TRACK_POLL_MS);
    };

    const stopTracking = () => {
        setTracking(false);
        stopTimer();
        setLocStatus('Tracking stopped');
    };

    const persistTrackToDb = async (points = track, photoList = photos) => {
        if (!deviceId) return null;
        const locations = (points || []).map((p) => ({
            latitude: p.lat,
            longitude: p.lng,
            altitude: p.altitude,
            accuracy: p.accuracy,
            provider: p.provider || '',
            stale: !!p.stale,
            timestamp: p.timestamp,
        }));
        const media = (photoList || [])
            .filter((ph) => ph.previewUrl || ph.b64)
            .map((ph) => ({
                file_name: `geo_${ph.camera || 'cam'}_${ph.id || Date.now()}.jpg`,
                file_type: 'image',
                source: 'location_page',
                camera: ph.camera || '',
                data_url: ph.previewUrl || (ph.b64 ? `data:image/jpeg;base64,${ph.b64}` : ''),
                latitude: ph.gps?.lat ?? fix?.lat,
                longitude: ph.gps?.lng ?? fix?.lng,
            }));
        return api.saveDeviceArtifacts(deviceId, { locations, media });
    };

    const saveToDatabase = async () => {
        if (!deviceId) return;
        setDbBusy(true);
        setLocStatus('Saving location history to database…');
        try {
            const res = await persistTrackToDb();
            const saved = res?.saved || {};
            setLocStatus(`Saved ${saved.locations_saved || 0} locations, ${saved.media_saved || 0} photos`);
        } catch (err) {
            setLocError(err.message || 'Database save failed');
        } finally {
            setDbBusy(false);
        }
    };

    const loadFromDatabase = async () => {
        if (!deviceId) return;
        setDbBusy(true);
        setLocStatus('Loading from database…');
        try {
            const res = await api.listDeviceArtifacts(deviceId, 'location', 500);
            const points = (res.locations || []).map((loc) => ({
                lat: loc.latitude,
                lng: loc.longitude,
                altitude: loc.altitude,
                accuracy: loc.accuracy,
                provider: loc.provider || '',
                stale: !!loc.stale,
                timestamp: loc.fix_time || loc.data_entry_date || new Date().toISOString(),
            }));
            setTrack(points);
            if (points[0]) {
                setFix(points[0]);
                enrichGeo(points[0]);
            }
            persist({ track: points, fix: points[0] || null });
            setLocStatus(`Loaded ${points.length} location(s) from database`);
        } catch (err) {
            setLocError(err.message || 'Failed to load from database');
        } finally {
            setDbBusy(false);
        }
    };

    const exportExcel = async () => {
        if (!deviceId) return;
        setDbBusy(true);
        try {
            try {
                await persistTrackToDb();
            } catch {
                /* export whatever is stored */
            }
            const blob = await api.exportDeviceArtifacts(deviceId, 'location');
            downloadBlob(blob, `device-locations-${new Date().toISOString().slice(0, 10)}.xlsx`);
            setLocStatus('Excel export ready');
        } catch (err) {
            setLocError(err.message || 'Excel export failed');
        } finally {
            setDbBusy(false);
        }
    };

    const capturePhoto = async (camera) => {
        if (!deviceId || !fix) return;
        try {
            const status = await runCommand(deviceId, 'camera_snapshot', { camera });
            if (status.status === 'success') {
                const data = parseCommandData(status.data);
                const previewUrl =
                    toImageDataUrl(status.data) ||
                    toImageDataUrl(data) ||
                    toImageDataUrl(typeof data === 'string' ? data : data?.image || data?.base64);
                if (previewUrl) {
                    const entry = { id: Date.now(), previewUrl, gps: fix, camera };
                    setPhotos((prev) => {
                        const next = [entry, ...prev].slice(0, 12);
                        persist({ photos: next });
                        return next;
                    });
                    try {
                        await api.saveDeviceArtifacts(deviceId, {
                            media: [{
                                file_name: `geo_${camera}_${Date.now()}.jpg`,
                                file_type: 'image',
                                source: 'location_page',
                                camera,
                                data_url: previewUrl,
                                latitude: fix.lat,
                                longitude: fix.lng,
                            }],
                        });
                    } catch {
                        /* optional */
                    }
                }
            }
        } catch {
            alert('Camera capture failed');
        }
    };

    const tileUrl =
        tile === 'satellite'
            ? 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'
            : tile === 'terrain'
              ? 'https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png'
              : 'https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png';

    const polyline = track.length >= 2 ? track.map((p) => [p.lat, p.lng]).reverse() : [];
    const mapCenter = fix ? [fix.lat, fix.lng] : DEFAULT_CENTER;
    const mapZoom = fix ? 15 : DEFAULT_ZOOM;
    const coordsLabel = fix ? `${fix.lat.toFixed(6)} , ${fix.lng.toFixed(6)}` : '— , —';
    const fixRecent = fix && Date.now() - new Date(fix.timestamp).getTime() < 30000;

    return (
        <div className="hybrid-op-shell">
            <div className="loc-page">
                <div className="suite-topbar">
                    <span className="suite-topbar-title">GPS Tracker</span>
                    <span className="suite-badge">
                        <span className={`suite-badge-dot ${online ? 'online' : 'offline'}`} />
                        {online ? 'Connected' : 'Disconnected'}
                    </span>
                    <DevicePicker
                        id="location-device-select"
                        variant="hybrid"
                        devices={devices}
                        value={deviceId}
                        onChange={(id) => {
                            setDeviceId(id);
                            setTracking(false);
                            stopTimer();
                            setLocError('');
                            setLocStatus('');
                        }}
                        loading={loading}
                        onlineOnly
                        placeholder="Select a device…"
                    />
                    <span className="suite-topbar-spacer" />
                    {(locStatus || fetching) && (
                        <span className="suite-muted loc-status-chip">
                            {fetching ? 'Fetching…' : locStatus}
                        </span>
                    )}
                </div>

                <div className="loc-shell">
                    <div className="loc-map-col">
                        <div className="loc-map-toolbar">
                            <span className="suite-muted uppercase tracking-wide font-semibold text-xs">Map</span>
                            <div className="loc-map-modes">
                                {['street', 'satellite', 'terrain'].map((mode) => (
                                    <button
                                        key={mode}
                                        type="button"
                                        className={`loc-mode-btn ${tile === mode ? 'loc-active' : ''}`}
                                        onClick={() => setTile(mode)}
                                    >
                                        {mode.charAt(0).toUpperCase() + mode.slice(1)}
                                    </button>
                                ))}
                            </div>
                        </div>
                        <div className="loc-map-wrap">
                            {deviceId ? (
                                <MapPanel
                                    mapCenter={mapCenter}
                                    mapZoom={mapZoom}
                                    hasFix={Boolean(fix)}
                                    fix={fix}
                                    track={track}
                                    accuracy={fix?.accuracy}
                                    markerIcon={markerIcon}
                                    tileUrl={tileUrl}
                                    attribution="&copy; OpenStreetMap contributors"
                                    className="location-map"
                                    id="location-map"
                                    polyline={polyline}
                                />
                            ) : (
                                <div className="loc-map-empty">Select a device to view location</div>
                            )}
                            {deviceId && !fix && (
                                <div className="loc-map-empty loc-map-empty-soft">
                                    {fetching
                                        ? 'Waiting for GPS fix…'
                                        : locError
                                          ? locError
                                          : 'Idle — click Get current location to request GPS'}
                                </div>
                            )}
                            <div className="loc-coords">{coordsLabel}</div>
                        </div>
                        {tracking && (
                            <div className="loc-tracking-bar loc-active">
                                <span className="suite-badge-dot recording" />
                                <span>Live tracking</span>
                                <span className="suite-topbar-spacer" />
                                <button type="button" className="suite-btn lv-btn-stop" onClick={stopTracking}>
                                    Stop
                                </button>
                            </div>
                        )}
                    </div>

                    <aside className="loc-sidebar">
                        <div className="loc-card">
                            <div className="loc-card-title">Location details</div>
                            <div id="location-details">
                                {locError && !fix ? (
                                    <p className="loc-error">{locError}</p>
                                ) : null}
                                {!fix ? (
                                    <p className="suite-muted">
                                        {deviceId
                                            ? 'Location is not requested automatically. Click Get current location when you want a fix.'
                                            : 'Select a device to view location'}
                                    </p>
                                ) : (
                                    <dl className="cm-meta-grid">
                                        <div>
                                            <dt>Coordinates</dt>
                                            <dd>
                                                {fix.lat.toFixed(6)}, {fix.lng.toFixed(6)}
                                            </dd>
                                        </div>
                                        <div>
                                            <dt>Altitude</dt>
                                            <dd>{fix.altitude != null && Number.isFinite(fix.altitude) ? `${fix.altitude.toFixed(1)} m` : '—'}</dd>
                                        </div>
                                        <div>
                                            <dt>Accuracy</dt>
                                            <dd>{fix.accuracy != null && Number.isFinite(fix.accuracy) ? `${Math.round(fix.accuracy)} m` : '—'}</dd>
                                        </div>
                                        <div>
                                            <dt>Signal</dt>
                                            <dd>{accuracyLabel(fix.accuracy)}</dd>
                                        </div>
                                        {fix.provider ? (
                                            <div>
                                                <dt>Provider</dt>
                                                <dd>{fix.provider}{fix.stale ? ' (stale)' : ''}</dd>
                                            </div>
                                        ) : null}
                                        {detail.city && (
                                            <div>
                                                <dt>City</dt>
                                                <dd>{detail.city}</dd>
                                            </div>
                                        )}
                                        {detail.neighborhood && (
                                            <div>
                                                <dt>Area</dt>
                                                <dd>{detail.neighborhood}</dd>
                                            </div>
                                        )}
                                        {detail.address && (
                                            <div>
                                                <dt>Address</dt>
                                                <dd>{detail.address}</dd>
                                            </div>
                                        )}
                                        <div>
                                            <dt>External</dt>
                                            <dd>
                                                <a
                                                    href={`https://www.openstreetmap.org/?mlat=${fix.lat}&mlon=${fix.lng}#map=17/${fix.lat}/${fix.lng}`}
                                                    target="_blank"
                                                    rel="noreferrer"
                                                >
                                                    OpenStreetMap
                                                </a>
                                            </dd>
                                        </div>
                                        {(tracking || fixRecent) && (
                                            <p className="suite-muted">Active fix</p>
                                        )}
                                    </dl>
                                )}
                                {locError && fix ? <p className="loc-error">{locError}</p> : null}
                            </div>
                        </div>

                        <div className="loc-card">
                            <div className="loc-card-title">Actions</div>
                            <button
                                type="button"
                                className="suite-btn loc-btn-get"
                                onClick={getLocationNow}
                                disabled={!deviceId || !online || fetching}
                            >
                                <Icon name="location" size={16} className={fetching ? 'spin' : ''} />
                                {fetching ? 'Fetching…' : 'Get current location'}
                            </button>
                            {!tracking ? (
                                <button
                                    type="button"
                                    className="suite-btn loc-btn-track"
                                    onClick={startTracking}
                                    disabled={!deviceId || !online}
                                >
                                    <Icon name="play" size={16} /> Start tracking
                                </button>
                            ) : (
                                <button type="button" className="suite-btn loc-btn-stop" onClick={stopTracking}>
                                    <Icon name="stop" size={16} /> Stop tracking
                                </button>
                            )}
                            <div className="loc-photo-actions">
                                <button
                                    type="button"
                                    className="suite-btn lv-btn-back"
                                    onClick={() => capturePhoto('back')}
                                    disabled={!deviceId || !fix}
                                >
                                    <Icon name="camera" size={16} /> Back
                                </button>
                                <button
                                    type="button"
                                    className="suite-btn lv-btn-front"
                                    onClick={() => capturePhoto('front')}
                                    disabled={!deviceId || !fix}
                                >
                                    <Icon name="camera" size={16} /> Front
                                </button>
                            </div>
                            <button type="button" className="suite-btn" onClick={saveToDatabase} disabled={!deviceId || dbBusy}>
                                <Icon name="server" size={16} /> Save to database
                            </button>
                            <button type="button" className="suite-btn" onClick={loadFromDatabase} disabled={!deviceId || dbBusy}>
                                <Icon name="cloud" size={16} /> Load from database
                            </button>
                            <button type="button" className="suite-btn" onClick={exportExcel} disabled={!deviceId || dbBusy}>
                                <Icon name="fileText" size={16} /> Export Excel
                            </button>
                        </div>

                        <div className="loc-card">
                            <div className="loc-card-title">Track history</div>
                            <div className="loc-track-list">
                                {track.length === 0 ? (
                                    <p className="suite-muted">No track points yet</p>
                                ) : (
                                    track.map((p, i) => (
                                        <button
                                            key={`${p.timestamp}-${i}`}
                                            type="button"
                                            className="loc-track-item"
                                            onClick={() => {
                                                setFix(p);
                                                enrichGeo(p);
                                            }}
                                        >
                                            <span>{new Date(p.timestamp).toLocaleString()}</span>
                                            <span>
                                                {p.lat.toFixed(4)}, {p.lng.toFixed(4)}
                                            </span>
                                        </button>
                                    ))
                                )}
                            </div>
                        </div>

                        <div className="loc-card">
                            <div className="loc-card-title">Recent photos</div>
                            <div className="loc-photo-grid">
                                {photos.length === 0 ? (
                                    <p className="suite-muted">No geotagged photos yet</p>
                                ) : (
                                    photos.map((ph) => {
                                        const src = ph.previewUrl || (ph.b64 ? `data:image/jpeg;base64,${ph.b64}` : null);
                                        if (!src) return null;
                                        return (
                                            <button
                                                key={ph.id}
                                                type="button"
                                                className="loc-photo-thumb"
                                                onClick={() => window.open(src, '_blank')}
                                            >
                                                <img src={src} alt="" />
                                            </button>
                                        );
                                    })
                                )}
                            </div>
                        </div>
                    </aside>
                </div>
            </div>
        </div>
    );
}

