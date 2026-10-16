import { useState } from "react";
import { systemOrder } from "../state/model";
import { formatBytes } from "../format";
import { useStore } from "../state/store";

export function SystemPage() {
  const { state, loadSystem, reveal, openSetting } = useStore();
  const [open, setOpen] = useState("");
  const items = state.system?.items ?? [];
  return (
    <div className="page-scroll system">
      <header className="page-head">
        <div>
          <p className="eyebrow">只读</p>
          <h1>系统占用</h1>
          <p className="lede">这里展示 Windows、虚拟内存、Docker、WSL 和开发环境占用。不会删除系统文件。</p>
        </div>
        <button className="btn primary" type="button" onClick={loadSystem}>读取占用</button>
      </header>
      {state.system?.note ? <p className="banner">{state.system.note}</p> : null}
      {!state.system ? <p className="lede">读取完成后，每一项都可以查看位置和说明。能打开 Windows 设置的项目会给出入口。</p> : null}
      {systemOrder.map((group) => {
        const rows = items.filter((item) => item.group === group);
        if (rows.length === 0) return null;
        return (
          <section key={group} className="sys-group">
            <h2>{group}</h2>
            {rows.map((item) => {
              const id = item.path + item.name;
              return (
                <article key={id} className="sys-row">
                  <div>
                    <strong>{item.name}</strong>
                    <span>{item.kind} · {item.status}</span>
                    <code>{item.path}</code>
                  </div>
                  <em>{item.known && item.bytes >= 0 ? formatBytes(item.bytes) : "大小见说明"}</em>
                  <div className="sys-actions">
                    <button type="button" onClick={() => reveal(item.path)}>查看位置</button>
                    <button type="button" aria-expanded={open === id} onClick={() => setOpen(open === id ? "" : id)}>查看说明</button>
                    {item.setting ? <button type="button" onClick={() => openSetting(item.setting)}>打开设置</button> : null}
                  </div>
                  {open === id ? <p className="sys-note">{item.note}</p> : null}
                </article>
              );
            })}
          </section>
        );
      })}
    </div>
  );
}
