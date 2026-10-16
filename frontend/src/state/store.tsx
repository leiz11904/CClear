import { createContext, useContext, useEffect, useMemo, useReducer, useRef, type ReactNode } from "react";
import { api, type SmartSummary, type Snapshot, type Status, type TaskState } from "../api";
import { initialState, reduce, type MapMode, type Scope, type Section, type State, type TaskKind } from "./model";

type Store = {
  state: State;
  setSection: (section: Section) => void;
  toggleTheme: () => void;
  theme: "light" | "dark";
  dismissConflict: () => void;
  cancelTask: () => void;
  startSmart: () => void;
  startAnalyze: () => void;
  openLevel: (path: string) => void;
  setMode: (mode: MapMode) => void;
  setHot: (path: string) => void;
  setSelected: (path: string) => void;
  scanCleanup: () => void;
  setCleanChecked: (id: string, on: boolean) => void;
  setCleanStage: (stage: State["cleanStage"]) => void;
  runCleanup: (ids: string[]) => Promise<void>;
  setDupeScope: (scope: Scope) => void;
  toggleDupe: (hash: string, fallback: boolean) => void;
  setDupeSort: (sort: "reclaim" | "count") => void;
  findDupes: () => void;
  recycleDupe: (path: string) => Promise<void>;
  loadApps: () => void;
  setAppQuery: (query: string) => void;
  setAppSort: (sort: "name" | "size") => void;
  setAppPublisher: (publisher: string) => void;
  uninstallApp: (id: string) => Promise<void>;
  loadSystem: () => void;
  loadHistory: () => void;
  reveal: (path: string) => Promise<void>;
  openSetting: (id: string) => Promise<void>;
  pickDirectory: () => Promise<string>;
  relaunchElevated: () => Promise<void>;
};

const Ctx = createContext<Store | null>(null);

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function StoreProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reduce, undefined, initialState);
  const [theme, setTheme] = useReducer((value: "light" | "dark") => (value === "dark" ? "light" : "dark"), document.documentElement.dataset.theme === "dark" ? "dark" : "light");
  const stateRef = useRef(state);
  stateRef.current = state;

  useEffect(() => {
    document.documentElement.dataset.theme = theme === "dark" ? "dark" : "";
    localStorage.setItem("cclear-theme", theme);
  }, [theme]);

  async function hydrate(snap: Snapshot) {
    if (snap.clean.items?.length || snap.clean.cancelled || snap.clean.note) {
      dispatch({ type: "clean-scanned", items: snap.clean.items ?? [], note: snap.clean.note, cancelled: snap.clean.cancelled });
    }
    if (snap.dupes?.note || snap.dupes?.groups?.length || snap.dupes?.cancelled) dispatch({ type: "dupes", report: snap.dupes });
    if (snap.apps?.apps?.length || snap.apps?.note) dispatch({ type: "apps", rows: snap.apps.apps ?? [], note: snap.apps.note });
    if (snap.system?.items?.length || snap.system?.note) dispatch({ type: "system", report: snap.system });
    if (snap.smart?.when || snap.smart?.note) dispatch({ type: "smart", summary: snap.smart });
    if (snap.status?.ready) await refreshAnalyze(snap.status);
  }

  async function refreshAnalyze(status?: Status) {
    const [level, types, large, next] = await Promise.all([api.level(stateRef.current.analyze.level.path || ""), api.types(), api.large(), status ? Promise.resolve(status) : api.status()]);
    dispatch({ type: "analyze", status: next, level, types: types ?? [], large: large ?? [] });
  }

  useEffect(() => {
    api.volume().then((volume) => dispatch({ type: "volume", volume })).catch((error: unknown) => dispatch({ type: "conflict", message: messageOf(error) }));
    api.securityStatus().then((status) => dispatch({ type: "elevated", elevated: status.elevated })).catch(() => {});
    api.places().then((places) => dispatch({ type: "places", places: places ?? [] })).catch(() => {});
    api.drives().then((drives) => {
      dispatch({ type: "drives", drives: drives ?? [] });
      if ((drives ?? []).length > 0 && stateRef.current.dupeScope.roots.length === 0) {
        dispatch({ type: "dupe-scope", scope: { roots: [drives[0].drive + "\\"], exclude: [], place: "" } });
      }
    }).catch(() => {});
    api.cleanupHistory().then((history) => dispatch({ type: "history", history: history ?? [] })).catch(() => {});
    api.snapshot().then((snap) => hydrate(snap)).catch(() => {});
    const offProgress = api.on<TaskState>("task:progress", (task) => dispatch({ type: "task-progress", task: { running: true, kind: task.kind as TaskKind, label: task.label, phase: task.phase, detail: task.detail } }));
    const offDone = api.on<SmartSummary>("task:done", (summary) => {
      dispatch({ type: "smart", summary });
      api.snapshot().then((snap) => hydrate({ ...snap, smart: summary })).catch(() => {});
      api.cleanupHistory().then((history) => dispatch({ type: "history", history: history ?? [] })).catch(() => {});
    });
    const offAnalyze = api.on<{ files: number; where: string }>("analyze:progress", (payload) => {
      const prev = stateRef.current.analyze.status;
      dispatch({
        type: "analyze-status",
        status: { running: true, ready: prev?.ready ?? false, cancelled: false, files: payload.files, allocated: prev?.allocated ?? 0, note: prev?.note ?? "", error: "", where: payload.where },
      });
    });
    const offAnalyzeDone = api.on<Status>("analyze:done", (status) => {
      refreshAnalyze(status).catch((error: unknown) => dispatch({ type: "conflict", message: messageOf(error) }));
    });
    return () => {
      offProgress();
      offDone();
      offAnalyze();
      offAnalyzeDone();
    };
  }, []);

  async function runHeavy(kind: TaskKind, label: string, work: () => Promise<void>, hold = false) {
    if (stateRef.current.task.running) {
      dispatch({ type: "task-started", kind, label });
      return;
    }
    dispatch({ type: "task-started", kind, label });
    try {
      await work();
      if (!hold && stateRef.current.task.kind === kind) dispatch({ type: "task-idle" });
    } catch (error) {
      dispatch({ type: "conflict", message: messageOf(error) });
      if (stateRef.current.task.kind === kind) dispatch({ type: "task-idle" });
    }
  }

  const store = useMemo<Store>(() => ({
    state,
    theme,
    setSection: (section) => dispatch({ type: "section", section }),
    toggleTheme: () => setTheme(),
    dismissConflict: () => dispatch({ type: "dismiss-conflict" }),
    cancelTask: () => {
      api.cancel().catch(() => {});
      api.cancelToolWork().catch(() => {});
    },
    startSmart: () => { void runHeavy("smart", "智能扫描", () => api.startSmart(), true); },
    startAnalyze: () => { void runHeavy("analyze", "磁盘分析", () => api.start(), true); },
    openLevel: (path) => { api.level(path).then((level) => dispatch({ type: "level", level })).catch((error: unknown) => dispatch({ type: "conflict", message: messageOf(error) })); },
    setMode: (mode) => dispatch({ type: "mode", mode }),
    setHot: (path) => dispatch({ type: "hot", path }),
    setSelected: (path) => dispatch({ type: "selected", path }),
    scanCleanup: () => {
      void runHeavy("clean", "垃圾扫描", async () => {
        const result = await api.scanCleanup();
        dispatch({ type: "clean-scanned", items: result.items ?? [], note: result.note, cancelled: result.cancelled });
      });
    },
    setCleanChecked: (id, on) => dispatch({ type: "clean-check", id, on }),
    setCleanStage: (stage) => dispatch({ type: "clean-stage", stage }),
    runCleanup: async (ids) => {
      await runHeavy("clean-run", "垃圾清理", async () => {
        const result = await api.runCleanup(ids, "确认清理");
        dispatch({ type: "clean-result", result });
        const history = await api.cleanupHistory().catch(() => stateRef.current.history);
        dispatch({ type: "history", history: history ?? [] });
      });
    },
    setDupeScope: (scope) => dispatch({ type: "dupe-scope", scope }),
    toggleDupe: (hash, fallback) => dispatch({ type: "dupe-toggle", hash, fallback }),
    setDupeSort: (sort) => dispatch({ type: "dupe-sort", sort }),
    findDupes: () => {
      const scope = stateRef.current.dupeScope;
      if (scope.roots.length === 0) {
        dispatch({ type: "conflict", message: "请先选择要扫描的磁盘或目录。" });
        return;
      }
      void runHeavy("dupes", "重复文件", async () => {
        const report = await api.findDuplicates({ roots: scope.roots, exclude: scope.exclude });
        dispatch({ type: "dupes", report });
      });
    },
    recycleDupe: async (path) => {
      await api.recycleDuplicate(path, "移到回收站");
      const current = stateRef.current.dupes;
      if (!current) return;
      const groups = current.groups
        .map((group) => ({ ...group, files: group.files.filter((file) => file.path.toLocaleLowerCase() !== path.toLocaleLowerCase()) }))
        .filter((group) => group.files.length >= 2);
      dispatch({
        type: "dupes",
        report: { ...current, groups, reclaimable: groups.reduce((total, group) => total + group.bytes * (group.files.length - 1), 0) },
      });
    },
    loadApps: () => {
      void runHeavy("apps", "软件信息", async () => {
        const list = await api.installedApps();
        dispatch({ type: "apps", rows: list.apps ?? [], note: list.note });
      });
    },
    setAppQuery: (query) => dispatch({ type: "app-query", query }),
    setAppSort: (sort) => dispatch({ type: "app-sort", sort }),
    setAppPublisher: (publisher) => dispatch({ type: "app-publisher", publisher }),
    uninstallApp: async (id) => { await api.launchUninstall(id, "启动卸载"); },
    loadSystem: () => {
      void runHeavy("system", "系统占用", async () => {
        dispatch({ type: "system", report: await api.readSystemTools() });
      });
    },
    loadHistory: () => { api.cleanupHistory().then((history) => dispatch({ type: "history", history: history ?? [] })).catch((error: unknown) => dispatch({ type: "conflict", message: messageOf(error) })); },
    reveal: async (path) => { await api.reveal(path); },
    openSetting: async (id) => { await api.openSetting(id); },
    pickDirectory: () => api.pickDirectory(),
    relaunchElevated: async () => { await api.relaunchElevated("管理员重启"); },
  }), [state, theme]);

  return <Ctx.Provider value={store}>{children}</Ctx.Provider>;
}

export function useStore() {
  const store = useContext(Ctx);
  if (!store) throw new Error("Store 尚未就绪");
  return store;
}
