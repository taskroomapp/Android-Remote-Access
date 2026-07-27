import React from 'react';
import Icon from '../ui/Icon';

export default function CameraControls({ deviceId, busy, onCapture }) {
    return (
        <div className="lv-section">
            <div className="suite-section-title">Camera capture</div>
            <div className="lv-capture-grid">
                <button type="button" className="suite-btn lv-btn-back" disabled={!deviceId || busy} onClick={() => onCapture('back')}>
                    <Icon name="camera" size={16} /> Back
                </button>
                <button type="button" className="suite-btn lv-btn-front" disabled={!deviceId || busy} onClick={() => onCapture('front')}>
                    <Icon name="camera" size={16} /> Front
                </button>
            </div>
            <p className="lv-hint">Click to capture a photo</p>
        </div>
    );
}
