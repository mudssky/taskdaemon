import { LogIn } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
        <Label>
          用户名
          <Input
            autoComplete="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
          />
        </Label>
        <Label>
          密码
          <Input
            autoComplete="current-password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Label>
        {login.isError ? (
          <p className="form-error">登录失败，请检查账号或服务状态。</p>
        ) : null}
        <Button variant="primary" type="submit" disabled={login.isPending}>
          <LogIn aria-hidden="true" data-icon="inline-start" />
          {login.isPending ? "登录中" : "登录"}
        </Button>
      </form>
    </section>
  );
}
