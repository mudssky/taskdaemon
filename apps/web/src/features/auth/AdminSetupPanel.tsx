import { UserPlus } from "lucide-react";
import { useState } from "react";
import { useInitializeAdminMutation } from "./auth.queries";

export function AdminSetupPanel() {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const initializeAdmin = useInitializeAdminMutation();
  const passwordMismatch =
    confirmPassword.length > 0 && password !== confirmPassword;
  const canSubmit =
    username.trim().length > 0 &&
    password.length > 0 &&
    confirmPassword.length > 0 &&
    !passwordMismatch &&
    !initializeAdmin.isPending;

  return (
    <section className="auth-panel" aria-labelledby="setup-title">
      <div>
        <p className="eyebrow">首次设置</p>
        <h2 id="setup-title">创建管理员账号</h2>
        <p>这是当前数据目录里的第一个管理员，创建成功后会直接进入控制台。</p>
      </div>
      <form
        className="auth-form"
        onSubmit={(event) => {
          event.preventDefault();
          if (!canSubmit) {
            return;
          }
          initializeAdmin.mutate({
            username: username.trim(),
            password,
          });
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
            autoComplete="new-password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        <label>
          确认密码
          <input
            autoComplete="new-password"
            type="password"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
          />
        </label>
        {passwordMismatch ? (
          <p className="form-error">两次输入的密码不一致。</p>
        ) : null}
        {initializeAdmin.isError ? (
          <p className="form-error">创建失败，请检查账号信息或服务状态。</p>
        ) : null}
        <button className="button primary" type="submit" disabled={!canSubmit}>
          <UserPlus aria-hidden="true" size={16} />
          {initializeAdmin.isPending ? "创建中" : "创建管理员"}
        </button>
      </form>
    </section>
  );
}
