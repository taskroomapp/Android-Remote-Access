import React from 'react';
import Icon from '../ui/Icon';
import TruncatedText from '../hybrid/TruncatedText';
import { avatarClass } from '../../features/comms/nav';
import { normalizePhone, primaryPhone } from '../../features/comms/phones';

export default function CommsList({
    listTitle,
    listSearch,
    setListSearch,
    deviceId,
    bootstrapLoading,
    filtered,
    selectedMessage,
    selectedContact,
    activePhone,
    onSelectItem,
}) {
    return (
        <main className="cm-list-col rp-col-main">
            <header className="cm-list-header">
                <h4 className="cm-list-title">{listTitle}</h4>
                <div className="cm-search-wrap">
                    <Icon name="search" size={16} />
                    <input
                        type="search"
                        className="cm-search"
                        placeholder="Filter list…"
                        value={listSearch}
                        onChange={(e) => setListSearch(e.target.value)}
                    />
                </div>
            </header>
            <div className="cm-list-panel cm-list-scroll">
                {!deviceId ? (
                    <p className="cm-empty">Select a device and fetch contacts to view them here</p>
                ) : bootstrapLoading ? (
                    <div className="fb-loading">
                        <Icon name="spinner" size={24} className="spin" />
                    </div>
                ) : filtered.length === 0 ? (
                    <p className="cm-empty">No items — sync when online or reload saved</p>
                ) : (
                    <div className="cm-list">
                        {filtered.map((item, i) => {
                            const title = item.title || item.displayName || item.name || item.displayPhone || 'Unknown';
                            const subtitle = item.subtitle || item.displayPhone || '';
                            const preview = item.body || item.snippet || '';
                            const itemPhone = item.address || item.phone || primaryPhone(item);
                            const active =
                                selectedMessage?.id != null && item.id != null
                                    ? selectedMessage.id === item.id
                                    : (selectedContact?.id && selectedContact.id === item.id) ||
                                      (activePhone && normalizePhone(activePhone) === normalizePhone(itemPhone));
                            return (
                                <button
                                    key={item.id || `${itemPhone}-${i}`}
                                    type="button"
                                    className={`cm-list-row ${active ? 'cm-list-active' : ''}`}
                                    onClick={() => onSelectItem(item)}
                                >
                                    <div className={`cm-avatar cm-avatar-sm ${avatarClass(title)}`}>
                                        {String(title)[0]?.toUpperCase() || '?'}
                                    </div>
                                    <div className="cm-list-body">
                                        <div className="cm-list-top">
                                            <TruncatedText text={title} className="cm-list-name" title={title} />
                                            <div className="cm-list-top-right">
                                                {item.count > 1 ? (
                                                    <span className="cm-badge cm-badge-count">{item.count}</span>
                                                ) : null}
                                                {item.timeLabel ? (
                                                    <span className="cm-list-time">{item.timeLabel}</span>
                                                ) : null}
                                            </div>
                                        </div>
                                        {(subtitle || preview) && (
                                            <div className="cm-list-bottom">
                                                {subtitle ? (
                                                    <span className="cm-list-phone">{subtitle}</span>
                                                ) : null}
                                                {preview ? (
                                                    <span className="cm-list-preview">
                                                        {subtitle ? ' · ' : ''}
                                                        {preview.slice(0, 100)}
                                                    </span>
                                                ) : null}
                                            </div>
                                        )}
                                    </div>
                                </button>
                            );
                        })}
                    </div>
                )}
            </div>
        </main>
    );
}
