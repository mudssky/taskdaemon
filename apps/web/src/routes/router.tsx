import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from "@tanstack/react-router";
import { AppShell } from "../components/layout/AppShell";
import { useSessionQuery } from "../features/auth/auth.queries";
import { LoginPanel } from "../features/auth/LoginPanel";
import { TaskDashboard } from "../features/tasks/TaskDashboard";

function ProtectedApp() {
  const session = useSessionQuery();

  if (session.isLoading) {
    return (
      <AppShell pageTitle="运行状态">
        <div className="panel">
          <p>正在检查登录状态...</p>
        </div>
      </AppShell>
    );
  }

  if (session.isError) {
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
  component: TaskDashboard,
});

const tasksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/tasks",
  component: TaskDashboard,
});

const runsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/runs",
  component: TaskDashboard,
});

export const routeTree = rootRoute.addChildren([
  indexRoute,
  tasksRoute,
  runsRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
