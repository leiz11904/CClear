import { useState } from "react";
import { Modal } from "../components/Modal";
import { formatBytes } from "../format";
import { reclaimOf, sortGroups } from "../state/model";
import { useStore } from "../state/store";

export function DupesPage() {
  const { state, setDupeScope, toggleDupe, setDupeSort, findDupes, recycleDupe, pickDirectory, reveal } = useStore();
  const [exclude, setExclude] = useState("");
  const [custom, setCustom] = useState("");
  const [pending, setPending] = useState("");
  const scope = state.dupeScope;
  const report = state.dupes;
  const groups = report ? sortGroups(report.groups, state.dupeSort) : [];
  function choosePlace(id: string, path: string) {
    setDupeScope({ roots: [path], exclude: scope.exclude, place: id });
  }
  function toggleDrive(path: string) {
    const roots = scope.roots.includes(path) ? scope.roots.filter((item) => item !== path) : [...scope.roots, path];
    setDupeScope({ ...scope, roots, place: "" });
  }
  return (
    <div className="dupes">
      <aside className="scope">
        <p className="eyebrow">扫描范围</p>
        <h1>重复文件</h1>
        <p className="lede">先选定磁盘或目录。系统目录、虚拟磁盘和受保护文件不能删除，副本只移入回收站。</p>
        <div className="places">
          {state.places.map((place) => (
            <button key={place.id} type="button" className={scope.place === place.id ? "chip on" : "chip"} aria-pressed={scope.place === place.id} onClick={() => choosePlace(place.id, place.path)}>{place.name}</button>
          ))}
        </div>
        <div className="drive-picks">
          {state.drives.map((drive) => {
            const path = `${drive.drive}\\`;
            const on = scope.roots.includes(path);
            return <button key={drive.drive} type="button" className={on ? "chip on" : "chip"} aria-pressed={on} onClick={() => toggleDrive(path)}>{drive.drive}</button>;
          })}
        </div>
        <label className="field">目录
          <input value={custom} onChange={(event) => setCustom(event.target.value)} aria-label="自定义扫描目录" placeholder="C:\\Users\\你的目录" />
        </label>
        <button className="btn" type="button" onClick={async () => { const picked = await pickDirectory(); if (picked) setDupeScope({ roots: [picked], exclude: scope.exclude, place: "custom" }); }}>选择目录</button>
        <button className="btn" type="button" onClick={() => { if (custom.trim()) setDupeScope({ roots: [custom.trim()], exclude: scope.exclude, place: "custom" }); }}>使用上面的路径</button>
        <label className="field">排除目录
          <input value={exclude} onChange={(event) => setExclude(event.target.value)} aria-label="排除目录" placeholder="不扫描的路径" />
        </label>
        <button className="btn" type="button" onClick={() => { if (!exclude.trim()) return; setDupeScope({ ...scope, exclude: [...scope.exclude, exclude.trim()] }); setExclude(""); }}>添加排除</button>
        <ul className="exclude">{scope.exclude.map((path) => <li key={path}><span>{path}</span><button type="button" onClick={() => setDupeScope({ ...scope, exclude: scope.exclude.filter((item) => item !== path) })}>移除</button></li>)}</ul>
      </aside>
      <section className="dupe-main">
        <header className="page-head">
          <div>
            <strong>将扫描</strong>
            <p>{scope.roots.length ? scope.roots.join("、") : "尚未选择"}{scope.exclude.length ? `；排除 ${scope.exclude.length} 个目录` : ""}</p>
          </div>
          <button className="btn primary" type="button" onClick={findDupes}>查找重复</button>
        </header>
        {state.task.kind === "dupes" && state.task.running ? (
          <ol className="phases" aria-live="polite">
            {["枚举", "候选筛选", "内容哈希"].map((phase) => <li key={phase} className={state.task.detail.includes(phase) ? "now" : ""}><span />{phase}</li>)}
          </ol>
        ) : null}
        {report?.cancelled ? <p className="banner warn" role="status">{report.note || "查找已取消，以下为部分结果。"}</p> : null}
        {report && !report.cancelled && report.note ? <p className="banner">{report.note}</p> : null}
        {report?.truncated ? <p className="banner warn" role="status">候选数量已截断，结果不是完整集合。</p> : null}
        {report?.errors?.length ? <ul className="issues" role="alert">{report.errors.map((error) => <li key={error}>{error}</li>)}</ul> : null}
        {report ? (
          <div className="dupe-tools">
            <span>可释放约 {formatBytes(report.reclaimable)} · {report.groups.length} 组</span>
            <button type="button" className={state.dupeSort === "reclaim" ? "chip on" : "chip"} aria-pressed={state.dupeSort === "reclaim"} onClick={() => setDupeSort("reclaim")}>按可释放空间</button>
            <button type="button" className={state.dupeSort === "count" ? "chip on" : "chip"} aria-pressed={state.dupeSort === "count"} onClick={() => setDupeSort("count")}>按副本数</button>
          </div>
        ) : <p className="lede">扫描会经过枚举、候选筛选和内容哈希。一组里至少会留下一份。</p>}
        <div className="dupe-groups">
          {groups.map((group, index) => {
            const open = Object.prototype.hasOwnProperty.call(state.dupeOpen, group.hash) ? state.dupeOpen[group.hash] : index < 2;
            return (
              <article key={group.hash} className="group">
                <button type="button" className="group-toggle" aria-expanded={open} onClick={() => toggleDupe(group.hash, index < 2)}>
                  <strong>重复组 {index + 1}</strong>
                  <span>可释放 {formatBytes(reclaimOf(group))} · {group.files.length} 个副本</span>
                </button>
                {open ? group.files.map((file, fileIndex) => (
                  <div className="dupe-file" key={file.path}>
                    <div>
                      <b>{fileIndex === 0 ? "默认保留" : "副本"}</b>
                      <code title={file.path}>{file.path}</code>
                    </div>
                    <em>{formatBytes(file.bytes)}</em>
                    <button type="button" onClick={() => reveal(file.path)}>定位</button>
                    <button type="button" disabled={group.files.length <= 1} onClick={() => setPending(file.path)}>移到回收站</button>
                  </div>
                )) : null}
              </article>
            );
          })}
        </div>
      </section>
      {pending ? (
        <Modal title="移到回收站" onClose={() => setPending("")} footer={<><button className="btn" type="button" onClick={() => setPending("")}>取消</button><button className="btn danger" type="button" onClick={() => { const path = pending; setPending(""); void recycleDupe(path); }}>移到回收站</button></>}>
          <p>文件会移入回收站，不会直接永久删除。这一组会至少保留一份。</p>
          <code>{pending}</code>
        </Modal>
      ) : null}
    </div>
  );
}
