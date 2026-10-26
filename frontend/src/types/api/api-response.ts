// Define the main API response structure
export interface APIResponse<T> {
  status: string; // 'success' | 'error' | 'warn'
  data?: T; // Optional response data
  error?: LogErrorInfo; // Optional error information
}

export interface LogData {
  status: string;
  level: string;
  message: string;
  name: string;
  timestamp: string;
  elapsed_us: number;
  route?: LogRouteInfo;
  actions?: LogAction[];
  time: string;
}

export interface LogRouteInfo {
  method: string;
  path: string;
  params: Record<string, string>;
  ip: string;
  response_bytes: number;
  action?: string; // The route's key in the Logs page actions, which matches routes with {params} too
}

export interface LogAction {
  name: string;
  status: string;
  level?: string;
  warnings?: Record<string, unknown>;
  result?: Record<string, unknown>;
  error?: LogErrorInfo;
  timestamp: string;
  elapsed_us: number;
  sub_actions?: LogAction[];
}

// Define the error structure
export interface LogErrorInfo {
  function: string;
  line_number: number;
  message: string;
  detail?: Record<string, unknown>;
  help?: string;
}
