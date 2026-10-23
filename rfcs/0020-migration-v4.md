# RFC-0020 从 4.x 迁移：`xihan migrate v4`

> 状态：已采纳 | 日期：2026-10-24 | 对应 ADR：0014 | 参考实现：`XiHan.Framework.Cli`（`migrate` 命令） | 规范：`migration/v4.md`

## 摘要

4.x 到 5.0 的迁移不提供运行期兼容桥（ADR-0014），由命令行工具 `xihan migrate v4` 对源码做语法级重写：模块变特性、实体基类变协议接口、注入标记变显式注册、动态 API 变操作契约（委托模式）、本地事件变消息、本地化资源变消息目录、包引用改名；改不了的位置留标记，结果出报告。规则正文在 `migration/v4.md`。

## 动机

迁移的工作量集中在入口层与协议层的机械改写（BasicApp：`[DynamicApi]` 165 处、`[PermissionAuthorize]` 501、`[UnitOfWork]` 287、实体基类 166、`ICurrentUser` / `ICurrentTenant` 255），业务方法体基本不动。把这些改写固化成有编号的规则，既是工具，也是迁移指南的事实源：每条规则说明 4.x 的写法在 5.0 对应什么，人手改也按同一张表。

## 决策

1. 工具只做语法级重写，不依赖 MSBuild 工作区：快、可在任何目录上跑、结果可复现；跨文件才能判定的改动留标记交人。
2. 规则有编号（`V4-01` … `V4-10`）与固定顺序，报告按规则给出命中、改动与标记数。
3. `[DynamicApi]` 以委托模式迁移：原服务类保留为作用域服务，生成薄操作类委托调用，先让契约层对齐 5.0，再逐个把服务体收进操作。
4. 调用链深处取"当前用户 / 当前租户"经内核的 `IInvocationAccessor`（RFC-0003 增补），工具生成应用层包装，不改调用点。
5. 实体以协议接口组合替代基类，`BasicId` → `Id`、`RowVersion` → `ConcurrencyStamp`、租户 ID 改为字符串；列名沿用 4.x，列类型的变化留标记。

## 与 .NET 的映射

- `xihan migrate v4` 在 `XiHan.Framework.Cli`，规则实现为 Roslyn `CSharpSyntaxRewriter`；项目文件与 JSON 资源由文本规则处理。
- 错误码：路径不存在 `CLI_NOTFOUND_002`，版本不认识 `CLI_VALIDATION_002`，规则编号不认识 `CLI_VALIDATION_003`。
- 标记与报告里的文案走消息目录 `cli`，按 `--locale` 渲染。

## 未决

- V4-06 的动词推导对非常规命名（如 `DoSomethingForXxx`）只能给出 kebab 原文，由人改成资源 / 动词；是否提供交互式确认留给后续版本。
- 4.x 的 `ICurrentTenant.Change` 与嵌套工作单元没有 5.0 对应物，只留标记。
