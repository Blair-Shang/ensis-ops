// Package tools 是 tools-service 的工具目录与执行入口。
//
// 新增工具：在本包用 init 调用 Register，声明分类、文案键与 Runner。
// 列表只返回元数据；处理发生在 tools.run，由 platform 转发到本进程。
// kind 为 format 的工具共用格式化页，kind 为 diff 的工具用文本对比页。
package tools
