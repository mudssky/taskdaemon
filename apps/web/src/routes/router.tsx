import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  type RouterHistory,
} from "@tanstack/react-router";
import { AppShell } from "../components/layout/AppShell";
import { AdminSetupPanel } from "../features/auth/AdminSetupPanel";
import { useAuthStatusQuery } from "../features/auth/auth.queries";
import { LoginPanel } from "../features/auth/LoginPanel";
import { RunHistoryPage } from "../features/runs/RunHistoryPage";
import { StatusOverviewPage } from "../features/status/StatusOverviewPage";
import { TaskManagementPage } from "../features/tasks/TaskManagementPage";

function ProtectedApp() {
  const authStatus = useAuthStatusQuery();

  if (authStatus.isLoading) {
    return (
      <AppShell pageTitle="运行状态">
        <div className="panel">
          <p>正在检查登录状态...</p>
        </div>
      </AppShell>
    );
  }

  if (authStatus.isError) {
    return (
      <AppShell pageTitle="登录">
        <div className="empty-state error">
          认证状态查询失败，请确认后端服务可用。
        </div>
      </AppShell>
    );
  }

  const status = authStatus.data;

  if (!status?.initialized) {
    return (
      <AppShell pageTitle="首次设置">
        <AdminSetupPanel />
      </AppShell>
    );
  }

  if (!status.authenticated) {
    return (
      <AppShell pageTitle="登录">
        <LoginPanel />
      </AppShell>
    );
  }

  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

const rootRoute = createRootRoute({
  component: ProtectedApp,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: StatusOverviewPage,
});

const tasksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/tasks",
  component: TaskManagementPage,
});

const runsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/runs",
  component: RunHistoryPage,
});

export const routeTree = rootRoute.addChildren([
  indexRoute,
  tasksRoute,
  runsRoute,
]);

export function createAppRouter(history?: RouterHistory) {
  return createRouter({ routeTree, history });
}

export const router = createAppRouter();

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
