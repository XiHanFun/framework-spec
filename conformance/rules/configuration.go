package rules

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var secretNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,255}$`)

// MaxSecretNameLength 是密钥名的最大长度（RFC-0014「密钥」）；文法只含 ASCII，字符数就是字节数。
const MaxSecretNameLength = 256

// IsSecretName 判断密钥名是否合文法（RFC-0014「密钥」）：字母或数字开头，其余是字母、数字、.、_、/、-，
// 最长 256 个字符；不含 ..、//，不以 / 结尾。
func IsSecretName(name string) bool {
	return secretNamePattern.MatchString(name) && !strings.Contains(name, "..") && !strings.Contains(name, "//") && !strings.HasSuffix(name, "/")
}

// ConfigurationContentVersion 给出 Nacos 远程配置文档的版本（RFC-0014「Nacos Provider」）：内容按 UTF-8 编码的 MD5，小写十六进制。
// 这是 Nacos 监听接口的约定，只用来与服务端比对内容是否变化，不用于任何安全目的。
func ConfigurationContentVersion(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

// FlattenJSONConfiguration 把 JSON 文档展平成配置的键值（RFC-0014「Nacos Provider」的 Json 格式）：对象按 : 展平，
// 数组按下标，null 保留为空值（nil）；文档不是 JSON 对象返回假（CONFIGURATION_VALIDATION_001）。
func FlattenJSONConfiguration(content string) (map[string]*string, bool) {
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return nil, false
	}
	// 根值之后不能再有别的内容
	if _, err := decoder.Token(); err != io.EOF {
		return nil, false
	}
	document, ok := root.(map[string]any)
	if !ok {
		return nil, false
	}
	values := map[string]*string{}
	flattenConfigurationValue(document, "", values)
	return values, true
}

// flattenConfigurationValue 把一个 JSON 值记到 prefix 之下：容器逐层展开，叶子写成文本。
func flattenConfigurationValue(value any, prefix string, values map[string]*string) {
	child := func(name string) string {
		if prefix == "" {
			return name
		}
		return prefix + ":" + name
	}
	text := func(text string) { values[prefix] = &text }
	switch typed := value.(type) {
	case map[string]any:
		for name, item := range typed {
			flattenConfigurationValue(item, child(name), values)
		}
	case []any:
		for index, item := range typed {
			flattenConfigurationValue(item, prefix+":"+strconv.Itoa(index), values)
		}
	case nil:
		values[prefix] = nil
	case string:
		text(typed)
	case json.Number:
		text(typed.String())
	case bool:
		// 布尔的文本形式规范不作规定，向量也不收；这里照参考实现
		if typed {
			text("True")
		} else {
			text("False")
		}
	}
}

// FlattenPropertiesConfiguration 把 properties 文档展平成配置的键值（RFC-0014「Nacos Provider」的 Properties 格式）：
// 每行 键=值，键里的 . 换成 :，# 与 ! 起头的行是注释，空行跳过；有一行的键为空返回假。
func FlattenPropertiesConfiguration(content string) (map[string]*string, bool) {
	values := map[string]*string{}
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		// 第一个分隔符之前是键；参考实现把冒号也认作分隔符，规范只写了等号
		key, value := line, ""
		if separator := strings.IndexAny(line, "=:"); separator >= 0 {
			key, value = line[:separator], strings.TrimSpace(line[separator+1:])
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, false
		}
		values[strings.ReplaceAll(key, ".", ":")] = &value
	}
	return values, true
}
