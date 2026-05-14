import { createRoot } from "react-dom/client";
import { Activity, Database, Play, Settings } from "lucide-react";
import "./styles.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Root element not found");
}

createRoot(root).render(
  <main className="shell">
    <aside className="sidebar" aria-label="主导航">
      <div className="brand">taskdaemon</div>
      <nav>
        <a href="/" aria-current="page">
          <Activity aria-hidden="true" size={18} />
          运行状态
        </a>
        <a href="/tasks">
          <Play aria-hidden="true" size={18} />
          任务
        </a>
        <a href="/runs">
          <Database aria-hidden="true" size={18} />
          执行历史
        </a>
        <a href="/settings">
          <Settings aria-hidden="true" size={18} />
          设置
        </a>
      </nav>
    </aside>
    <section className="content" aria-labelledby="page-title">
      <header className="topbar">
        <div>
          <p className="eyebrow">本地守护进程</p>
          <h1 id="page-title">运行状态</h1>
        </div>
        <span className="status">API 未连接</span>
      </header>
      <div className="panel">
        <h2>任务队列</h2>
        <p>项目骨架已就绪。后续任务会在这里接入任务列表、手动触发和执行历史。</p>
      </div>
    </section>
  </main>
);
