import React from 'react';
import Icon from '../ui/Icon';
import { formatSessionClock } from '../../features/live/formatters';

export default function CaptureStage({
    active,
    recording,
    elapsed,
    canPrev,
    canNext,
    goPrev,
    goNext,
    onOpenLightbox,
    onTouchStart,
    onTouchEnd,
    sliderItems,
    selectCapture,
}) {
    return (
        <div
            className="lv-viewport"
            onTouchStart={onTouchStart}
            onTouchEnd={onTouchEnd}
        >
            <button
                type="button"
                className="lv-slide-arrow lv-slide-prev"
                disabled={!canPrev}
                onClick={goPrev}
                aria-label="Previous capture"
            >
                <Icon name="chevronLeft" size={22} />
            </button>
            <button
                type="button"
                className="lv-slide-arrow lv-slide-next"
                disabled={!canNext}
                onClick={goNext}
                aria-label="Next capture"
            >
                <Icon name="chevronRight" size={22} />
            </button>

            <div className="lv-viewport-stage">
                <div className="lv-viewport-inner">
                    {!active ? (
                        <p className="lv-empty">Capture a photo or recording to preview here</p>
                    ) : active.kind === 'camera' ? (
                        <button
                            type="button"
                            className="lv-media-open"
                            onClick={onOpenLightbox}
                            title="Open fullscreen"
                        >
                            <img src={active.previewUrl} alt={active.fileName} className="lv-media-img" />
                        </button>
                    ) : (
                        <div className="lv-audio-stage">
                            <div className="lv-audio-glyph">
                                <Icon name="mic" size={40} />
                            </div>
                            <audio controls className="lv-audio-player" src={active.previewUrl} />
                            <p className="lv-audio-name">{active.fileName}</p>
                        </div>
                    )}
                </div>
            </div>

            <div className="lv-viewport-overlay">
                <div className={`lv-rec-badge ${recording ? 'lv-active' : ''}`}>
                    <span className="suite-badge-dot recording" /> REC
                </div>
                {active ? (
                    <div className="lv-meta-overlay">
                        <strong>{active.kind === 'camera' ? (active.camera === 'front' ? 'Front' : 'Back') : 'Audio'}</strong>
                        {' · '}
                        {new Date(active.timestamp).toLocaleTimeString()}
                    </div>
                ) : null}
                <div className="lv-viewport-clock">{formatSessionClock(elapsed)}</div>
            </div>

            {sliderItems.length > 1 ? (
                <div className="lv-slide-dots" aria-hidden="true">
                    {sliderItems.slice(0, 24).map((c) => (
                        <button
                            key={c.id}
                            type="button"
                            className={`lv-slide-dot ${c.id === active?.id ? 'lv-dot-active' : ''}`}
                            onClick={() => selectCapture(c)}
                        />
                    ))}
                    {sliderItems.length > 24 ? <span className="lv-dot-more">+{sliderItems.length - 24}</span> : null}
                </div>
            ) : null}
        </div>
    );
}
