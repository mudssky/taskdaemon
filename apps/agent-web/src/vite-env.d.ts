/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** mock | http；缺省 mock（G2 未就绪时本地开发）。 */
  readonly VITE_AGENT_API_MODE?: string;
  /** http 模式下 gateway origin。 */
  readonly VITE_AGENT_API_ORIGIN?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
