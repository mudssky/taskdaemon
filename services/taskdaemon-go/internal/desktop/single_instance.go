package desktop

// SingleInstanceUniqueID 是 Wails SingleInstance 跨平台锁标识。
// 使用 reverse-DNS，避免与其它应用冲突。
const SingleInstanceUniqueID = "com.taskdaemon.desktop"

// SingleInstanceExitCode 是第二实例在通知首实例后退出时使用的码。
const SingleInstanceExitCode = 0
