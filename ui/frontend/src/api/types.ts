// The JSON internal/deskapi serves. Keep in step with its views.go,
// settingsapi.go, extras.go and model.go.

export type Platform = 'android' | 'ios' | string;

export type DeviceStatus =
  | 'needs_you'
  | 'needs_setup'
  | 'locked'
  | 'in_control'
  | 'working'
  | 'idle'
  | 'offline';

export interface Features {
  unlock: boolean;
  background: boolean;
  files: boolean;
  companion: boolean;
}

export interface Device {
  id: string;
  platform: Platform;
  name: string;
  model: string;
  osVersion: string;
  connection: string;
  backend: string;
  state: string;
  detail?: string;
  setup?: string;
  helperExpires?: string;
  status: DeviceStatus;
  statusLine: string;
  taskId?: string;
  features: Features;
}

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Point {
  x: number;
  y: number;
}

export type TaskState = 'working' | 'waiting' | 'paused' | 'done' | 'stopped' | 'failed';

export interface Step {
  index: number;
  text: string;
  tool?: string;
  args?: string;
  status: string;
  at: string;
  elapsedMs: number;
  before?: string;
  after?: string;
  target?: Rect;
  point?: Point;
}

export interface Task {
  id: string;
  deviceId: string;
  deviceName: string;
  agent: string;
  title: string;
  state: TaskState;
  started: string;
  ended?: string;
  steps: Step[];
  readOnly?: boolean;
}

export type NeedKind = 'approval' | 'unlock' | 'handoff';

export interface Need {
  id: string;
  kind: NeedKind;
  deviceId: string;
  deviceName: string;
  app?: string;
  agent?: string;
  taskId?: string;
  title: string;
  detail?: string;
  rule?: string;
  created: string;
  deadline?: string;
  hasScreenshot: boolean;
  shotW?: number;
  shotH?: number;
  target?: Rect;
  point?: Point;
  to?: Point;
  alwaysAllow?: boolean;
  allowTask?: boolean;
  endsOnUnlock?: boolean;
  unblock?: boolean;
  readOnly?: boolean;
}

export type Decision = 'allow_once' | 'allow_task' | 'allow_always' | 'deny' | 'take_over';

export interface AppState {
  devices: Device[];
  tasks: Task[];
  needs: Need[];
}

export interface Bootstrap {
  version: string;
  platform: string;
  startPage?: string;
  agentToken?: string;
  rulesFile?: string;
  changelog?: string;
  inAppAgent: boolean;
  canQuit: boolean;
  keychain: boolean;
  telemetry: boolean;
  has: Record<string, boolean>;
}

export interface BlockedApp {
  name: string;
  kind?: string;
  id?: string;
}

export interface Settings {
  preset: number;
  blockedApps: BlockedApp[];
  readOnly: boolean;
  unlockFromComputer: boolean;
  hideCodes: boolean;
  keepDays: number;
  record: boolean;
  developerMode: boolean;
  remoteAgents: boolean;
  remoteAddr: string;
  usageStats: boolean;
  autoUpdate: boolean;
  chatModel: string;
  chatAgentModel: string;
  ollamaBaseURL: string;
  ollamaModels: string[];
}

export interface HistoryPage {
  tasks: Task[];
  more: boolean;
}

export interface PINState {
  saved: boolean;
  supported: boolean;
}

export interface MirrorSize {
  deviceId: string;
  w: number;
  h: number;
}

export interface ScopeView {
  mode: 'everything' | 'read_only' | 'watch_only' | 'apps' | string;
  apps: string[];
}

export interface AgentView {
  name: string;
  approved: boolean;
  scope: ScopeView;
}

export interface AgentSetupView {
  id: string;
  name: string;
  url?: string;
  found: boolean;
  state: 'connected' | 'outdated' | 'not_connected';
  command?: string;
  settings?: string;
  restart: 'none' | 'session' | 'app';
  problem?: string;
  manual?: boolean;
  extension?: boolean;
  opens?: boolean;
}

export interface Exemption {
  rule: string;
  with: string[] | null;
  appId: string;
  appName: string;
}

export interface Rule {
  name: string;
  apps: string[];
  categories: string[];
  actions: string[];
  labels: string[];
  decision: 'allow' | 'ask' | 'deny' | string;
}

export interface RulesView {
  file: Rule[];
  builtIn: Rule[];
}

export interface FileProblem {
  file: string;
  path: string;
  error: string;
}

export interface HostCheck {
  missing?: string;
  detail?: string;
}

export interface Check {
  id: string;
  status: 'pass' | 'fail' | 'warn' | 'skip' | string;
  fix?: string;
  detail?: string;
}

export interface Checkup {
  deviceId: string;
  platform: string;
  family?: string;
  system?: string;
  checks: Check[];
  at: string;
}

export interface ConnectionWay {
  id: string;
  available: boolean;
  reason?: string;
  detail?: string;
}

export interface Connections {
  chosen: string;
  current: string;
  ways: ConnectionWay[];
}

export interface RecordingInfo {
  taskId: string;
  start: string;
  lengthMs: number;
  hasFile: boolean;
  marks: { atMs: number; step: number; text: string }[];
}

export interface ProfileView {
  id: string;
  provider: string;
  label: string;
  hasKey: boolean;
}

export interface ChatModelView {
  id: string;
  provider: string;
  label: string;
  ready: boolean;
}

export interface ModelView {
  keychain: boolean;
  profiles: ProfileView[];
  active: string;
  chatModel: string;
  chatAgentModel: string;
  models: ChatModelView[];
  ollamaBaseURL: string;
  ollamaModels: string[];
  providers: { id: string; label: string }[];
}

export interface DiskImage {
  needed: boolean;
  name: string;
  image: string;
  size: number;
  progress: { done: number; total: number; running: boolean; error?: string; ok: boolean };
}

export interface ChangeEvent {
  kind: 'devices' | 'tasks' | 'needs';
  deviceId?: string;
  taskId?: string;
  needId?: string;
}

/** What Pinky keeps for replaying tasks: deskapi's Library (GET /api/library). */
export interface Library {
  /** The folder the files are in. */
  dir?: string;
  apps: LibraryApp[];
  /** Flow files that cannot be read. */
  broken?: string[];
  /** How many bytes the notes on one app may take. */
  maxNotes: number;
}

/** One app's notes, flows and parts; app is "" for the flows of no app. */
export interface LibraryApp {
  app: string;
  notes: string;
  flows: LibraryFlow[];
  parts: LibraryPart[];
}

export interface LibraryFlow {
  name: string;
  description?: string;
  /** The request the flow was kept from. */
  task?: string;
  version: number;
  file?: string;
  steps: LibraryStep[];
}

export interface LibraryStep {
  kind: string;
  /** What the step does, as Pinky words it. */
  text: string;
  /** What the screen shows afterwards. */
  leads?: string;
  /** Why, in the words of the agent that first did it. */
  intent?: string;
  /** The id of the part the step names. */
  part?: string;
  /** That part is gone. */
  missing?: boolean;
  optional?: boolean;
  /** The part has a picture of where it leads (api.partShotURL). */
  shot?: boolean;
}

export interface LibraryPart {
  id: string;
  kind: string;
  text: string;
  leads?: string;
  intent?: string;
  after?: string[];
  seen: number;
  last?: string;
  /** Replays found it leading elsewhere; agents are not offered it. */
  broken?: boolean;
  /** The flows that use it. */
  flows?: string[];
  /** It has a picture of where it leads (api.partShotURL). */
  shot?: boolean;
}
