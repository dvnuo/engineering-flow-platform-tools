@echo off
REM ============================================================================
REM start-bridge.cmd —— 本地桥接服务的启动器，由 efp-bridge:// 协议链接调用。
REM
REM 作用：Windows 在用户点击/输入一个 efp-bridge:// 链接时，会运行注册表里登记的
REM 这条命令，并把链接的完整 URL 作为第一个参数 (%1) 传进来（例如 efp-bridge://start）。
REM 这个 .cmd 就是那条命令，负责把本地桥接服务拉起来。用它做中间层，是因为协议处理器
REM 只能指向一个可执行文件，而我们要固定参数、记录调用、将来还要从 URL 里解析 token。
REM
REM 验证阶段：起 Python 验证服务 efp_bridge_probe.py，并记一行日志证明协议确实生效。
REM 产品阶段：把最后一行换成 `browser serve --origin ... --token ...`（token 从 %1 解析）。
REM ============================================================================

REM 记录一次调用（%~1 去掉外层引号；用引号包住，URL 里若带 & 也不会被 cmd 当成分隔符）
if not exist "%LOCALAPPDATA%\efp" mkdir "%LOCALAPPDATA%\efp"
echo %DATE% %TIME%  invoked arg="%~1">>"%LOCALAPPDATA%\efp\bridge-launch.log"

REM ↓↓↓ 按你的机器改这三行，改完保存 ↓↓↓
set "PROBE=C:\Users\User\Days\2026-09-12\jmeter-test\efp_bridge_probe.py"
set "ORIGIN=https://efp.gfx-mock.iwpb.ape1.preprod.aws.cloud.comp"
set "TOKEN=efp-test-token-123"
REM ↑↑↑ 改完保存 ↑↑↑

REM 起一个可见窗口跑验证服务，这样输入协议链接后你能亲眼看到它被拉起。
start "EFP Bridge" python "%PROBE%" --portal-origin "%ORIGIN%" --token "%TOKEN%" --browser-exe browser
