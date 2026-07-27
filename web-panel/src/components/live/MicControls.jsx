import React from 'react';
import Icon from '../ui/Icon';

export default function MicControls({ deviceId, busy, recording, onStart, onStop }) {
    return (
        <div className="lv-section">
            <div className="suite-section-title">Audio recording</div>
            <div className="lv-capture-grid">
                <button type="button" className="suite-btn lv-btn-record" disabled={!deviceId || busy || recording} onClick={onStart}>
                    <Icon name="mic" size={16} /> Record
                </button>
                <button type="button" className="suite-btn lv-btn-stop" disabled={!recording || busy} onClick={onStop}>
                    <Icon name="stop" size={16} /> Stop
                </button>
            </div>
            <div className="lv-hint">
                {recording ? (
                    <span className="lv-recording-label">
                        <span className="suite-badge-dot recording" /> Recording…
                    </span>
                ) : (
                    'Requires microphone permission'
                )}
            </div>
        </div>
    );
}
