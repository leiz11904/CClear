import { describe, expect, it } from "vitest";
import type { CleanItem, CleanRunResult } from "../api";
import { initialChecks, initialState, reduce } from "./model";

const item: CleanItem = {
  id: "temp",
  name: "临时文件",
  description: "",
  group: "regular",
  warning: "",
  note: "",
  risk: 1,
  count: 2,
  bytes: 100,
  unknown: false,
  defaultOn: true,
  needsAdmin: false,
  truncated: false,
  errors: [],
  preview: [],
};

const advanced: CleanItem = { ...item, id: "dism", name: "组件存储", risk: 3, defaultOn: true, bytes: -1, unknown: true };

function scanned() {
  return reduce(initialState(), { type: "clean-scanned", items: [item, advanced], note: "", cancelled: false });
}

describe("page state", () => {
  it("keeps scan results when the page changes", () => {
    let state = scanned();
    state = reduce(state, { type: "dupes", report: { groups: [], reclaimable: 42, note: "保留", errors: ["denied"], truncated: true, cancelled: false } });
    state = reduce(state, { type: "apps", rows: [{ id: "1", name: "App", version: "", publisher: "", location: "", note: "", bytes: 1, sizeKnown: true, kind: "win32", canUninstall: true }], note: "" });
    state = reduce(state, { type: "section", section: "analyze" });
    state = reduce(state, { type: "section", section: "clean" });
    expect(state.cleanItems.map((row) => row.id)).toEqual(["temp", "dism"]);
    expect(state.dupes?.reclaimable).toBe(42);
    expect(state.apps).toHaveLength(1);
    expect(state.section).toBe("clean");
  });

  it("leaves high-risk cleanup items off", () => {
    expect(initialChecks([item, advanced])).toEqual({ temp: true, dism: false });
  });
});

describe("tasks", () => {
  it("refuses a second heavy task and keeps the first result", () => {
    let state = scanned();
    state = reduce(state, { type: "task-started", kind: "dupes", label: "重复文件" });
    const next = reduce(state, { type: "task-started", kind: "analyze", label: "磁盘分析" });
    expect(next.task.kind).toBe("dupes");
    expect(next.task.running).toBe(true);
    expect(next.conflict).toContain("重复文件");
    expect(next.cleanItems).toHaveLength(2);
  });

  it("keeps a partial cleanup result after cancel", () => {
    const result: CleanRunResult = {
      freed: 10,
      estimated: 8,
      deleted: 1,
      skipped: 2,
      failed: 1,
      errors: ["正在使用"],
      logPath: "C:\\logs\\clean.log",
      cancelled: true,
      partial: true,
    };
    let state = scanned();
    state = reduce(state, { type: "task-started", kind: "clean-run", label: "垃圾清理" });
    state = reduce(state, { type: "clean-result", result });
    expect(state.task.running).toBe(false);
    expect(state.cleanResult?.partial).toBe(true);
    expect(state.cleanResult?.deleted).toBe(1);
    expect(state.cleanResult?.logPath).toContain("clean.log");
    expect(state.cleanItems).toHaveLength(2);
    expect(state.cleanStage).toBe("report");
  });
});
