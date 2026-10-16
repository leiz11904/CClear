import { CircleDot, Copy, HardDrive, History, LayoutDashboard, Moon, Package, Sun, Trash2 } from "lucide-react";
import logoUrl from "../assets/cclear-logo-v3.png";
import { useStore } from "../state/store";
import type { Section } from "../state/model";
import { OverviewPage } from "../pages/OverviewPage";
import { CleanupPage } from "../pages/CleanupPage";
import { AnalyzePage } from "../pages/AnalyzePage";
import { DupesPage } from "../pages/DupesPage";
import { AppsPage } from "../pages/AppsPage";
import { SystemPage } from "../pages/SystemPage";
import { HistoryPage } from "../pages/HistoryPage";

const items: Array<[Section, string, typeof Trash2]> = [
  ["overview", "总览", LayoutDashboard],
  ["clean", "垃圾清理", Trash2],
  ["analyze", "磁盘分析", CircleDot],
  ["dupes", "重复文件", Copy],
  ["apps", "软件管理", Package],
  ["system", "系统占用", HardDrive],
  ["history", "清理历史", History],
];

export function Shell() {
  const { state, setSection, theme, toggleTheme, cancelTask, dismissConflict } = useStore();
  const page = {
    overview: <OverviewPage />,
    clean: <CleanupPage />,
    analyze: <AnalyzePage />,
    dupes: <DupesPage />,
    apps: <AppsPage />,
    system: <SystemPage />,
    history: <HistoryPage />,
  }[state.section];
  return (
    <div className="app">
      <aside className="rail">
        <div className="brand">
          <img className="mark" src={logoUrl} alt="" />
          <div>
            <strong>CClear</strong>
            <em>系统空间管理</em>
          </div>
        </div>
        <nav aria-label="功能">
          {items.map(([id, label, Icon]) => (
            <button key={id} className={state.section === id ? "nav on" : "nav"} type="button" aria-current={state.section === id ? "page" : undefined} onClick={() => setSection(id)}>
              <Icon size={16} aria-hidden="true" />
              {label}
            </button>
          ))}
        </nav>
        <button className="theme" type="button" onClick={toggleTheme} aria-pressed={theme === "dark"}>
          {theme === "dark" ? <Sun size={16} aria-hidden="true" /> : <Moon size={16} aria-hidden="true" />}
          {theme === "dark" ? "浅色" : "深色"}
        </button>
      </aside>
      <div className="stage">
        <div className="taskbar" aria-live="polite">
          {state.task.running ? (
            <p>
              <span className="pulse" aria-hidden="true" />
              正在执行{state.task.label}
              {state.task.phase ? ` · ${state.task.phase}` : ""}
              {state.task.detail ? ` · ${state.task.detail}` : ""}
              <button type="button" onClick={cancelTask}>取消任务</button>
            </p>
          ) : null}
        </div>
        {state.conflict ? (
          <div className="conflict" role="alert">
            <span>{state.conflict}</span>
            <button type="button" onClick={dismissConflict}>知道了</button>
          </div>
        ) : null}
        <div className="page">{page}</div>
      </div>
    </div>
  );
}
