import { Link, Outlet, useLocation } from "@tanstack/react-router";
import {
  Activity,
  Clock3,
  Database,
  LogOut,
  Play,
  Settings,
} from "lucide-react";
import type { PropsWithChildren } from "react";
import {
  useAuthStatusQuery,
  useLogoutMutation,
} from "../../features/auth/auth.queries";

type AppShellProps = PropsWithChildren<{
  pageTitle?: string;
  pageKicker?: string;
}>;

export function AppShell({
  pageTitle,
  pageKicker = "本地守护进程",
  children,
}: AppShellProps) {
  const location = useLocation();
  const authStatus = useAuthStatusQuery();
  const logout = useLogoutMutation();
  const title = pageTitle ?? pageTitleForPath(location.pathname);
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
          <Link
            to="/"
            activeOptions={{ exact: true }}
            activeProps={{ "aria-current": "page" }}
          >
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
          <Link to="/settings" activeProps={{ "aria-current": "page" }}>
            <Settings aria-hidden="true" size={18} />
            设置
          </Link>
        </nav>
      </aside>
      <section className="content" aria-labelledby="page-title">
        <header className="topbar">
          <div>
            <p className="eyebrow">{pageKicker}</p>
            <h1 id="page-title">{title}</h1>
          </div>
          <div className="topbar-actions">
            <span className="status">{statusText}</span>
            {authStatus.data?.authenticated ? (
              <button
                aria-label="退出登录"
                className="icon-button"
                disabled={logout.isPending}
                onClick={() => logout.mutate()}
                title="退出登录"
                type="button"
              >
                <LogOut aria-hidden="true" size={16} />
              </button>
            ) : null}
          </div>
        </header>
        {children ?? <Outlet />}
      </section>
    </main>
  );
}

function pageTitleForPath(pathname: string): string {
  if (pathname === "/tasks/new") {
    return "创建任务";
  }
  if (/^\/tasks\/[^/]+\/edit$/.test(pathname)) {
    return "编辑任务";
  }
  if (pathname.startsWith("/tasks")) {
    return "任务";
  }
  if (pathname.startsWith("/runs")) {
    return "执行历史";
  }
  if (pathname.startsWith("/settings")) {
    return "设置";
  }
  return "运行状态";
}
