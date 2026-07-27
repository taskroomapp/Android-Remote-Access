import React from 'react';
import Icon from '../ui/Icon';
import TruncatedText from '../hybrid/TruncatedText';
import { avatarClass } from '../../features/comms/nav';
import { formatMsgTime } from '../../features/comms/messages';
import { formatPhoneDisplay, normalizePhone } from '../../features/comms/phones';

export default function ContactDetail({
    detailPerson,
    activePhone,
    selectedMessage,
    openThread,
}) {
    return (
        <>
            <div className="cm-details-hero">
                <div className={`cm-avatar cm-avatar-lg ${avatarClass(detailPerson.title)}`}>
                    {(detailPerson.title || '?')[0]?.toUpperCase()}
                </div>
                <div className="cm-details-name">
                    <TruncatedText
                        text={detailPerson.displayName || detailPerson.title}
                        lines={2}
                        title={detailPerson.displayName || detailPerson.title}
                    />
                </div>
                {detailPerson.displayPhone ? (
                    <div className="cm-details-sub">{detailPerson.displayPhone}</div>
                ) : null}
                {detailPerson.displayName && activePhone && normalizePhone(activePhone) !== normalizePhone(detailPerson.displayPhone) ? (
                    <div className="cm-details-sub">{formatPhoneDisplay(activePhone)}</div>
                ) : null}
            </div>
            <dl className="cm-meta-grid">
                {(detailPerson.phones || []).map((p, i) => {
                    const num = p.number || p;
                    return (
                        <div key={i}>
                            <dt>{p.type || 'Phone'}</dt>
                            <dd className="cm-meta-phone-row">
                                <span>{formatPhoneDisplay(num)}</span>
                                <button
                                    type="button"
                                    className="cm-action-btn"
                                    onClick={() => openThread(num, { contact: detailPerson, tab: 'messages' })}
                                >
                                    View SMS
                                </button>
                            </dd>
                        </div>
                    );
                })}
                {!(detailPerson.phones || []).length && activePhone ? (
                    <div>
                        <dt>Phone</dt>
                        <dd className="cm-meta-phone-row">
                            <span>{formatPhoneDisplay(activePhone)}</span>
                            <button
                                type="button"
                                className="cm-action-btn"
                                onClick={() => openThread(activePhone, { contact: detailPerson, tab: 'messages' })}
                            >
                                View SMS
                            </button>
                        </dd>
                    </div>
                ) : null}
                {(detailPerson.emails || []).map((e, i) => (
                    <div key={`e-${i}`}>
                        <dt>Email</dt>
                        <dd>{e.address || e}</dd>
                    </div>
                ))}
            </dl>
            {selectedMessage?.body ? (
                <div className="cm-selected-msg">
                    <h5 className="cm-section-title">Selected message</h5>
                    <div className="cm-bubble cm-bubble-preview">
                        <p>{selectedMessage.body}</p>
                        <time>{formatMsgTime(selectedMessage.date || selectedMessage.timestamp)}</time>
                    </div>
                </div>
            ) : null}
            {activePhone ? (
                <div className="cm-action-grid">
                    <button
                        type="button"
                        className="cm-action-btn cm-action-primary"
                        onClick={() => openThread(activePhone, {
                            contact: detailPerson,
                            message: selectedMessage,
                            tab: 'messages',
                        })}
                    >
                        <Icon name="message" size={14} /> Open conversation
                    </button>
                </div>
            ) : null}
        </>
    );
}
