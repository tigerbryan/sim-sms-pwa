export interface VersionInfo {
  version: string
  commit: string
  build_time: string
  go_version: string
  qmi_go_version: string
  smscodec_version: string
  license: string
  sms_only: boolean
}

export interface Device {
  id: string
  control_path: string
  driver: string
  usb_vid: string
  usb_pid: string
  manufacturer: string
  product: string
  network_interface: string
  serial_ports: string[]
  sysfs_path: string
  status: string
  busy: boolean
}

export interface DevicesResponse {
  devices: Device[]
  backend: string
  scan_time: string
}

export interface SIMInfo {
  available: boolean
  present: boolean
  ready: boolean
  pin_status: string
  operator: string
  mcc: string
  mnc: string
  imsi?: string
  iccid?: string
  phone?: string
  registered: boolean
  roaming: boolean
}

export interface SignalInfo {
  available: boolean
  rssi: string
  rsrp: string
  rsrq: string
  sinr: string
  technology: string
  plmn: string
  roaming: boolean
  registered: boolean
}

export interface GuardFinding {
  code: string
  interface?: string
  detail: string
}

export interface GuardState {
  state: string
  checked_at: string
  wds_status: string
  findings: GuardFinding[]
}

export interface QMIValidation {
  mode: string
  stage: string
  device_open: string
  dms: string
  uim: string
  nas: string
  wds: string
  wds_status: string
  wms_list: string
  wms_subscribe: string
  sms: string
  stored_messages: number
  imported_messages: number
  terminal: boolean
  detail?: string
  updated_at: string
}

export interface PushPlusConfig {
  enabled: boolean
  template: string
  token_configured: boolean
}

export interface DashboardSMS {
  total: number
  unread: number
  last?: string
}

export interface DashboardNotifications {
  pushplus: PushPlusConfig
}

export interface Dashboard {
  version: string
  backend: string
  device_status: string
  device?: Device
  qmi_status: string
  sim: SIMInfo
  signal: SignalInfo
  sms: DashboardSMS
  notifications: DashboardNotifications
  unread_sms: number
  last_sms?: string
  pushplus: PushPlusConfig
  uptime_seconds: number
  database_status: string
  sms_only: boolean
  data_guard: GuardState
  qmi_validation: QMIValidation
}

export interface SMSMessage {
  id: number
  device_id: string
  sender: string
  recipient?: string
  timestamp: string
  received_at: string
  encoding: string
  body: string
  is_multipart: boolean
  parts_total: number
  parts_received: number
  status: string
}

export interface SMSPage {
  unread_total?: number
  items: SMSMessage[]
  page: number
  page_size: number
  per_page: number
  total: number
}

export interface NotificationItem {
  id: number
  kind: string
  title: string
  status: string
  attempts: number
  next_attempt_at: string
  created_at: string
  updated_at: string
}

export interface NotificationsResponse {
  pushplus: PushPlusConfig
  items: NotificationItem[]
}

export interface GeneralSettings {
  backend: string
  scan_interval: string
  backend_restart_required: boolean
}

export interface SecuritySettings {
  sms_only: boolean
  immutable: boolean
  message: string
}

export interface SMSSettings {
  sending_enabled: boolean
  store_raw_pdu: boolean
}

export interface LoggingSettings {
  level: 'debug' | 'info' | 'warn' | 'error'
}

export interface SettingsResponse {
  general: GeneralSettings
  security: SecuritySettings
  sms: SMSSettings
  pushplus: PushPlusConfig
  logging: LoggingSettings
}

export interface SettingsUpdate {
  general?: Pick<GeneralSettings, 'scan_interval'>
  logging?: LoggingSettings
  pushplus?: {
    enabled: boolean
    token: string
    template: string
  }
}

export interface LogEntry {
  time: string
  level: string
  message: string
  fields?: Record<string, unknown>
}

export interface LogsResponse {
  items: LogEntry[]
}

export interface DiagnosticsResponse {
  version: VersionInfo
  os: string
  architecture: string
  uptime_seconds: number
  database_ready: boolean
  backend: string
  detected_devices: Device[]
  active_device?: Device | null
  sms_only: boolean
  guard: GuardState
  qmi_validation: QMIValidation
}

export interface AuthStatus {
  authenticated: boolean
  username: string
}

export interface LoginReply {
  authenticated: boolean
  csrf_token: string
}

export interface APIErrorResponse {
  error?: {
    code?: string
    message?: string
  }
}
