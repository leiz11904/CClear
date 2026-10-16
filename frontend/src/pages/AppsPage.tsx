import { useMemo, useState } from "react";
import { Modal } from "../components/Modal";
import { formatBytes } from "../format";
import { useStore } from "../state/store";

const kindLabel: Record<string, string> = { win32: "Win32", store: "Microsoft Store", system: "系统组件" };

export function AppsPage() {
  const { state, loadApps, setAppQuery, setAppSort, setAppPublisher, uninstallApp } = useStore();
  const [pending, setPending] = useState<{ id: string; name: string } | null>(null);
  const [started, setStarted] = useState("");
  const publishers = useMemo(() => Array.from(new Set(state.apps.map((app) => app.publisher).filter(Boolean))).sort((a, b) => a.localeCompare(b, "zh-CN")), [state.apps]);
  const rows = state.apps
    .filter((app) => {
      const query = state.appQuery.trim().toLocaleLowerCase();
      const text = `${app.name} ${app.publisher} ${app.location}`.toLocaleLowerCase();
      return (!query || text.includes(query)) && (!state.appPublisher || app.publisher === state.appPublisher);
    })
    .sort((a, b) => state.appSort === "size" ? (b.sizeKnown ? b.bytes : -1) - (a.sizeKnown ? a.bytes : -1) : a.name.localeCompare(b.name, "zh-CN"));
  return (
    <div className="page-scroll apps">
      <header className="page-head">
        <div>
          <p className="eyebrow">应用列表</p>
          <h1>软件管理</h1>
          <p className="lede">容量来自 Windows 登记，不是重新测量的目录大小。CClear 不会直接删除安装目录。</p>
        </div>
        <button className="btn primary" type="button" onClick={loadApps}>刷新软件列表</button>
      </header>
      {state.appNote ? <p className="banner">{state.appNote}</p> : null}
      {started ? <p className="banner" role="status">{started}</p> : null}
      <div className="app-tools">
        <input value={state.appQuery} onChange={(event) => setAppQuery(event.target.value)} aria-label="搜索软件" placeholder="搜索名称、发布者或位置" />
        <button type="button" className={state.appSort === "name" ? "chip on" : "chip"} aria-pressed={state.appSort === "name"} onClick={() => setAppSort("name")}>按名称</button>
        <button type="button" className={state.appSort === "size" ? "chip on" : "chip"} aria-pressed={state.appSort === "size"} onClick={() => setAppSort("size")}>按容量</button>
        <select aria-label="发布者" value={state.appPublisher} onChange={(event) => setAppPublisher(event.target.value)}>
          <option value="">全部发布者</option>
          {publishers.map((name) => <option key={name} value={name}>{name}</option>)}
        </select>
      </div>
      {state.apps.length === 0 ? <p className="lede">刷新后会区分 Win32、Microsoft Store 和系统组件。系统组件不提供卸载。</p> : null}
      <ul className="app-table">
        {rows.map((app) => (
          <li key={app.id}>
            <span className="app-mark" aria-hidden="true">{mark(app.name)}</span>
            <div>
              <strong>{app.name}</strong>
              <span>{kindLabel[app.kind] || "Win32"} · {app.version || "版本未知"} · {app.publisher || "发布者未知"}</span>
              <code>{app.location || app.note || "未提供位置"}</code>
            </div>
            <em>{app.sizeKnown ? formatBytes(app.bytes) : "大小未知"}</em>
            {app.canUninstall && app.kind !== "system" ? <button type="button" onClick={() => setPending({ id: app.id, name: app.name })}>卸载…</button> : <span className="keep">不提供卸载</span>}
          </li>
        ))}
      </ul>
      {pending ? (
        <Modal title={`卸载 ${pending.name}`} onClose={() => setPending(null)} footer={<><button className="btn" type="button" onClick={() => setPending(null)}>取消</button><button className="btn danger" type="button" onClick={() => { const item = pending; setPending(null); void uninstallApp(item.id).then(() => setStarted(`已启动“${item.name}”的卸载程序。它关闭之后，可以刷新列表。`)); }}>启动卸载程序</button></>}>
          <p>CClear 只启动 Windows 登记的卸载程序，不会直接删除安装目录。卸载程序退出后，用“刷新软件列表”查看结果。</p>
        </Modal>
      ) : null}
    </div>
  );
}

function mark(name: string): string {
  const text = name.trim();
  return text ? text.slice(0, 1).toUpperCase() : "?";
}
