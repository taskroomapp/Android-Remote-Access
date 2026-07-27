import React from 'react';
import {
    LayoutDashboard,
    Smartphone,
    Terminal,
    ScrollText,
    Settings,
    FolderOpen,
    Download,
    MapPin,
    Users,
    Radio,
    LogOut,
    Menu,
    X,
    Search,
    Lock,
    AlertTriangle,
    CheckCircle2,
    XCircle,
    Clock,
    Loader2,
    Battery,
    BatteryLow,
    BatteryMedium,
    BatteryFull,
    Calendar,
    Package,
    Factory,
    ChevronRight,
    ChevronDown,
    Home,
    File,
    Folder,
    Image,
    FileText,
    Music,
    Video,
    RefreshCw,
    Upload,
    Eye,
    Trash2,
    Filter,
    Grid3x3,
    List,
    ArrowUpDown,
    Wifi,
    WifiOff,
    Camera,
    Mic,
    Play,
    Square,
    Send,
    MessageSquare,
    Star,
    Phone,
    Mail,
    Map,
    Satellite,
    Mountain,
    ExternalLink,
    RotateCcw,
    Ban,
    HardDrive,
    Cloud,
    Server,
    Circle,
    MoreHorizontal,
    PanelLeftClose,
    PanelLeft,
    Info,
    Film,
    UnfoldVertical,
    FoldVertical,
    ListChecks,
    Globe2,
    Keyboard,
    FolderTree,
    CircleHelp,
    ChevronLeft,
    ArrowUp,
    ArrowDown,
    Maximize2,
} from 'lucide-react';

const ICONS = {
    dashboard: LayoutDashboard,
    devices: Smartphone,
    console: Terminal,
    orders: Terminal,
    audit: ScrollText,
    settings: Settings,
    files: FolderOpen,
    downloads: Download,
    location: MapPin,
    contacts: Users,
    live: Radio,
    logout: LogOut,
    menu: Menu,
    close: X,
    search: Search,
    lock: Lock,
    warning: AlertTriangle,
    success: CheckCircle2,
    error: XCircle,
    pending: Clock,
    spinner: Loader2,
    battery: Battery,
    batteryLow: BatteryLow,
    batteryMedium: BatteryMedium,
    batteryFull: BatteryFull,
    calendar: Calendar,
    package: Package,
    factory: Factory,
    chevronRight: ChevronRight,
    chevronDown: ChevronDown,
    home: Home,
    file: File,
    folder: Folder,
    image: Image,
    fileText: FileText,
    music: Music,
    video: Video,
    refresh: RefreshCw,
    upload: Upload,
    eye: Eye,
    trash: Trash2,
    filter: Filter,
    grid: Grid3x3,
    list: List,
    sort: ArrowUpDown,
    wifi: Wifi,
    wifiOff: WifiOff,
    camera: Camera,
    mic: Mic,
    play: Play,
    stop: Square,
    send: Send,
    message: MessageSquare,
    star: Star,
    phone: Phone,
    mail: Mail,
    map: Map,
    satellite: Satellite,
    terrain: Mountain,
    external: ExternalLink,
    retry: RotateCcw,
    cancel: Ban,
    storage: HardDrive,
    cloud: Cloud,
    server: Server,
    dot: Circle,
    more: MoreHorizontal,
    panelClose: PanelLeftClose,
    panelOpen: PanelLeft,
    folderOpen: FolderOpen,
    info: Info,
    film: Film,
    smartphone: Smartphone,
    unfold: UnfoldVertical,
    fold: FoldVertical,
    listChecks: ListChecks,
    globe: Globe2,
    keyboard: Keyboard,
    fileTree: FolderTree,
    help: CircleHelp,
    chevronLeft: ChevronLeft,
    arrowUp: ArrowUp,
    arrowDown: ArrowDown,
    download: Download,
    maximize: Maximize2,
    logo: Smartphone,
};

/** SVG icon wrapper — use `name` keys from ICONS instead of emoji text. */
export default function Icon({ name, size = 20, className = '', strokeWidth = 2, ...rest }) {
    const Component = ICONS[name];
    if (!Component) {
        return null;
    }
    return (
        <Component
            size={size}
            strokeWidth={strokeWidth}
            className={`ui-icon ${className}`.trim()}
            aria-hidden={rest['aria-label'] ? undefined : true}
            {...rest}
        />
    );
}

export function fileTypeIcon(name, isDirectory) {
    if (isDirectory) return 'folder';
    const lower = (name || '').toLowerCase();
    if (/\.(png|jpe?g|gif|webp|bmp|svg)$/.test(lower)) return 'image';
    if (/\.(mp4|mkv|webm|mov|avi)$/.test(lower)) return 'video';
    if (/\.(mp3|wav|m4a|ogg|flac)$/.test(lower)) return 'music';
    if (/\.(txt|md|json|xml|html|css|js|ts|log|csv)$/.test(lower)) return 'fileText';
    return 'file';
}

export function statusIcon(status) {
    switch (status) {
        case 'success':
        case 'completed':
            return 'success';
        case 'error':
        case 'failed':
            return 'error';
        case 'pending':
        case 'waiting':
        case 'queued':
            return 'pending';
        case 'in_progress':
            return 'spinner';
        case 'cancelled':
            return 'cancel';
        default:
            return 'dot';
    }
}
