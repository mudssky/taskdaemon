import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  type RouterHistory,
} from "@tanstack/react-router";
import { AgentWorkspacePage } from "../features/agent/AgentWorkspacePage";

const rootRoute = createRootRoute({
  component: () => <Outlet />,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: AgentWorkspacePage,
});

const threadRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/threads/$threadId",
  component: AgentWorkspacePage,
});

export const routeTree = rootRoute.addChildren([indexRoute, threadRoute]);

/**
 * 创建应用路由（可注入 history 便于测试）。
 *
 * 参数:
 *   - history: 可选 history。
 *
 * 返回值:
 *   - Router 实例。
 */
export function createAppRouter(history?: RouterHistory) {
  return createRouter({ routeTree, history });
}

export const router = createAppRouter();

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
