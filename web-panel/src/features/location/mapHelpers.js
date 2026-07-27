import React, { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import { isValidPoint } from '../../features/location/geocode';

export function MapResizer() {
    const map = useMap();
    useEffect(() => {
        const t = setTimeout(() => map.invalidateSize(), 100);
        const t2 = setTimeout(() => map.invalidateSize(), 400);
        const onResize = () => map.invalidateSize();
        window.addEventListener('resize', onResize);
        return () => {
            clearTimeout(t);
            clearTimeout(t2);
            window.removeEventListener('resize', onResize);
        };
    }, [map]);
    return null;
}

export function MapViewSync({ center, zoom, hasFix }) {
    const map = useMap();
    const prev = useRef(null);
    useEffect(() => {
        if (!center || !isValidPoint(center[0], center[1])) return;
        const key = `${center[0]},${center[1]},${zoom},${hasFix}`;
        if (prev.current === key) return;
        prev.current = key;
        if (hasFix) {
            map.flyTo(center, zoom, { duration: 0.6 });
        } else {
            map.setView(center, zoom);
        }
        map.invalidateSize();
    }, [center, zoom, hasFix, map]);
    return null;
}
