import { LogIn } from "lucide-react";
import { useState } from "react";
import { useLoginMutation } from "./auth.queries";

export function LoginPanel() {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const login = useLoginMutation();

  return (
    <section className="auth-panel" aria-labelledby="login-title">
      <div>
        <p className="eyebrow">管理员登录</p>
        <h2 id="login-title">连接 taskdaemon</h2>
        <p>登录后可以管理任务、触发执行和查看历史。</p>
      </div>
      <form
        className="auth-form"
        onSubmit={(event) => {
          event.preventDefault();
          login.mutate({ username, password });
        }}
      >
        <label>
          用户名
          <input
            autoComplete="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />
        </label>
        <label>
          密码
          <input
            autoComplete="current-password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        {login.isError ? (
          <p className="form-error">登录失败，请检查账号或服务状态。</p>
        ) : null}
        <button
          className="button primary"
          type="submit"
          disabled={login.isPending}
        >
          <LogIn aria-hidden="true" size={16} />
          {login.isPending ? "登录中" : "登录"}
        </button>
      </form>
    </section>
  );
}
