import { useEffect, useState, type ReactNode } from "react";
import { ChevronRight, CircleDot, Files, LayoutGrid, PieChart } from "lucide-react";
import type { Entry, FileRow, TypeRow } from "../api";
import { Lens, RankList, TreeMap } from "../analyze/charts";
import { formatBytes, formatPercent, paletteFor } from "../format";
import { useStore } from "../state/store";

export function AnalyzePage() {
  const { state, theme, startAnalyze, cancelTask, openLevel, setMode, setHot, setSelected, reveal } = useStore();
  const [tip, setTip] = useState<{ x: number; y: number; entry: Entry } | null>(null);
  const { status, level, types, large, mode, hot, selected } = state.analyze;
  useEffect(() => { setTip(null); }, [level.path]);
  const colors = paletteFor(theme);
  const volume = state.volume;
  const scanned = volume && status?.ready && volume.total > 0 ? Math.min(100, (status.allocated / volume.total) * 100) : 0;
  const running = state.task.kind === "analyze" && state.task.running;
  return (
    <div className="analyze">
      <header className="an-top">
        <div className="an-stats">
          <span>总容量 <strong>{volume ? formatBytes(volume.total) : "—"}</strong></span>
          <span>已用 <strong>{volume ? formatBytes(volume.used) : "—"}</strong></span>
          <span>剩余 <strong>{volume ? formatBytes(volume.free) : "—"}</strong></span>
          <span>本次统计 <strong>{status?.ready ? formatBytes(status.allocated) : "—"}</strong></span>
        </div>
        <div className="meter slim" aria-hidden="true"><i style={{ width: `${scanned}%` }} /></div>
        <div className="an-actions">
          <div className="crumbs">
            {level.crumbs.length === 0 ? <span>分析完成后从占用最多的目录看起</span> : null}
            {level.crumbs.map((crumb, index) => (
              <span key={crumb.path}>
                {index > 0 ? <ChevronRight size={14} aria-hidden="true" /> : null}
                <button type="button" className={index === level.crumbs.length - 1 ? "crumb now" : "crumb"} onClick={() => openLevel(crumb.path)}>{crumb.name}</button>
              </span>
            ))}
          </div>
          <div className="modes" role="group" aria-label="查看方式">
            <Mode icon={<CircleDot size={15} />} label="气泡" on={mode === "lens"} onClick={() => setMode("lens")} />
            <Mode icon={<LayoutGrid size={15} />} label="矩形" on={mode === "map"} onClick={() => setMode("map")} />
            <Mode icon={<PieChart size={15} />} label="类型" on={mode === "types"} onClick={() => setMode("types")} />
            <Mode icon={<Files size={15} />} label="大文件" on={mode === "large"} onClick={() => setMode("large")} />
          </div>
          <button className="btn primary" type="button" onClick={() => (running ? cancelTask() : startAnalyze())}>{running ? "取消分析" : status?.ready ? "重新分析" : "开始分析"}</button>
        </div>
        {running ? <p className="status-line" aria-live="polite">{state.task.detail ? `正在分析 ${state.task.detail}` : "正在分析，不会删除任何文件"}</p> : null}
        {status?.cancelled ? <p className="banner warn" role="status">{status.note || "分析已取消，以下为部分结果。"}</p> : null}
        {status?.error ? <p className="banner warn" role="alert">部分目录未读取。{status.error.split("\n").length} 项</p> : null}
      </header>
      <div className="an-body">
        <aside className="an-list">
          <div className="side-head"><h2>目录</h2><span>{level.entries.length} 项</span></div>
          {level.entries.length === 0 ? <p className="empty">当前目录的子文件夹会列在这里。</p> : (
            <RankList entries={level.entries} colors={colors} hot={hot} selected={selected} onHot={setHot} onSelect={setSelected} onOpen={openLevel} />
          )}
          {level.capped ? <p className="fine">只保留占用最大的 40 个文件夹。</p> : null}
        </aside>
        <section className="an-map" aria-label="空间地图">
          {!status?.ready && !running ? <div className="empty map-empty"><p>气泡面积对应真实占用。开始分析后可以进入文件夹。</p></div> : null}
          {(status?.ready || level.entries.length > 0) && mode === "lens" ? <Lens entries={level.entries} colors={colors} hot={hot} selected={selected} onHot={setHot} onSelect={setSelected} onTip={setTip} onOpen={openLevel} /> : null}
          {(status?.ready || level.entries.length > 0) && mode === "map" ? <TreeMap entries={level.entries} colors={colors} hot={hot} selected={selected} onHot={setHot} onSelect={setSelected} onTip={setTip} onOpen={openLevel} /> : null}
          {mode === "types" ? <TypeList rows={types} /> : null}
          {mode === "large" ? <LargeList rows={large} onOpen={reveal} /> : null}
          {tip && (mode === "lens" || mode === "map") ? (
            <div className="tip" style={{ left: tip.x, top: tip.y }}>
              <strong>{tip.entry.name}</strong>
              <span>{formatBytes(tip.entry.allocated)} · {formatPercent(tip.entry.percent)}</span>
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}

function Mode({ icon, label, on, onClick }: { icon: ReactNode; label: string; on: boolean; onClick: () => void }) {
  return <button type="button" className={on ? "mode on" : "mode"} aria-pressed={on} onClick={onClick}>{icon}{label}</button>;
}

function TypeList({ rows }: { rows: TypeRow[] }) {
  if (rows.length === 0) return <div className="empty">类型合计会在分析完成后出现，不会加进顶部容量。</div>;
  const max = Math.max(...rows.map((row) => row.allocated), 1);
  return (
    <div className="sheet">
      <h2>文件类型</h2>
      <ul>{rows.map((row) => <li key={row.ext}><span>{row.ext}</span><b style={{ width: `${(row.allocated / max) * 100}%` }} /><em>{formatBytes(row.allocated)}</em></li>)}</ul>
    </div>
  );
}

function LargeList({ rows, onOpen }: { rows: FileRow[]; onOpen: (path: string) => Promise<void> }) {
  if (rows.length === 0) return <div className="empty">大文件会在分析完成后列在这里。定位只会打开资源管理器。</div>;
  return (
    <ul className="files">
      {rows.map((row) => (
        <li key={row.path}>
          <div><strong>{row.name}</strong><span>{row.path}</span></div>
          <em>{formatBytes(row.allocated)}</em>
          <button type="button" onClick={() => onOpen(row.path)}>定位</button>
        </li>
      ))}
    </ul>
  );
}
