/**
 * Shared operator controls — use these inside hybrid-op-shell pages so
 * interaction and color stay consistent (Chat Hybrid HCI + teal accent).
 */
import Icon from '../ui/Icon';

export function HybridPrimaryButton({ children, icon, loading, className = '', ...rest }) {
    return (
        <button type="button" className={`fb-btn fb-btn-primary ${className}`.trim()} disabled={loading || rest.disabled} {...rest}>
            {loading ? <Icon name="spinner" size={16} className="spin" /> : icon ? <Icon name={icon} size={16} /> : null}
            {children}
        </button>
    );
}

export function HybridSecondaryButton({ children, icon, className = '', ...rest }) {
    return (
        <button type="button" className={`fb-btn-sm ${className}`.trim()} {...rest}>
            {icon ? <Icon name={icon} size={16} /> : null}
            {children}
        </button>
    );
}

export function HybridAccentButton({ children, icon, className = '', ...rest }) {
    return (
        <button type="button" className={`fb-btn-sm fb-btn-accent ${className}`.trim()} {...rest}>
            {icon ? <Icon name={icon} size={16} /> : null}
            {children}
        </button>
    );
}

export function HybridEmptyState({ icon = 'folder', title, hint }) {
    return (
        <div className="fb-empty-state" role="status">
            <Icon name={icon} size={40} />
            <p>{title}</p>
            {hint ? <p className="suite-muted">{hint}</p> : null}
        </div>
    );
}

export function HybridLoadingState({ label = 'Loading…' }) {
    return (
        <div className="fb-loading" role="status" aria-live="polite">
            <Icon name="spinner" size={28} className="spin" aria-hidden />
            {label}
        </div>
    );
}
