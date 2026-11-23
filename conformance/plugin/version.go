package plugin

import (
	"runtime/debug"
	"strings"
)

// modulePath 是一致性模块的路径，在构建信息里按它找本模块的版本。
const modulePath = "github.com/xihanfun/framework-spec/conformance"

// develVersion 是取不到模块版本时的开发版标识：在本仓库里直接构建或测试，或模块被 replace 到本地目录。
const develVersion = "devel"

// Version 是运行器的版本：握手时作为宿主 SDK 版本上报，xihan-conformance version 也打印它。
// 它取自构建信息里一致性模块的版本，不带前缀 v：go install …/cmd/xihan-conformance@v1.2.3 得到 1.2.3，
// 作为库被别的模块依赖时是那个模块的 go.mod 钉的版本；取不到时是 devel。
var Version = moduleVersion(debug.ReadBuildInfo())

// moduleVersion 从构建信息里取一致性模块的版本并去掉前缀 v；没有构建信息、本模块不在其中，
// 或版本是空的、是 (devel) 时返回 develVersion。
func moduleVersion(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return develVersion
	}
	version := ""
	if info.Main.Path == modulePath {
		// 运行器自己是主模块：go install 带版本时是那个版本，在仓库里构建时是 (devel) 或由版本库信息推出的伪版本
		version = info.Main.Version
	} else {
		for _, dependency := range info.Deps {
			if dependency.Path != modulePath {
				continue
			}
			version = dependency.Version
			if dependency.Replace != nil {
				// replace 到另一个版本时取那个版本，replace 到本地目录时没有版本
				version = dependency.Replace.Version
			}
			break
		}
	}
	if version == "" || version == "(devel)" {
		return develVersion
	}
	return strings.TrimPrefix(version, "v")
}
