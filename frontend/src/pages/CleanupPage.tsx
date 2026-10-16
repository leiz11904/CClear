import { useState } from "react";
import type { CleanItem } from "../api";
import { Modal } from "../components/Modal";
import { formatBytes } from "../format";
import { cleanBuckets } from "../state/model";
import { useStore } from "../state/store";

const steps = ["扫描", "摘要", "审核", "确认", "结果"];

export function CleanupPage() {
  const { state, scanCleanup, setCleanChecked, setCleanStage, runCleanup, reveal, relaunchElevated } = useStore();
  const [confirming, setConfirming] = useState(false);
  const [elevate, setElevate] = useState(false);
  const buckets = cleanBuckets(state.cleanItems);
  const selected = state.cleanItems.filter((item) => state.cleanChecked[item.id]);
  const total = selected.reduce((sum, item) => sum + Math.max(0, item.bytes), 0);
  const needsAdmin = selected.some((item) => item.needsAdmin) && !state.elevated;
  const stageIndex = state.cleanStage === "idle" ? 0 : state.cleanStage === "summary" ? 1 : state.cleanStage === "review" ? 2 : 4;
  return (
    <div className="page-scroll cleanup">
      <header className="page-head">
        <div>
          <p className="eyebrow">可释放空间</p>
          <h1>{state.cleanStage === "idle" ? "先看清楚再清理" : formatBytes(total)}</h1>
          <p className="lede">扫描只列出规则内的候选项。高风险项目默认关闭，确认后才会执行。</p>
        </div>
        <button className="btn primary" type="button" onClick={scanCleanup}>扫描垃圾</button>
      </header>
      <ol className="stepper" aria-label="清理步骤">
        {steps.map((step, index) => <li key={step} className={index === stageIndex ? "now" : index < stageIndex ? "done" : ""}><span>{index + 1}</span>{step}</li>)}
      </ol>
      {state.cleanCancelled || state.cleanNote ? <p className="banner warn" role="status">{state.cleanNote || "扫描已取消，以下为部分结果。"}</p> : null}
      {state.cleanStage === "idle" ? (
        <section className="flow">
          <article><strong>推荐</strong><p>临时文件和可重建缓存，默认可以清理。</p></article>
          <article><strong>谨慎</strong><p>回收站等内容默认关闭，需要你单独打开。</p></article>
          <article><strong>高级系统项目</strong><p>可能需要管理员权限，默认关闭，且不会静默执行。</p></article>
        </section>
      ) : null}
      {state.cleanStage === "summary" ? (
        <section className="summary-board">
          <Summary title="推荐" rows={buckets.recommended} />
          <Summary title="谨慎" rows={buckets.caution} />
          <Summary title="高级系统项目" rows={buckets.advanced} />
          <button className="btn primary" type="button" onClick={() => setCleanStage("review")}>审核清理内容</button>
        </section>
      ) : null}
      {state.cleanStage === "review" || state.cleanStage === "report" ? (
        <div className="review">
          <Group title="推荐" hint="适合大多数人" rows={buckets.recommended} checked={state.cleanChecked} onCheck={setCleanChecked} />
          <Group title="谨慎" hint="删除后可能无法从回收站恢复" rows={buckets.caution} checked={state.cleanChecked} onCheck={setCleanChecked} />
          <Group title="高级系统项目" hint="默认关闭。管理员项目会标明权限要求" rows={buckets.advanced} checked={state.cleanChecked} onCheck={setCleanChecked} />
          {state.cleanStage === "review" ? (
            <div className="review-bar">
              {needsAdmin ? <button className="btn" type="button" onClick={() => setElevate(true)}>管理员重启</button> : null}
              <button className="btn primary" type="button" disabled={selected.length === 0 || needsAdmin} onClick={() => setConfirming(true)}>
                审核所选 · {formatBytes(total)}
              </button>
            </div>
          ) : null}
        </div>
      ) : null}
      {state.cleanStage === "report" && state.cleanResult ? (
        <section className="report" aria-live="polite">
          <h2>{state.cleanResult.partial || state.cleanResult.cancelled ? "部分完成" : "清理结果"}</h2>
          <dl>
            <div><dt>释放</dt><dd>{formatBytes(state.cleanResult.freed)}</dd></div>
            <div><dt>预计</dt><dd>{formatBytes(state.cleanResult.estimated)}</dd></div>
            <div><dt>删除</dt><dd>{state.cleanResult.deleted}</dd></div>
            <div><dt>跳过</dt><dd>{state.cleanResult.skipped}</dd></div>
            <div><dt>失败</dt><dd>{state.cleanResult.failed}</dd></div>
          </dl>
          {state.cleanResult.errors?.length ? <ul className="issues" role="alert">{state.cleanResult.errors.map((error) => <li key={error}>{error}</li>)}</ul> : null}
          {state.cleanResult.logPath ? <button className="btn" type="button" onClick={() => reveal(state.cleanResult!.logPath)}>打开清理日志</button> : null}
        </section>
      ) : null}
      {confirming && state.cleanStage !== "report" ? (
        <Modal
          title="确认清理"
          onClose={() => setConfirming(false)}
          footer={
            <>
              <button className="btn" type="button" onClick={() => setConfirming(false)}>返回审核</button>
              <button className="btn danger" type="button" onClick={() => runCleanup(selected.map((item) => item.id))}>开始清理</button>
            </>
          }
        >
          <p>将清理 {selected.length} 项，预计释放 {formatBytes(total)}。这一步会删除所选内容，通常无法恢复。</p>
          <ul>{selected.map((item) => <li key={item.id}>{item.name} · {item.unknown ? "待系统分析" : formatBytes(item.bytes)} · {item.risk >= 3 ? "高级" : item.risk === 2 ? "谨慎" : "推荐"}{item.needsAdmin ? " · 需要管理员" : ""}</li>)}</ul>
        </Modal>
      ) : null}
      {elevate ? (
        <Modal title="请求管理员权限" onClose={() => setElevate(false)} footer={<><button className="btn" type="button" onClick={() => setElevate(false)}>取消</button><button className="btn primary" type="button" onClick={() => relaunchElevated()}>重新启动</button></>}>
          <p>将重新启动 CClear，并由 Windows 询问管理员权限。这不会修改系统安全设置。</p>
        </Modal>
      ) : null}
    </div>
  );
}

function Summary({ title, rows }: { title: string; rows: CleanItem[] }) {
  const bytes = rows.reduce((sum, row) => sum + Math.max(0, row.bytes), 0);
  const problems = rows.reduce((sum, row) => sum + row.errors.length, 0);
  return (
    <article>
      <span>{title}</span>
      <strong>{formatBytes(bytes)}</strong>
      <em>{rows.length} 类{problems ? ` · ${problems} 条读取问题` : ""}{rows.some((row) => row.truncated) ? " · 有截断" : ""}</em>
    </article>
  );
}

function Group({ title, hint, rows, checked, onCheck }: { title: string; hint: string; rows: CleanItem[]; checked: Record<string, boolean>; onCheck: (id: string, on: boolean) => void }) {
  if (rows.length === 0) return null;
  return (
    <section className="bucket">
      <header><h2>{title}</h2><p>{hint}</p></header>
      {rows.map((row) => (
        <article key={row.id} className={checked[row.id] ? "clean-row on" : "clean-row"}>
          <label>
            <input type="checkbox" checked={!!checked[row.id]} onChange={(event) => onCheck(row.id, event.target.checked)} />
            <span>
              <strong>{row.name}</strong>
              <em>{row.risk >= 3 ? "高级系统项目" : row.risk === 2 ? "谨慎" : "推荐"} · {row.count.toLocaleString("zh-CN")} 项{row.needsAdmin ? " · 需要管理员权限" : ""}{row.truncated ? " · 结果已截断" : ""}</em>
            </span>
            <b>{row.unknown ? "待系统分析" : formatBytes(row.bytes)}</b>
          </label>
          <p>{row.description}</p>
          {row.warning ? <p className="warning">{row.warning}</p> : null}
          {row.note ? <p>{row.note}</p> : null}
          {row.errors?.length ? <ul className="issues" role="alert">{row.errors.map((error) => <li key={error}>{error}</li>)}</ul> : null}
          {row.preview?.length ? <details><summary>查看路径</summary>{row.preview.map((path) => <code key={path}>{path}</code>)}</details> : null}
        </article>
      ))}
    </section>
  );
}
