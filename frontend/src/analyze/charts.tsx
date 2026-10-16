import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent, type ReactNode } from "react";
import { ChevronRight, Folder } from "lucide-react";
import type { Entry } from "../api";
import { formatBytes, formatPercent, inkOn } from "../format";

export const plotW = 1000;
export const plotH = 640;

function blendHex(hex: string, target: string, amount: number): string {
  const parse = (value: string) => [1, 3, 5].map((start) => parseInt(value.slice(start, start + 2), 16));
  const [r, g, b] = parse(hex);
  const [tr, tg, tb] = parse(target);
  const mix = (from: number, to: number) => Math.round(from + (to - from) * amount).toString(16).padStart(2, "0");
  return `#${mix(r, tr)}${mix(g, tg)}${mix(b, tb)}`;
}

function textFits(text: string, fontSize: number, width: number): boolean {
  return text.length * fontSize * 0.62 <= width;
}

type Mark = {
  key: string;
  name: string;
  size: string;
  x: number;
  y: number;
  span: number;
  wide: number;
  align: "center" | "left";
  fill: string;
  icon: boolean;
};

export function PlotStage({ label, marks, children }: { label: string; marks: Mark[]; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState({ w: 0, h: 0 });
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => {
      const rect = el.getBoundingClientRect();
      setBox({ w: rect.width, h: rect.height });
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  const scale = box.w > 0 && box.h > 0 ? Math.min(box.w / plotW, box.h / plotH) : 0;
  const ox = (box.w - plotW * scale) / 2;
  const oy = (box.h - plotH * scale) / 2;
  return (
    <div className="plot-wrap" ref={ref}>
      <svg className="plot" viewBox={`0 0 ${plotW} ${plotH}`} role="img" aria-label={label}>{children}</svg>
      {scale > 0
        ? marks.map((mark) => {
            const px = mark.span * scale;
            if (px < 46) return null;
            const big = px >= 92;
            const nameSize = big ? 16 : 13;
            const needed = mark.name.length * nameSize * 0.62;
            const boxW = Math.max(mark.wide * scale, Math.min(needed + 8, 168));
            return (
              <div key={mark.key} className={mark.align === "center" ? "plot-label center" : "plot-label"} style={{ left: ox + mark.x * scale, top: oy + mark.y * scale, width: boxW, color: mark.fill, fontSize: nameSize }}>
                {mark.icon && big ? <Folder className="plot-folder" size={22} strokeWidth={1.7} aria-hidden="true" /> : null}
                <strong>{mark.name}</strong>
                {big && textFits(mark.size, 12, boxW) ? <span>{mark.size}</span> : null}
              </div>
            );
          })
        : null}
    </div>
  );
}

export function RankList({
  entries,
  colors,
  hot,
  selected,
  onHot,
  onSelect,
  onOpen,
}: {
  entries: Entry[];
  colors: string[];
  hot: string;
  selected: string;
  onHot: (path: string) => void;
  onSelect: (path: string) => void;
  onOpen: (path: string) => void;
}) {
  function onKey(event: ReactKeyboardEvent<HTMLOListElement>) {
    const index = Math.max(0, entries.findIndex((entry) => entry.path === selected));
    if (event.key === "ArrowDown" || event.key === "ArrowUp" || event.key === "Home" || event.key === "End") {
      event.preventDefault();
      const next = event.key === "Home" ? 0 : event.key === "End" ? entries.length - 1 : event.key === "ArrowDown" ? Math.min(entries.length - 1, index + 1) : Math.max(0, index - 1);
      onSelect(entries[next].path);
      event.currentTarget.querySelector<HTMLButtonElement>(`[data-i="${next}"]`)?.focus();
    }
    if ((event.key === "Enter" || event.key === "ArrowRight") && entries[index]) onOpen(entries[index].path);
  }
  return (
    <ol className="rank-list" tabIndex={0} onKeyDown={onKey} aria-label="文件夹占用排行">
      {entries.map((entry, index) => {
        const fill = colors[entry.color % colors.length];
        const on = selected === entry.path || hot === entry.path;
        return (
          <li key={entry.path}>
            <button
              type="button"
              data-i={index}
              className={on ? "row on" : "row"}
              aria-current={selected === entry.path ? "true" : undefined}
              aria-label={`进入 ${entry.name}，占用 ${formatBytes(entry.allocated)}`}
              onMouseEnter={() => { onHot(entry.path); onSelect(entry.path); }}
              onMouseLeave={() => onHot("")}
              onFocus={() => onSelect(entry.path)}
              onClick={() => onOpen(entry.path)}
            >
              <i style={{ background: fill }} />
              <span className="name">{entry.name}</span>
              <span className="size">{formatBytes(entry.allocated)}</span>
              <ChevronRight className="enter" size={16} aria-hidden="true" />
            </button>
          </li>
        );
      })}
    </ol>
  );
}

export function placeTip(event: MouseEvent<SVGGElement>, entry: Entry, onTip: (tip: { x: number; y: number; entry: Entry } | null) => void) {
  const host = event.currentTarget.ownerSVGElement?.parentElement;
  if (!host) return;
  const box = host.getBoundingClientRect();
  onTip({
    x: Math.max(8, Math.min(event.clientX - box.left + 12, box.width - 220)),
    y: Math.max(8, Math.min(event.clientY - box.top + 12, box.height - 96)),
    entry,
  });
}

export function Lens({
  entries,
  colors,
  hot,
  selected,
  onHot,
  onSelect,
  onTip,
  onOpen,
}: {
  entries: Entry[];
  colors: string[];
  hot: string;
  selected: string;
  onHot: (path: string) => void;
  onSelect: (path: string) => void;
  onTip: (tip: { x: number; y: number; entry: Entry } | null) => void;
  onOpen: (path: string) => void;
}) {
  const drawn = entries.filter((entry) => entry.r >= 8);
  if (drawn.length === 0) return <div className="empty">这一层没有可以展开的文件夹。</div>;
  return (
    <PlotStage
      label="目录占用气泡图"
      marks={drawn.map((entry) => ({
        key: entry.path,
        name: entry.name,
        size: formatBytes(entry.allocated),
        x: entry.x,
        y: entry.y,
        span: entry.r * 2,
        wide: entry.r * 1.55,
        align: "center" as const,
        fill: inkOn(colors[entry.color % colors.length]),
        icon: true,
      }))}
    >
      {drawn.map((entry) => {
        const fill = colors[entry.color % colors.length];
        const on = hot === entry.path || selected === entry.path;
        return (
          <g
            key={entry.path}
            className={on ? "bubble hot" : "bubble"}
            role="button"
            tabIndex={0}
            aria-label={`${entry.name}，占用 ${formatBytes(entry.allocated)}，${formatPercent(entry.percent)}，单击进入`}
            onMouseEnter={(event) => { onHot(entry.path); onSelect(entry.path); placeTip(event, entry, onTip); }}
            onMouseMove={(event) => placeTip(event, entry, onTip)}
            onMouseLeave={() => { onHot(""); onTip(null); }}
            onFocus={() => { onHot(entry.path); onSelect(entry.path); }}
            onBlur={() => onHot("")}
            onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onOpen(entry.path); } }}
            onClick={() => onOpen(entry.path)}
          >
            <circle cx={entry.x} cy={entry.y} r={entry.r} fill={on ? blendHex(fill, "#ffffff", 0.18) : fill} opacity={0.92} stroke={on ? "#ffffff" : blendHex(fill, "#ffffff", 0.25)} strokeWidth={on ? 3 : 1} />
          </g>
        );
      })}
    </PlotStage>
  );
}

export function TreeMap({
  entries,
  colors,
  hot,
  selected,
  onHot,
  onSelect,
  onTip,
  onOpen,
}: {
  entries: Entry[];
  colors: string[];
  hot: string;
  selected: string;
  onHot: (path: string) => void;
  onSelect: (path: string) => void;
  onTip: (tip: { x: number; y: number; entry: Entry } | null) => void;
  onOpen: (path: string) => void;
}) {
  const drawn = entries.filter((entry) => entry.tw > 1 && entry.th > 1);
  if (drawn.length === 0) return <div className="empty">这一层没有可以展开的文件夹。</div>;
  return (
    <PlotStage
      label="目录占用矩形图"
      marks={drawn.map((entry) => ({
        key: entry.path,
        name: entry.name,
        size: formatBytes(entry.allocated),
        x: entry.tx + 14,
        y: entry.ty + entry.th / 2,
        span: Math.min(entry.tw, entry.th),
        wide: Math.max(0, entry.tw - 24),
        align: "left" as const,
        fill: inkOn(colors[entry.color % colors.length]),
        icon: true,
      }))}
    >
      {drawn.map((entry) => {
        const fill = colors[entry.color % colors.length];
        const on = hot === entry.path || selected === entry.path;
        return (
          <g key={entry.path} className={on ? "cell hot" : "cell"} role="button" tabIndex={0} aria-label={`${entry.name}，占用 ${formatBytes(entry.allocated)}，单击进入`} onMouseEnter={(event) => { onHot(entry.path); onSelect(entry.path); placeTip(event, entry, onTip); }} onMouseMove={(event) => placeTip(event, entry, onTip)} onMouseLeave={() => { onHot(""); onTip(null); }} onFocus={() => onSelect(entry.path)} onBlur={() => onHot("")} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); onOpen(entry.path); } }} onClick={() => onOpen(entry.path)}>
            <rect x={entry.tx + 3} y={entry.ty + 3} width={Math.max(0, entry.tw - 6)} height={Math.max(0, entry.th - 6)} rx={12} fill={on ? blendHex(fill, "#ffffff", 0.16) : fill} opacity={0.92} stroke={on ? "#ffffff" : "transparent"} strokeWidth={on ? 3 : 0} />
          </g>
        );
      })}
    </PlotStage>
  );
}
