import React from 'react';
import Icon from '../ui/Icon';
import TruncatedText from '../hybrid/TruncatedText';
import { formatShortTime } from '../../features/live/formatters';

export default function CaptureStrip({
    captures,
    sliderItems,
    active,
    activeId,
    timelineFilter,
    setTimelineFilter,
    selectCapture,
    clearSession,
    stripRef,
}) {
    return (
        <div className="lv-timeline">
            <div className="lv-timeline-head">
                <span className="lv-timeline-summary">
                    Session · <strong>{captures.length}</strong> total
                    {sliderItems.length !== captures.length ? (
                        <> · showing <strong>{sliderItems.length}</strong></>
                    ) : null}
                </span>
                <div className="lv-timeline-tools">
                    <select
                        className="suite-select lv-timeline-filter"
                        value={timelineFilter}
                        onChange={(e) => {
                            const v = e.target.value;
                            setTimelineFilter(v);
                            const pool =
                                v === 'photo'
                                    ? captures.filter((c) => c.kind === 'camera')
                                    : v === 'audio'
                                      ? captures.filter((c) => c.kind === 'audio')
                                      : captures;
                            if (pool.length && !pool.some((c) => c.id === activeId)) {
                                selectCapture(pool[0]);
                            }
                        }}
                    >
                        <option value="all">All types</option>
                        <option value="photo">Photos</option>
                        <option value="audio">Audio</option>
                    </select>
                    <button type="button" className="suite-btn" onClick={clearSession} disabled={!captures.length}>
                        Clear all
                    </button>
                </div>
            </div>
            <div className="lv-timeline-list lv-filmstrip" ref={stripRef}>
                {sliderItems.length === 0 ? (
                    <p className="lv-filmstrip-empty">No captures in this filter</p>
                ) : (
                    sliderItems.map((c, i) => (
                        <button
                            key={c.id}
                            type="button"
                            data-lv-index={i}
                            className={`lv-timeline-item ${c.id === active?.id ? 'lv-timeline-active' : ''}`}
                            onClick={() => selectCapture(c)}
                        >
                            <div className="lv-timeline-thumb">
                                {c.kind === 'camera' ? (
                                    <img src={c.previewUrl} alt="" />
                                ) : (
                                    <Icon name="mic" size={24} />
                                )}
                                <span className={`lv-thumb-badge ${c.kind === 'camera' ? 'lv-thumb-photo' : 'lv-thumb-audio'}`}>
                                    {c.kind === 'camera' ? (c.camera === 'front' ? 'F' : 'B') : 'A'}
                                </span>
                            </div>
                            <div className="lv-timeline-meta">
                                <TruncatedText text={c.fileName} lines={1} title={c.fileName} />
                                <span className="lv-timeline-time">{formatShortTime(c.timestamp)}</span>
                            </div>
                        </button>
                    ))
                )}
            </div>
        </div>
    );
}
