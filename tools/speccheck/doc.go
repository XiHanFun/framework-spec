// Package speccheck 是规范仓库自身的门禁，只有测试：错误码登记表与正文、向量、协议一致，
// 清单 Schema 与 RFC-0002 的读取步骤一致（ADR-0242），应用清单 Schema 拒绝 packaging/app-manifest.md 禁止的写法（ADR-0276）。
//
// 它是独立模块，不随 conformance 模块发布，Schema 校验库只在这里引用。
package speccheck
