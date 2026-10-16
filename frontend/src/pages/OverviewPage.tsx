import { CircleDot, Copy, HardDrive, History, Package, Trash2 } from "lucide-react";
import { formatBytes } from "../format";
import type { Section } from "../state/model";
import { useStore } from "../state/store";

export function OverviewPage() {
  const { state, setSection, startSmart, cancelTask } = useStore();
  const volume = state.volume;
  const used = volume && volume.total > 0 ? (volume.used / volume.total) * 100 : 0;
  const last = state.history[0];
  const tiles = [
    { id: "clean" as Section, label: "垃圾候选", value: state.smart ? formatBytes(state.smart.cleanBytes) : "尚未扫描", note: state.smart ? `${state.smart.cleanCount} 个分类` : "只统计，不删除", icon: Trash2 },
    { id: "dupes" as Section, label: "重复文件", value: state.dupes ? formatBytes(state.dupes.reclaimable) : "尚未扫描", note: state.dupes ? `${state.dupes.groups.length} 组可释放` : "按内容确认", icon: Copy },
    { id: "analyze" as Section, label: "大文件", value: state.smart ? formatBytes(state.smart.largeBytes) : "尚未扫描", note: state.smart ? `${state.smart.largeCount} 个较大文件` : "来自磁盘分析", icon: CircleDot },
    { id: "apps" as Section, label: "已安装软件", value: state.apps.length ? String(state.apps.length) : "尚未读取", note: "使用登记的卸载程序", icon: Package },
  ];
  return (
    <div className="page-scroll overview">
      <section className="hero">
        <div>
          <p className="eyebrow">总览</p>
          <h1>{volume ? `${volume.drive.replace(":", "")} 盘还有 ${formatBytes(volume.free)}` : "正在读取系统盘"}</h1>
          <p className="lede">智能扫描只做只读分析。结束后由你决定进入哪个模块，不会自动清理。</p>
          <p className="meta">最近扫描：{state.smart?.when || "还没有"}{state.smart?.cancelled ? " · 部分结果" : ""}</p>
          <button className="btn primary hero-btn" type="button" onClick={() => (state.task.kind === "smart" && state.task.running ? cancelTask() : startSmart())}>
            {state.task.kind === "smart" && state.task.running ? `取消扫描 · ${state.task.phase || "准备"}` : "开始智能扫描"}
          </button>
        </div>
        <div className="hero-disk" aria-label="系统盘容量">
          <strong>{volume ? formatBytes(volume.used) : "—"}</strong>
          <span>已用 / {volume ? formatBytes(volume.total) : "—"}</span>
          <div className="meter" aria-hidden="true"><i style={{ width: `${used}%` }} /></div>
          <span>剩余 {volume ? formatBytes(volume.free) : "—"}</span>
        </div>
      </section>
      {state.task.kind === "smart" && state.task.running ? (
        <ol className="phases" aria-label="智能扫描阶段">
          {["垃圾候选项", "磁盘占用", "重复文件", "软件信息", "系统与开发环境"].map((phase, index) => {
            const current = state.task.phase === phase;
            const done = (state.smart?.completed ?? []).includes(phase) || ["垃圾候选项", "磁盘占用", "重复文件", "软件信息", "系统与开发环境"].indexOf(state.task.phase) > index;
            return <li key={phase} className={current ? "now" : done ? "done" : ""}><span>{index + 1}</span>{phase}</li>;
          })}
        </ol>
      ) : null}
      {state.smart?.note ? <p className={state.smart.cancelled ? "banner warn" : "banner"} role={state.smart.cancelled ? "status" : undefined}>{state.smart.note}</p> : null}
      <section className="tile-row">
        {tiles.map((tile) => (
          <button key={tile.id} className="tile" type="button" onClick={() => setSection(tile.id)}>
            <tile.icon size={18} aria-hidden="true" />
            <span>{tile.label}</span>
            <strong>{tile.value}</strong>
            <em>{tile.note}</em>
          </button>
        ))}
      </section>
      <section className="module-row">
        <button className="module" type="button" onClick={() => setSection("system")}>
          <HardDrive size={18} aria-hidden="true" />
          <strong>系统占用</strong>
          <span>{state.system ? `${state.system.items.length} 项只读占用` : "Windows、虚拟内存、Docker、WSL 与开发缓存"}</span>
        </button>
        <button className="module" type="button" onClick={() => setSection("history")}>
          <History size={18} aria-hidden="true" />
          <strong>最近一次清理</strong>
          <span>{last ? `${last.when} · 逻辑 ${last.logical} · 预计 ${last.estimated}` : "还没有清理记录"}</span>
        </button>
      </section>
    </div>
  );
}
