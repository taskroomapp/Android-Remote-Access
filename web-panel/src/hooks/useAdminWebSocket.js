import { useEffect, useState } from 'react';
import { subscribeAdminWebSocket } from '../lib/adminWebSocket';

export function useAdminWebSocket() {
    const [connected, setConnected] = useState(false);

    useEffect(() => subscribeAdminWebSocket(setConnected), []);

    return connected;
}
