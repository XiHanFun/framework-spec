package rules

// Exact 把规则包内部的 exact（数字保留原文的 JSON 读法）交给外部测试包：回放向量时与规则用同一种读法比较，不另抄一份。
var Exact = exact
