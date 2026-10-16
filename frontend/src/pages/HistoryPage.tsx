import { useStore } from "../state/store";

export function HistoryPage() {
  const { state, loadHistory } = useStore();
  return (
    <div className="page-scroll history">
      <header className="page-head">
        <div>
          <p className="eyebrow">时间线</p>
          <h1>清理历史</h1>
          <p className="lede">每条记录同时保留逻辑大小和预计释放，方便对照实际结果。</p>
        </div>
        <button className="btn primary" type="button" onClick={loadHistory}>刷新历史</button>
      </header>
      {state.history.length === 0 ? <p className="lede">还没有清理记录。完成一次确认后的清理后，会出现在这条时间线上。</p> : null}
      <ol className="timeline">
        {state.history.map((item) => (
          <li key={item.when + item.version}>
            <span className="dot" aria-hidden="true" />
            <div>
              <strong>{item.when}</strong>
              <p>{item.items.join("、") || "未记录项目"} · CClear {item.version}</p>
              <p>逻辑 {item.logical || "—"} · 预计 {item.estimated || "—"}</p>
              <p>删除 {item.deleted} · 跳过 {item.skipped} · 失败 {item.failed}</p>
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}
