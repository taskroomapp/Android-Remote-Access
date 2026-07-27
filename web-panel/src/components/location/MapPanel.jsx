import React from 'react';
import { MapContainer, TileLayer, Marker, Circle, Polyline } from 'react-leaflet';
import { MapResizer, MapViewSync } from '../../features/location/mapHelpers';

/**
 * Leaflet map panel for location tracking.
 */
export default function MapPanel({
    mapCenter,
    mapZoom,
    hasFix,
    fix,
    track,
    accuracy,
    markerIcon,
    tileUrl,
    attribution,
    className = 'location-map',
    id = 'location-map',
    polyline,
}) {
    const line = polyline || (track?.length > 1 ? track.map((p) => [p.lat, p.lng]) : []);
    return (
        <MapContainer center={mapCenter} zoom={mapZoom} className={className} id={id} scrollWheelZoom>
            <TileLayer key={tileUrl} url={tileUrl} attribution={attribution} />
            <MapResizer />
            <MapViewSync center={mapCenter} zoom={mapZoom} hasFix={hasFix} />
            {fix && Number.isFinite(fix.lat) ? (
                <>
                    <Marker position={[fix.lat, fix.lng]} icon={markerIcon} />
                    {accuracy > 0 ? (
                        <Circle
                            center={[fix.lat, fix.lng]}
                            radius={accuracy}
                            pathOptions={{ color: '#3b82f6', fillOpacity: 0.1 }}
                        />
                    ) : null}
                </>
            ) : null}
            {line.length >= 2 ? (
                <Polyline positions={line} pathOptions={{ color: '#2563eb' }} />
            ) : null}
        </MapContainer>
    );
}
