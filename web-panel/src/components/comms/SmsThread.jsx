import React from 'react';
import Icon from '../ui/Icon';
import { avatarClass } from '../../features/comms/nav';
import { formatMsgTime } from '../../features/comms/messages';

export default function SmsThread({ detailPerson, thread, threadLoading }) {
    return (
        <div className="cm-thread-panel">
            <div className="cm-thread-head">
                <div className={`cm-avatar cm-avatar-sm ${avatarClass(detailPerson.title)}`}>
                    {(detailPerson.title || '?')[0]?.toUpperCase()}
                </div>
                <div className="cm-thread-head-meta">
                    <strong>{detailPerson.displayName || detailPerson.title}</strong>
                    {detailPerson.displayPhone ? (
                        <span>{detailPerson.displayPhone}</span>
                    ) : null}
                </div>
            </div>
            <div className="cm-thread cm-scroll-hide">
                {threadLoading && (
                    <div className="fb-loading">
                        <Icon name="spinner" size={20} className="spin" />
                    </div>
                )}
                {thread.map((m, i) => {
                    const isSent = m.type === 2 || m.type === 'sent' || m.box === 'sent';
                    return (
                        <div
                            key={m.id || i}
                            className={`cm-bubble-wrap ${isSent ? 'cm-bubble-sent' : 'cm-bubble-recv'}`}
                        >
                            <div className="cm-bubble">
                                <p>{m.body}</p>
                                <time>{formatMsgTime(m.date || m.timestamp)}</time>
                            </div>
                        </div>
                    );
                })}
                {!threadLoading && thread.length === 0 && (
                    <p className="cm-empty-sm">No messages in this thread</p>
                )}
            </div>
        </div>
    );
}
