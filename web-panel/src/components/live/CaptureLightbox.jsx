import React from 'react';
import Icon from '../ui/Icon';

export default function CaptureLightbox({
    active,
    activeIndex,
    sliderItemsLength,
    canPrev,
    canNext,
    goPrev,
    goNext,
    onClose,
}) {
    if (!active) return null;
    return (
        <div className="lv-lightbox" role="dialog" aria-modal="true">
            <button type="button" className="lv-lightbox-backdrop" aria-label="Close" onClick={onClose} />
            <button type="button" className="suite-btn lv-lightbox-close" onClick={onClose}>
                <Icon name="close" size={18} />
            </button>
            <button
                type="button"
                className="lv-slide-arrow lv-slide-prev lv-lightbox-arrow"
                disabled={!canPrev}
                onClick={goPrev}
                aria-label="Previous"
            >
                <Icon name="chevronLeft" size={24} />
            </button>
            <button
                type="button"
                className="lv-slide-arrow lv-slide-next lv-lightbox-arrow"
                disabled={!canNext}
                onClick={goNext}
                aria-label="Next"
            >
                <Icon name="chevronRight" size={24} />
            </button>
            <div className="lv-lightbox-content">
                {active.kind === 'camera' ? (
                    <img src={active.previewUrl} alt={active.fileName} className="lv-lightbox-img" />
                ) : (
                    <audio controls className="lv-lightbox-audio" src={active.previewUrl} autoPlay />
                )}
            </div>
            <div className="lv-lightbox-meta">
                {activeIndex + 1} / {sliderItemsLength} · {active.fileName}
            </div>
        </div>
    );
}
