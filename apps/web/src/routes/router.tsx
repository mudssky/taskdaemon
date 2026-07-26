import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  type RouterHistory,
} from "@tanstack/react-router";
import { AppShell } from "../components/layout/AppShell";
import { AudioSettingsPage } from "../features/audio/AudioSettingsPage";
import { AdminSetupPanel } from "../features/auth/AdminSetupPanel";
import { useAuthStatusQuery } from "../features/auth/auth.queries";
import { LoginPanel } from "../features/auth/LoginPanel";
import { NotificationListPage } from "../features/notifications/NotificationListPage";
import { parseNotificationSearch } from "../features/notifications/notification.schema";
import { RunHistoryPage } from "../features/runs/RunHistoryPage";
import { StatusOverviewPage } from "../features/status/StatusOverviewPage";
import { TaskCreatePage } from "../features/tasks/TaskCreatePage";
import { TaskEditPage } from "../features/tasks/TaskEditPage";
import { TaskManagementPage } from "../features/tasks/TaskManagementPage";
import {
  DesktopEnvironmentPanel,
  DesktopNotificationPanel,
} from "../lib/desktop";

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

const taskCreateRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/tasks/new",
  component: TaskCreatePage,
});

const taskEditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/tasks/$taskId/edit",
  component: TaskEditPage,
});

const runsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/runs",
  component: RunHistoryPage,
});

const notificationsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/notifications",
  validateSearch: (search: Record<string, unknown>) =>
    parseNotificationSearch(search),
  component: NotificationListPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  component: SettingsPage,
});

/** 设置页：音频（T0）+ Desktop 能力（TD1/TD2 append-only）。 */
function SettingsPage() {
  return (
    <>
      <AudioSettingsPage />
      <div className="settings-grid">
        <DesktopEnvironmentPanel />
        <DesktopNotificationPanel />
      </div>
    </>
  );
}

export const routeTree = rootRoute.addChildren([
  indexRoute,
  tasksRoute,
  taskCreateRoute,
  taskEditRoute,
  runsRoute,
  notificationsRoute,
  settingsRoute,
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
