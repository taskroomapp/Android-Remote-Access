import React from 'react';

/**
 * Accessible truncation: visual ellipsis + native tooltip with full string (HCI).
 */
export default function TruncatedText({
    text = '',
    className = '',
    lines = 1,
    as: Tag = 'span',
    title,
    ...rest
}) {
    const full = text == null ? '' : String(text);
    const tip = title ?? (full.length > 0 ? full : undefined);
    const lineClass = lines > 1 ? 'hy-truncate-2' : 'hy-truncate';

    return (
        <Tag className={`${lineClass} ${className}`.trim()} title={tip} {...rest}>
            {full}
        </Tag>
    );
}
