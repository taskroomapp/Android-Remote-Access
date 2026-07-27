export function isValidPoint(lat, lng) {
    return Number.isFinite(lat) && Number.isFinite(lng) && Math.abs(lat) <= 90 && Math.abs(lng) <= 180;
}

export function accuracyLabel(meters) {
    if (meters == null || !Number.isFinite(meters)) return 'Unknown';
    if (meters <= 10) return 'Excellent';
    if (meters <= 30) return 'Good';
    if (meters <= 100) return 'Fair';
    return 'Poor';
}

/** Reverse-geocode via Nominatim; returns address detail object or null. */
export async function reverseGeocode(point) {
    const url = `https://nominatim.openstreetmap.org/reverse?lat=${point.lat}&lon=${point.lng}&format=json`;
    const res = await fetch(url, {
        headers: {
            Accept: 'application/json',
            'Accept-Language': navigator.language || 'en',
            'User-Agent': 'AndroidRemoteAccessPanel/1.0 (operator console)',
        },
    });
    if (!res.ok) return null;
    const data = await res.json();
    return {
        _for: `${point.lat},${point.lng}`,
        address: data.display_name || '',
        ...data.address,
    };
}
