import type { AppItem, CleanItem, CleanRunResult, DuplicateGroup, DuplicateResult, Entry, FileRow, HistoryItem, Level, SmartSummary, Status, SystemResult, TypeRow, Volume } from "../api";

export type Section = "overview" | "clean" | "analyze" | "dupes" | "apps" | "system" | "history";
export type TaskKind = "" | "smart" | "analyze" | "clean" | "dupes" | "apps" | "system" | "clean-run";
export type CleanStage = "idle" | "summary" | "review" | "report";
export type MapMode = "lens" | "map" | "types" | "large";

export type Task = {
  running: boolean;
  kind: TaskKind;
  label: string;
  phase: string;
  detail: string;
};

export type Scope = {
  roots: string[];
  exclude: string[];
  place: string;
};

export type State = {
  section: Section;
  task: Task;
  conflict: string;
  volume: Volume | null;
  elevated: boolean;
  places: Array<{ id: string; name: string; path: string }>;
  drives: Volume[];
  history: HistoryItem[];
  smart: SmartSummary | null;
  cleanItems: CleanItem[];
  cleanNote: string;
  cleanCancelled: boolean;
  cleanChecked: Record<string, boolean>;
  cleanStage: CleanStage;
  cleanResult: CleanRunResult | null;
  dupes: DuplicateResult | null;
  dupeScope: Scope;
  dupeOpen: Record<string, boolean>;
  dupeSort: "reclaim" | "count";
  apps: AppItem[];
  appNote: string;
  appQuery: string;
  appSort: "name" | "size";
  appPublisher: string;
  system: SystemResult | null;
  analyze: {
    status: Status | null;
    level: Level;
    types: TypeRow[];
    large: FileRow[];
    mode: MapMode;
    hot: string;
    selected: string;
  };
};

export type Action =
  | { type: "section"; section: Section }
  | { type: "task-started"; kind: TaskKind; label: string }
  | { type: "task-progress"; task: Task }
  | { type: "task-idle" }
  | { type: "conflict"; message: string }
  | { type: "dismiss-conflict" }
  | { type: "volume"; volume: Volume | null }
  | { type: "elevated"; elevated: boolean }
  | { type: "places"; places: State["places"] }
  | { type: "drives"; drives: Volume[] }
  | { type: "history"; history: HistoryItem[] }
  | { type: "smart"; summary: SmartSummary }
  | { type: "clean-scanned"; items: CleanItem[]; note: string; cancelled: boolean }
  | { type: "clean-check"; id: string; on: boolean }
  | { type: "clean-stage"; stage: CleanStage }
  | { type: "clean-result"; result: CleanRunResult }
  | { type: "dupes"; report: DuplicateResult }
  | { type: "dupe-scope"; scope: Scope }
  | { type: "dupe-toggle"; hash: string; fallback: boolean }
  | { type: "dupe-sort"; sort: "reclaim" | "count" }
  | { type: "apps"; rows: AppItem[]; note: string }
  | { type: "app-query"; query: string }
  | { type: "app-sort"; sort: "name" | "size" }
  | { type: "app-publisher"; publisher: string }
  | { type: "system"; report: SystemResult }
  | { type: "analyze"; status: Status | null; level: Level; types: TypeRow[]; large: FileRow[] }
  | { type: "analyze-status"; status: Status }
  | { type: "level"; level: Level }
  | { type: "mode"; mode: MapMode }
  | { type: "hot"; path: string }
  | { type: "selected"; path: string };

const emptyLevel: Level = { path: "", name: "", allocated: 0, files: 0, parent: "", capped: false, crumbs: [], entries: [] };

export const idleTask: Task = { running: false, kind: "", label: "", phase: "", detail: "" };

export function initialState(): State {
  return {
    section: "overview",
    task: idleTask,
    conflict: "",
    volume: null,
    elevated: false,
    places: [],
    drives: [],
    history: [],
    smart: null,
    cleanItems: [],
    cleanNote: "",
    cleanCancelled: false,
    cleanChecked: {},
    cleanStage: "idle",
    cleanResult: null,
    dupes: null,
    dupeScope: { roots: [], exclude: [], place: "" },
    dupeOpen: {},
    dupeSort: "reclaim",
    apps: [],
    appNote: "",
    appQuery: "",
    appSort: "name",
    appPublisher: "",
    system: null,
    analyze: { status: null, level: emptyLevel, types: [], large: [], mode: "lens", hot: "", selected: "" },
  };
}

export function initialChecks(items: CleanItem[]): Record<string, boolean> {
  const checks: Record<string, boolean> = {};
  for (const item of items) {
    checks[item.id] = item.defaultOn && item.risk < 3 && item.bytes > 0;
  }
  return checks;
}

export function cleanBuckets(items: CleanItem[]) {
  return {
    recommended: items.filter((item) => item.risk < 2),
    caution: items.filter((item) => item.risk === 2),
    advanced: items.filter((item) => item.risk >= 3),
  };
}

export function reclaimOf(group: DuplicateGroup): number {
  return Math.max(0, group.bytes) * Math.max(0, group.files.length - 1);
}

export function sortGroups(groups: DuplicateGroup[], sort: "reclaim" | "count"): DuplicateGroup[] {
  return [...groups].sort((a, b) => (sort === "count" ? b.files.length - a.files.length || reclaimOf(b) - reclaimOf(a) : reclaimOf(b) - reclaimOf(a)));
}

export const systemOrder = ["Windows 系统文件", "虚拟内存和休眠文件", "Docker", "WSL", "开发工具缓存", "其他开发环境"];

function finish(state: State, kinds: TaskKind[]): Task {
  if (!state.task.running || kinds.includes(state.task.kind)) return idleTask;
  return state.task;
}

export function reduce(state: State, action: Action): State {
  switch (action.type) {
    case "section":
      return { ...state, section: action.section };
    case "task-started":
      if (state.task.running) {
        const label = state.task.label || "当前任务";
        return { ...state, conflict: `正在执行${label}。请等待完成，或先取消当前任务。` };
      }
      return { ...state, conflict: "", task: { running: true, kind: action.kind, label: action.label, phase: "准备", detail: "" } };
    case "task-progress":
      if (!state.task.running) return state;
      return { ...state, task: { ...action.task, running: true } };
    case "task-idle":
      return { ...state, task: idleTask };
    case "conflict":
      return { ...state, conflict: action.message };
    case "dismiss-conflict":
      return { ...state, conflict: "" };
    case "volume":
      return { ...state, volume: action.volume };
    case "elevated":
      return { ...state, elevated: action.elevated };
    case "places":
      return { ...state, places: action.places };
    case "drives":
      return { ...state, drives: action.drives };
    case "history":
      return { ...state, history: action.history };
    case "smart":
      return { ...state, smart: action.summary, task: idleTask };
    case "clean-scanned":
      return {
        ...state,
        cleanItems: action.items,
        cleanNote: action.note,
        cleanCancelled: action.cancelled,
        cleanChecked: initialChecks(action.items),
        cleanStage: action.items.length > 0 || action.cancelled ? "summary" : "idle",
        cleanResult: null,
        task: finish(state, ["clean", "smart"]),
      };
    case "clean-check":
      return { ...state, cleanChecked: { ...state.cleanChecked, [action.id]: action.on } };
    case "clean-stage":
      return { ...state, cleanStage: action.stage };
    case "clean-result":
      return { ...state, cleanResult: action.result, cleanStage: "report", task: finish(state, ["clean-run"]) };
    case "dupes":
      return { ...state, dupes: action.report, task: finish(state, ["dupes", "smart"]) };
    case "dupe-scope":
      return { ...state, dupeScope: action.scope };
    case "dupe-toggle": {
      const current = Object.prototype.hasOwnProperty.call(state.dupeOpen, action.hash) ? state.dupeOpen[action.hash] : action.fallback;
      return { ...state, dupeOpen: { ...state.dupeOpen, [action.hash]: !current } };
    }
    case "dupe-sort":
      return { ...state, dupeSort: action.sort };
    case "apps":
      return { ...state, apps: action.rows, appNote: action.note, task: finish(state, ["apps", "smart"]) };
    case "app-query":
      return { ...state, appQuery: action.query };
    case "app-sort":
      return { ...state, appSort: action.sort };
    case "app-publisher":
      return { ...state, appPublisher: action.publisher };
    case "system":
      return { ...state, system: action.report, task: finish(state, ["system", "smart"]) };
    case "analyze-status":
      return { ...state, analyze: { ...state.analyze, status: action.status } };
    case "analyze":
      return {
        ...state,
        analyze: {
          ...state.analyze,
          status: action.status,
          level: action.level,
          types: action.types,
          large: action.large,
          hot: "",
          selected: action.level.entries[0]?.path ?? "",
        },
        task: state.task.kind === "analyze" && action.status && !action.status.running ? idleTask : state.task,
      };
    case "level":
      return { ...state, analyze: { ...state.analyze, level: action.level, hot: "", selected: action.level.entries[0]?.path ?? "" } };
    case "mode":
      return { ...state, analyze: { ...state.analyze, mode: action.mode } };
    case "hot":
      return { ...state, analyze: { ...state.analyze, hot: action.path } };
    case "selected":
      return { ...state, analyze: { ...state.analyze, selected: action.path } };
    default:
      return state;
  }
}
