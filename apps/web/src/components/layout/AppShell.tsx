import { Link, Outlet } from "@tanstack/react-router";
import { Activity, Clock3, Database, Play, Settings } from "lucide-react";
import type { PropsWithChildren } from "react";
import { useAuthStatusQuery } from "../../features/auth/auth.queries";

type AppShellProps = PropsWithChildren<{
  pageTitle?: string;
  pageKicker?: string;
}>;

export function AppShell({
  pageTitle = "运行状态",
  pageKicker = "本地守护进程",
  children,
}: AppShellProps) {
  const authStatus = useAuthStatusQuery();
  const statusText = authStatus.data?.authenticated
    ? `管理员 ${authStatus.data.admin?.username ?? ""}`.trim()
    : "未登录";

  return (
    <main className="shell">
      <aside className="sidebar" aria-label="主导航">
        <div className="brand">
          <Clock3 aria-hidden="true" size={20} />
          taskdaemon
        </div>
        <nav>
          <Link to="/" activeProps={{ "aria-current": "page" }}>
            <Activity aria-hidden="true" size={18} />
            运行状态
          </Link>
          <Link to="/tasks" activeProps={{ "aria-current": "page" }}>
            <Play aria-hidden="true" size={18} />
            任务
          </Link>
          <Link to="/runs" activeProps={{ "aria-current": "page" }}>
            <Database aria-hidden="true" size={18} />
            执行历史
          </Link>
          <a href="#settings" aria-disabled="true">
            <Settings aria-hidden="true" size={18} />
            设置
          </a>
        </nav>
      </aside>
      <section className="content" aria-labelledby="page-title">
        <header className="topbar">
          <div>
            <p className="eyebrow">{pageKicker}</p>
            <h1 id="page-title">{pageTitle}</h1>
          </div>
          <span className="status">{statusText}</span>
        </header>
        {children ?? <Outlet />}
      </section>
    </main>
  );
}
