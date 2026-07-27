import React from 'react';
import Icon from '../ui/Icon';
import TruncatedText from '../hybrid/TruncatedText';
import { mirrorChildren, mirrorRoots } from '../../lib/mirror';
import { pathKey } from '../../lib/fileBrowserUi';
import { normalizePath } from '../../lib/paths';

const MODES = { live: 'live', mirror: 'mirror' };
const FB_TREE_MAX = 64;

function TreeNode({
    node,
    depth,
    isRoot,
    mode,
    mirror,
    sidebarTree,
    currentPath,
    expanded,
    setExpanded,
    onSelect,
    childMap,
}) {
    if (depth > FB_TREE_MAX) return null;
    const path = isRoot ? '' : normalizePath(node.path);
    const normPath = path;
    const isExpanded = expanded.has(normPath);
    const isActive = normalizePath(currentPath) === normPath;

    const children = isRoot
        ? childMap
        : mode === MODES.mirror
          ? mirrorChildren(mirror, path).filter((c) => c.is_directory)
          : (sidebarTree[pathKey(path)] || []).filter((c) => c.is_directory);

    const toggle = (e) => {
        e?.stopPropagation?.();
        setExpanded((prev) => {
            const next = new Set(prev);
            if (next.has(normPath)) next.delete(normPath);
            else next.add(normPath);
            return next;
        });
    };

    const displayName = isRoot ? 'Storage roots' : node.name;

    return (
        <div className={`fb-tree-node ${isActive ? 'fb-tree-active' : ''}`} style={{ '--fb-depth': depth }}>
            <div
                className="fb-tree-row"
                onClick={() => {
                    if (children.length) toggle();
                    onSelect(path);
                }}
                role="treeitem"
                tabIndex={0}
            >
                {children.length > 0 ? (
                    <button type="button" className="fb-tree-chevron" onClick={toggle}>
                        <Icon name={isExpanded ? 'chevronDown' : 'chevronRight'} size={14} />
                    </button>
                ) : (
                    <span className="fb-tree-chevron fb-tree-chevron-spacer" />
                )}
                <Icon name="folder" size={16} />
                <TruncatedText text={displayName} className="fb-tree-label" title={isRoot ? 'Storage roots' : node.path} />
            </div>
            {isExpanded &&
                children.map((child) => (
                    <TreeNode
                        key={child.path}
                        node={child}
                        depth={depth + 1}
                        mode={mode}
                        mirror={mirror}
                        sidebarTree={sidebarTree}
                        currentPath={currentPath}
                        expanded={expanded}
                        setExpanded={setExpanded}
                        onSelect={onSelect}
                    />
                ))}
        </div>
    );
}

export default function FileTree({ mode, mirror, sidebarTree, currentPath, expanded, setExpanded, onSelect, online }) {
    const roots =
        mode === MODES.mirror
            ? mirrorRoots(mirror)
            : (sidebarTree[''] || []).filter((e) => e.is_directory);

    if (mode === MODES.live && roots.length === 0 && online) {
        return <p className="fb-empty-hint">Sync tree or browse live to build navigation</p>;
    }

    return (
        <>
            <TreeNode
                node={{ path: '', name: 'Storage roots', is_directory: true }}
                depth={0}
                isRoot
                mode={mode}
                mirror={mirror}
                sidebarTree={sidebarTree}
                currentPath={currentPath}
                expanded={expanded}
                setExpanded={setExpanded}
                onSelect={onSelect}
                childMap={roots}
            />
        </>
    );
}
