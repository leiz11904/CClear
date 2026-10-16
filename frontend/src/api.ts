export type Volume = { drive: string; total: number; free: number; used: number };
export type Status = {
  running: boolean;
  ready: boolean;
  cancelled: boolean;
  files: number;
  allocated: number;
  note: string;
  error: string;
  where: string;
};
export type Crumb = { name: string; path: string };
export type Entry = {
  name: string;
  path: string;
  allocated: number;
  files: number;
  percent: number;
  color: number;
  x: number;
  y: number;
  r: number;
  tx: number;
  ty: number;
  tw: number;
  th: number;
};
export type Level = {
  path: string;
  name: string;
  allocated: number;
  files: number;
  parent: string;
  capped: boolean;
  crumbs: Crumb[];
  entries: Entry[];
};
export type TypeRow = { ext: string; files: number; allocated: number; percent: number };
export type FileRow = { name: string; path: string; allocated: number };
export type CleanItem = {
  id: string;
  name: string;
  description: string;
  group: string;
  warning: string;
  note: string;
  risk: number;
  count: number;
  bytes: number;
  unknown: boolean;
  defaultOn: boolean;
  needsAdmin: boolean;
  truncated: boolean;
  errors: string[];
  preview: string[];
};
export type CleanScanResult = { items: CleanItem[]; cancelled: boolean; note: string };
export type CleanRunResult = {
  freed: number;
  estimated: number;
  deleted: number;
  skipped: number;
  failed: number;
  errors: string[];
  logPath: string;
  cancelled: boolean;
  partial: boolean;
};
export type DuplicateFile = { path: string; bytes: number };
export type DuplicateGroup = { hash: string; bytes: number; files: DuplicateFile[] };
export type DuplicateResult = {
  groups: DuplicateGroup[];
  reclaimable: number;
  note: string;
  errors: string[];
  truncated: boolean;
  cancelled: boolean;
};
export type DuplicateQuery = { roots: string[]; exclude: string[] };
export type AppItem = {
  id: string;
  name: string;
  version: string;
  publisher: string;
  location: string;
  note: string;
  bytes: number;
  sizeKnown: boolean;
  kind: string;
  canUninstall: boolean;
};
export type AppList = { apps: AppItem[]; note: string; errors: string[]; cancelled: boolean };
export type SystemItem = {
  name: string;
  kind: string;
  path: string;
  status: string;
  note: string;
  bytes: number;
  known: boolean;
  group: string;
  setting: string;
};
export type SystemResult = { items: SystemItem[]; note: string };
export type HistoryItem = {
  when: string;
  version: string;
  logical: string;
  estimated: string;
  deleted: number;
  skipped: number;
  failed: number;
  items: string[];
};
export type SecurityStatus = { elevated: boolean };
export type Place = { id: string; name: string; path: string };
export type TaskState = { running: boolean; kind: string; label: string; phase: string; detail: string };
export type SmartSummary = {
  when: string;
  cancelled: boolean;
  completed: string[];
  cleanBytes: number;
  cleanCount: number;
  diskAllocated: number;
  diskFiles: number;
  dupeBytes: number;
  dupeGroups: number;
  appCount: number;
  largeBytes: number;
  largeCount: number;
  systemCount: number;
  note: string;
};
export type Snapshot = {
  task: TaskState;
  clean: CleanScanResult;
  dupes: DuplicateResult;
  apps: AppList;
  system: SystemResult;
  smart: SmartSummary;
  status: Status;
};

type GoApp = {
  Volume: () => Promise<Volume>;
  Start: () => Promise<void>;
  Cancel: () => Promise<void>;
  Status: () => Promise<Status>;
  Level: (path: string) => Promise<Level>;
  Types: () => Promise<TypeRow[]>;
  Large: () => Promise<FileRow[]>;
  Reveal: (path: string) => Promise<void>;
  ScanCleanup: () => Promise<CleanScanResult>;
  RunCleanup: (ids: string[], confirmation: string) => Promise<CleanRunResult>;
  FindDuplicates: (query: DuplicateQuery) => Promise<DuplicateResult>;
  RecycleDuplicate: (path: string, confirmation: string) => Promise<void>;
  InstalledApps: () => Promise<AppList>;
  LaunchUninstall: (name: string, confirmation: string) => Promise<void>;
  ReadSystemTools: () => Promise<SystemResult>;
  CleanupHistory: () => Promise<HistoryItem[]>;
  CancelToolWork: () => Promise<void>;
  SecurityStatus: () => Promise<SecurityStatus>;
  RelaunchElevated: (confirmation: string) => Promise<void>;
  StartSmart: () => Promise<void>;
  Task: () => Promise<TaskState>;
  Snapshot: () => Promise<Snapshot>;
  Places: () => Promise<Place[]>;
  Drives: () => Promise<Volume[]>;
  OpenSetting: (id: string) => Promise<void>;
};

type Runtime = {
  EventsOn: (name: string, cb: (payload: never) => void) => () => void;
  OpenDirectoryDialog?: (options: { Title: string }) => Promise<string>;
};

const demo = import.meta.env.DEV && new URLSearchParams(window.location.search).has("demo");
const listeners = new Map<string, Set<(payload: never) => void>>();

function emit<T>(name: string, payload: T) {
  listeners.get(name)?.forEach((cb) => cb(payload as never));
}

const demoRoot: Level = {
  path: "C:\\",
  name: "Windows (C:)",
  allocated: 412_800_000_000,
  files: 1_286_430,
  parent: "",
  capped: false,
  crumbs: [{ name: "Windows (C:)", path: "C:\\" }],
  entries: [
    { name: "Users", path: "C:\\Users", allocated: 128_400_000_000, files: 346_281, percent: 31.1, color: 0, x: 250, y: 280, r: 150, tx: 0, ty: 0, tw: 420, th: 360 },
    { name: "Program Files", path: "C:\\Program Files", allocated: 83_600_000_000, files: 214_902, percent: 20.3, color: 1, x: 560, y: 180, r: 112, tx: 420, ty: 0, tw: 300, th: 240 },
    { name: "Windows", path: "C:\\Windows", allocated: 74_200_000_000, files: 504_183, percent: 18.0, color: 2, x: 760, y: 360, r: 104, tx: 720, ty: 0, tw: 280, th: 240 },
    { name: "ProgramData", path: "C:\\ProgramData", allocated: 42_700_000_000, files: 95_043, percent: 10.3, color: 3, x: 430, y: 470, r: 78, tx: 420, ty: 240, tw: 300, th: 180 },
    { name: "Games", path: "C:\\Games", allocated: 31_900_000_000, files: 43_018, percent: 7.7, color: 4, x: 680, y: 520, r: 66, tx: 720, ty: 240, tw: 280, th: 200 },
    { name: "临时文件", path: "C:\\Temp", allocated: 9_400_000_000, files: 58_400, percent: 2.3, color: 5, x: 180, y: 500, r: 40, tx: 0, ty: 360, tw: 420, th: 140 },
  ],
};

function demoChild(path: string): Level {
  const picked = demoRoot.entries.find((entry) => entry.path === path) ?? demoRoot.entries[0];
  return {
    path: picked.path,
    name: picked.name,
    allocated: picked.allocated,
    files: picked.files,
    parent: "C:\\",
    capped: false,
    crumbs: [...demoRoot.crumbs, { name: picked.name, path: picked.path }],
    entries: [
      { name: "主要内容", path: `${picked.path}\\主要内容`, allocated: Math.round(picked.allocated * 0.62), files: 10, percent: 62, color: 0, x: 360, y: 300, r: 140, tx: 0, ty: 0, tw: 620, th: 640 },
      { name: "其他项目", path: `${picked.path}\\其他项目`, allocated: Math.round(picked.allocated * 0.38), files: 4, percent: 38, color: 1, x: 700, y: 280, r: 96, tx: 620, ty: 0, tw: 380, th: 640 },
    ],
  };
}

const demoClean: CleanItem[] = [
  { id: "temp", name: "临时文件", description: "超过 72 小时的普通临时文件", group: "regular", warning: "", note: "", risk: 1, count: 1842, bytes: 2_860_000_000, unknown: false, defaultOn: true, needsAdmin: false, truncated: false, errors: [], preview: ["C:\\Users\\demo\\AppData\\Local\\Temp\\old.tmp"] },
  { id: "browser", name: "浏览器缓存", description: "浏览器关闭时可安全重建的缓存", group: "regular", warning: "", note: "有 1 个目录未能读取", risk: 1, count: 923, bytes: 1_240_000_000, unknown: false, defaultOn: true, needsAdmin: false, truncated: true, errors: ["open C:\\Users\\demo\\AppData\\Local\\Google\\Chrome\\Cache: access is denied"], preview: [] },
  { id: "recycle", name: "回收站", description: "清空系统盘回收站", group: "recycle", warning: "清空后无法从回收站恢复", note: "", risk: 2, count: 31, bytes: 5_700_000_000, unknown: false, defaultOn: false, needsAdmin: false, truncated: false, errors: [], preview: [] },
  { id: "dism", name: "组件存储", description: "交给 Windows DISM 清理已取代组件", group: "system", warning: "系统级操作", note: "待系统分析", risk: 3, count: 0, bytes: -1, unknown: true, defaultOn: false, needsAdmin: true, truncated: false, errors: [], preview: [] },
];

const demoDupes: DuplicateResult = {
  reclaimable: 3_400_000_000,
  note: "按内容哈希确认，不会自动删除。",
  errors: ["open C:\\System Volume Information: access is denied"],
  truncated: true,
  cancelled: false,
  groups: [
    { hash: "demo-a", bytes: 1_700_000_000, files: [{ path: "C:\\Users\\demo\\Videos\\video-copy.mp4", bytes: 1_700_000_000 }, { path: "C:\\Users\\demo\\Downloads\\video-copy.mp4", bytes: 1_700_000_000 }] },
    { hash: "demo-b", bytes: 420_000_000, files: [{ path: "C:\\Users\\demo\\Documents\\archive.zip", bytes: 420_000_000 }, { path: "C:\\Users\\demo\\Documents\\archive-2.zip", bytes: 420_000_000 }, { path: "C:\\Backup\\archive.zip", bytes: 420_000_000 }] },
  ],
};

const demoApps: AppList = {
  note: "占用数字来自系统登记。卸载只会启动登记的卸载程序。",
  errors: [],
  cancelled: false,
  apps: [
    { id: "0", name: "示例应用", version: "4.2.1", publisher: "Example Studio", location: "C:\\Program Files\\Example", note: "", bytes: 2_400_000_000, sizeKnown: true, kind: "win32", canUninstall: true },
    { id: "1", name: "示例商店应用", version: "1.8", publisher: "Contoso", location: "C:\\Program Files\\WindowsApps\\Contoso", note: "", bytes: 180_000_000, sizeKnown: true, kind: "store", canUninstall: true },
    { id: "2", name: "Windows 组件示例", version: "10.0", publisher: "Microsoft Corporation", location: "C:\\Windows", note: "系统组件", bytes: 0, sizeKnown: false, kind: "system", canUninstall: false },
  ],
};

const demoSystem: SystemResult = {
  note: "系统文件只展示占用。CClear 不会删除 pagefile、hiberfil、WinSxS 或系统还原点。",
  items: [
    { name: "虚拟内存 pagefile.sys", kind: "系统", path: "C:\\pagefile.sys", status: "正常", note: "由 Windows 管理，禁止删除。", bytes: 8_590_000_000, known: true, group: "虚拟内存和休眠文件", setting: "pagefile" },
    { name: "休眠文件 hiberfil.sys", kind: "系统", path: "C:\\hiberfil.sys", status: "正常", note: "由 Windows 管理，禁止删除。", bytes: 6_400_000_000, known: true, group: "虚拟内存和休眠文件", setting: "pagefile" },
    { name: "组件存储 WinSxS", kind: "系统", path: "C:\\Windows\\WinSxS", status: "见磁盘分析", note: "只能通过系统 DISM 清理。", bytes: -1, known: true, group: "Windows 系统文件", setting: "storage" },
    { name: "Docker Desktop 数据", kind: "运行环境", path: "C:\\Users\\demo\\AppData\\Local\\Docker", status: "只读", note: "运行环境，不自动清理。", bytes: 18_200_000_000, known: true, group: "Docker", setting: "" },
    { name: "WSL 虚拟磁盘", kind: "虚拟磁盘", path: "C:\\Users\\demo\\AppData\\Local\\Packages\\CanonicalGroupLimited\\LocalState\\ext4.vhdx", status: "只读", note: "VHDX，禁止直接删除。", bytes: 12_000_000_000, known: true, group: "WSL", setting: "" },
    { name: "npm 缓存", kind: "缓存", path: "C:\\Users\\demo\\AppData\\Local\\npm-cache", status: "只读", note: "缓存。", bytes: 640_000_000, known: true, group: "开发工具缓存", setting: "" },
    { name: "Go 模块缓存", kind: "缓存", path: "C:\\Users\\demo\\go\\pkg\\mod", status: "只读", note: "模块缓存。", bytes: 2_100_000_000, known: true, group: "开发工具缓存", setting: "" },
  ],
};

let demoTask: TaskState = { running: false, kind: "", label: "", phase: "", detail: "" };
let demoToken = 0;
const emptySmart: SmartSummary = { when: "", cancelled: false, completed: [], cleanBytes: 0, cleanCount: 0, diskAllocated: 0, diskFiles: 0, dupeBytes: 0, dupeGroups: 0, appCount: 0, largeBytes: 0, largeCount: 0, systemCount: 0, note: "" };
const demoMemory: Snapshot = {
  task: demoTask,
  clean: { items: [], cancelled: false, note: "" },
  dupes: { groups: [], reclaimable: 0, note: "", errors: [], truncated: false, cancelled: false },
  apps: { apps: [], note: "", errors: [], cancelled: false },
  system: { items: [], note: "" },
  smart: emptySmart,
  status: { running: false, ready: false, cancelled: false, files: 0, allocated: 0, note: "", error: "", where: "" },
};

function demoBusy(kind: string, label: string) {
  if (demoTask.running) throw new Error(`正在执行${demoTask.label}。请等待完成，或先取消当前任务`);
  demoTask = { running: true, kind, label, phase: "准备", detail: "" };
  emit("task:progress", demoTask);
}

function demoRelease() {
  demoTask = { running: false, kind: "", label: "", phase: "", detail: "" };
}

const wait = (ms: number) => new Promise((resolve) => window.setTimeout(resolve, ms));

const demoApp: GoApp = {
  Volume: async () => ({ drive: "C:", total: 1_000_000_000_000, used: 641_000_000_000, free: 359_000_000_000 }),
  Start: async () => {
    demoBusy("analyze", "磁盘分析");
    const token = ++demoToken;
    demoTask = { ...demoTask, phase: "磁盘占用", detail: "Users" };
    emit("task:progress", demoTask);
    emit("analyze:progress", { files: 1200, where: "C:\\Users" });
    await wait(400);
    if (demoToken !== token) return;
    demoRelease();
    emit("analyze:done", { running: false, ready: true, cancelled: false, files: demoRoot.files, allocated: demoRoot.allocated, note: "", error: "", where: "" });
  },
  Cancel: async () => {
    demoToken += 1;
    const kind = demoTask.kind;
    demoRelease();
    if (kind === "analyze" || kind === "smart") {
      demoMemory.status = { running: false, ready: true, cancelled: true, files: 1200, allocated: 20_000_000_000, note: "扫描已取消。已统计的部分仍只用于展示占用。", error: "", where: "" };
      emit("analyze:done", demoMemory.status);
    }
    if (kind === "smart") {
      demoMemory.clean = { items: demoClean.slice(0, 2), cancelled: true, note: "扫描已取消，以下为已完成的部分结果。" };
      const summary: SmartSummary = { when: "刚刚", cancelled: true, completed: ["垃圾候选项"], cleanBytes: 4_100_000_000, cleanCount: 2, diskAllocated: 20_000_000_000, diskFiles: 1200, dupeBytes: 0, dupeGroups: 0, appCount: 0, largeBytes: 0, largeCount: 0, systemCount: 0, note: "智能扫描已取消，已完成的阶段结果已保留。" };
      demoMemory.smart = summary;
      emit("task:done", summary);
    }
  },
  Status: async () => ({ running: demoTask.kind === "analyze", ready: true, cancelled: false, files: demoRoot.files, allocated: demoRoot.allocated, note: "", error: "", where: "" }),
  Level: async (path) => (!path || path === "C:\\" ? demoRoot : demoChild(path)),
  Types: async () => [
    { ext: "视频", files: 284, allocated: 142_600_000_000, percent: 34.5 },
    { ext: "应用程序", files: 7820, allocated: 96_200_000_000, percent: 23.3 },
    { ext: "图片", files: 38401, allocated: 58_900_000_000, percent: 14.3 },
  ],
  Large: async () => [
    { name: "archive-backup.zip", path: "C:\\Users\\demo\\Downloads\\archive-backup.zip", allocated: 18_700_000_000 },
    { name: "Windows.iso", path: "C:\\Users\\demo\\Downloads\\Windows.iso", allocated: 6_400_000_000 },
  ],
  Reveal: async () => {},
  ScanCleanup: async () => {
    demoBusy("clean", "垃圾扫描");
    await wait(250);
    demoRelease();
    demoMemory.clean = { items: demoClean, cancelled: false, note: "" };
    return demoMemory.clean;
  },
  RunCleanup: async () => {
    demoBusy("clean-run", "垃圾清理");
    await wait(250);
    demoRelease();
    return { freed: 4_100_000_000, estimated: 3_700_000_000, deleted: 2765, skipped: 4, failed: 1, errors: ["演示：有 1 个文件正在使用"], logPath: "C:\\Users\\demo\\AppData\\Roaming\\CClear\\logs\\clean-demo.log", cancelled: false, partial: false };
  },
  FindDuplicates: async (query) => {
    demoBusy("dupes", "重复文件");
    for (const phase of ["枚举", "候选筛选", "内容哈希"]) {
      demoTask = { ...demoTask, phase: "重复文件", detail: phase };
      emit("task:progress", demoTask);
      await wait(180);
    }
    demoRelease();
    const report = { ...demoDupes, note: `范围：${query.roots.join("、") || "系统盘"}${query.exclude.length ? `；排除 ${query.exclude.length} 个目录` : ""}。${demoDupes.note}` };
    demoMemory.dupes = report;
    return report;
  },
  RecycleDuplicate: async () => {},
  InstalledApps: async () => {
    demoBusy("apps", "软件信息");
    await wait(200);
    demoRelease();
    demoMemory.apps = demoApps;
    return demoApps;
  },
  LaunchUninstall: async () => {},
  ReadSystemTools: async () => {
    demoBusy("system", "系统占用");
    await wait(200);
    demoRelease();
    demoMemory.system = demoSystem;
    return demoSystem;
  },
  CleanupHistory: async () => [{ when: "2026-10-16 09:30", version: "3.0.0", logical: "3.8 GB", estimated: "3.4 GB", deleted: 2765, skipped: 4, failed: 0, items: ["临时文件", "浏览器缓存"] }],
  CancelToolWork: async () => {
    demoToken += 1;
    demoRelease();
  },
  SecurityStatus: async () => ({ elevated: false }),
  RelaunchElevated: async () => {},
  StartSmart: async () => {
    demoBusy("smart", "智能扫描");
    const token = ++demoToken;
    const phases = ["垃圾候选项", "磁盘占用", "重复文件", "软件信息", "系统与开发环境"];
    for (const phase of phases) {
      if (demoToken !== token) {
        emit("task:done", { when: "刚刚", cancelled: true, completed: phases.slice(0, phases.indexOf(phase)), cleanBytes: 4_100_000_000, cleanCount: demoClean.length, diskAllocated: 0, diskFiles: 0, dupeBytes: 0, dupeGroups: 0, appCount: 0, largeBytes: 0, largeCount: 0, systemCount: 0, note: "智能扫描已取消，已完成的阶段结果已保留。" });
        return;
      }
      demoTask = { ...demoTask, phase, detail: phase };
      emit("task:progress", demoTask);
      await wait(280);
    }
    if (demoToken !== token) return;
    demoRelease();
    demoMemory.clean = { items: demoClean, cancelled: false, note: "" };
    demoMemory.dupes = demoDupes;
    demoMemory.apps = demoApps;
    demoMemory.system = demoSystem;
    demoMemory.status = { running: false, ready: true, cancelled: false, files: demoRoot.files, allocated: demoRoot.allocated, note: "", error: "", where: "" };
    const summary: SmartSummary = { when: "刚刚", cancelled: false, completed: phases, cleanBytes: 4_100_000_000, cleanCount: demoClean.length, diskAllocated: demoRoot.allocated, diskFiles: demoRoot.files, dupeBytes: demoDupes.reclaimable, dupeGroups: demoDupes.groups.length, appCount: demoApps.apps.length, largeBytes: 25_100_000_000, largeCount: 2, systemCount: demoSystem.items.length, note: "扫描完成。选择一个模块继续处理，不会自动清理。" };
    demoMemory.smart = summary;
    emit("analyze:done", demoMemory.status);
    emit("task:done", summary);
  },
  Task: async () => demoTask,
  Snapshot: async () => ({ ...demoMemory, task: demoTask }),
  Places: async () => [
    { id: "downloads", name: "下载", path: "C:\\Users\\demo\\Downloads" },
    { id: "pictures", name: "图片", path: "C:\\Users\\demo\\Pictures" },
    { id: "videos", name: "视频", path: "C:\\Users\\demo\\Videos" },
    { id: "documents", name: "文档", path: "C:\\Users\\demo\\Documents" },
  ],
  Drives: async () => [{ drive: "C:", total: 1_000_000_000_000, used: 641_000_000_000, free: 359_000_000_000 }],
  OpenSetting: async () => {},
};

function goApp(): GoApp {
  if (demo) return demoApp;
  const w = window as unknown as { go?: { main?: { App?: GoApp } } };
  const app = w.go?.main?.App;
  if (!app) throw new Error("界面还没有连上 Go 后端");
  return app;
}

function runtime(): Runtime | undefined {
  return (window as unknown as { runtime?: Runtime }).runtime;
}

export const api = {
  volume: () => goApp().Volume(),
  start: () => goApp().Start(),
  cancel: () => goApp().Cancel(),
  status: () => goApp().Status(),
  level: (path: string) => goApp().Level(path),
  types: () => goApp().Types(),
  large: () => goApp().Large(),
  reveal: (path: string) => goApp().Reveal(path),
  scanCleanup: () => goApp().ScanCleanup(),
  runCleanup: (ids: string[], confirmation: string) => goApp().RunCleanup(ids, confirmation),
  findDuplicates: (query: DuplicateQuery) => goApp().FindDuplicates(query),
  recycleDuplicate: (path: string, confirmation: string) => goApp().RecycleDuplicate(path, confirmation),
  installedApps: () => goApp().InstalledApps(),
  launchUninstall: (name: string, confirmation: string) => goApp().LaunchUninstall(name, confirmation),
  readSystemTools: () => goApp().ReadSystemTools(),
  cleanupHistory: () => goApp().CleanupHistory(),
  cancelToolWork: () => goApp().CancelToolWork(),
  securityStatus: () => goApp().SecurityStatus(),
  relaunchElevated: (confirmation: string) => goApp().RelaunchElevated(confirmation),
  startSmart: () => goApp().StartSmart(),
  task: () => goApp().Task(),
  snapshot: () => goApp().Snapshot(),
  places: () => goApp().Places(),
  drives: () => goApp().Drives(),
  openSetting: (id: string) => goApp().OpenSetting(id),
  pickDirectory: async () => {
    const dialog = runtime()?.OpenDirectoryDialog;
    if (!dialog) return "";
    return dialog({ Title: "选择文件夹" });
  },
  on<T>(name: string, cb: (payload: T) => void) {
    if (demo) {
      const set = listeners.get(name) ?? new Set();
      const wrapped = cb as (payload: never) => void;
      set.add(wrapped);
      listeners.set(name, set);
      return () => set.delete(wrapped);
    }
    const rt = runtime();
    if (!rt) return () => {};
    return rt.EventsOn(name, cb as (payload: never) => void);
  },
};
