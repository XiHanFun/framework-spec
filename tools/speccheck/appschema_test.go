package speccheck_test

import (
	"encoding/json"
	"testing"
)

// appExample 是应用清单 xihan.app.json 的一个例子：name 说明它考的是 packaging/app-manifest.md 的哪一条，text 是清单的 JSON 文本。
type appExample struct {
	name string
	text string
}

// appAccepted 是正文允许的应用清单，Schema 必须放行。
var appAccepted = []appExample{
	{"only the required members", `{"name":"orders","project":"src/Orders/Orders.csproj"}`},
	{"a one-letter name", `{"name":"a","project":"a.csproj"}`},
	{"a 63-character name ending with a digit", `{"name":"a23456789012345678901234567890123456789012345678901234567890123","project":"a.csproj"}`},
	{"a name with inner hyphens", `{"name":"orders-api-v2","project":"a.csproj"}`},
	{"every member", `{
		"$schema": "https://spec.xihanfun.com/packaging/app.v1.json",
		"name": "orders",
		"project": "src/Orders/Orders.csproj",
		"profile": "full",
		"build": {"context": "."},
		"image": {"repository": "registry.example.com/shop/orders", "tag": "1.2.3", "aot": true},
		"ports": {"http": 8080, "grpc": 8081},
		"bindings": {
			"orders-db": {"type": "connection:relational", "secret": "orders-db"},
			"cache": {"type": "cache:distributed"},
			"jobs": {"type": "queue"},
			"files": {"type": "blob"},
			"events": {"type": "stream"},
			"nacos": {"type": "configuration:remote"}
		},
		"config": {"mountPath": "/etc/xihan", "files": ["appsettings.Production.json"]},
		"probes": {"startupSeconds": 60, "periodSeconds": 10, "timeoutSeconds": 2},
		"shutdown": {"graceSeconds": 30},
		"resources": {"requests": {"cpu": "500m", "memory": "512Mi"}, "limits": {"cpu": "1", "memory": "1Gi"}},
		"replicas": 2,
		"scaling": {"min": 2, "max": 5, "cpuPercent": 70},
		"leader": {"lease": "orders-leader"},
		"environment": {"ASPNETCORE_ENVIRONMENT": "Production", "_flag": "1"}
	}`},
}

// appRejected 是正文禁止、Schema 也必须拒绝的写法：每条只犯一处，其余照 appAccepted 的最小清单。
var appRejected = []appExample{
	// name：^[a-z][a-z0-9-]{0,62}$，不以 - 结尾
	{"name missing", `{"project":"a.csproj"}`},
	{"name ending with a hyphen", `{"name":"orders-","project":"a.csproj"}`},
	{"name of 63 characters ending with a hyphen", `{"name":"a2345678901234567890123456789012345678901234567890123456789012-","project":"a.csproj"}`},
	{"name of 64 characters", `{"name":"a234567890123456789012345678901234567890123456789012345678901234","project":"a.csproj"}`},
	{"name starting with a digit", `{"name":"1orders","project":"a.csproj"}`},
	{"name starting with a hyphen", `{"name":"-orders","project":"a.csproj"}`},
	{"name in upper case", `{"name":"Orders","project":"a.csproj"}`},
	{"name with an underscore", `{"name":"orders_api","project":"a.csproj"}`},
	{"name with a dot", `{"name":"orders.api","project":"a.csproj"}`},
	{"empty name", `{"name":"","project":"a.csproj"}`},
	{"name with a trailing newline", "{\"name\":\"orders\\n\",\"project\":\"a.csproj\"}"},
	// project 与 build.context
	{"project missing", `{"name":"orders"}`},
	{"empty project", `{"name":"orders","project":""}`},
	{"empty build context", `{"name":"orders","project":"a.csproj","build":{"context":""}}`},
	// profile
	{"unknown profile", `{"name":"orders","project":"a.csproj","profile":"tiny"}`},
	{"profile in upper case", `{"name":"orders","project":"a.csproj","profile":"Standard"}`},
	// image
	{"empty image repository", `{"name":"orders","project":"a.csproj","image":{"repository":""}}`},
	{"empty image tag", `{"name":"orders","project":"a.csproj","image":{"tag":""}}`},
	{"aot as text", `{"name":"orders","project":"a.csproj","image":{"aot":"true"}}`},
	// ports：1–65535
	{"http port 0", `{"name":"orders","project":"a.csproj","ports":{"http":0}}`},
	{"http port 65536", `{"name":"orders","project":"a.csproj","ports":{"http":65536}}`},
	{"grpc port 0", `{"name":"orders","project":"a.csproj","ports":{"grpc":0}}`},
	{"grpc port 65536", `{"name":"orders","project":"a.csproj","ports":{"grpc":65536}}`},
	{"fractional port", `{"name":"orders","project":"a.csproj","ports":{"http":8080.5}}`},
	// bindings：名字 ^[a-z][a-z0-9-]*$，type 六选一且必填，secret 是 DNS 标签
	{"binding name in upper case", `{"name":"orders","project":"a.csproj","bindings":{"Db":{"type":"queue"}}}`},
	{"binding name starting with a digit", `{"name":"orders","project":"a.csproj","bindings":{"1db":{"type":"queue"}}}`},
	{"binding without a type", `{"name":"orders","project":"a.csproj","bindings":{"db":{}}}`},
	{"unknown binding type", `{"name":"orders","project":"a.csproj","bindings":{"db":{"type":"connection:document"}}}`},
	{"binding secret ending with a hyphen", `{"name":"orders","project":"a.csproj","bindings":{"db":{"type":"queue","secret":"orders-db-"}}}`},
	{"binding secret in upper case", `{"name":"orders","project":"a.csproj","bindings":{"db":{"type":"queue","secret":"OrdersDb"}}}`},
	{"binding secret with a dot", `{"name":"orders","project":"a.csproj","bindings":{"db":{"type":"queue","secret":"orders.db"}}}`},
	// config：mountPath 是绝对路径，files 是不带目录分隔符的文件名
	{"relative mount path", `{"name":"orders","project":"a.csproj","config":{"mountPath":"etc/xihan"}}`},
	{"config file with a slash", `{"name":"orders","project":"a.csproj","config":{"files":["conf/appsettings.json"]}}`},
	{"config file with a backslash", `{"name":"orders","project":"a.csproj","config":{"files":["conf\\appsettings.json"]}}`},
	{"empty config file name", `{"name":"orders","project":"a.csproj","config":{"files":[""]}}`},
	// probes 与 shutdown：都至少 1
	{"period of 0 seconds", `{"name":"orders","project":"a.csproj","probes":{"periodSeconds":0}}`},
	{"timeout of 0 seconds", `{"name":"orders","project":"a.csproj","probes":{"timeoutSeconds":0}}`},
	{"startup of 0 seconds", `{"name":"orders","project":"a.csproj","probes":{"startupSeconds":0}}`},
	{"grace of 0 seconds", `{"name":"orders","project":"a.csproj","shutdown":{"graceSeconds":0}}`},
	// resources：cpu 与 memory 是 k8s 数量
	{"cpu that is not a quantity", `{"name":"orders","project":"a.csproj","resources":{"requests":{"cpu":"half"}}}`},
	{"memory with an unknown suffix", `{"name":"orders","project":"a.csproj","resources":{"limits":{"memory":"512MB"}}}`},
	// replicas 与 scaling
	{"0 replicas", `{"name":"orders","project":"a.csproj","replicas":0}`},
	{"scaling min of 0", `{"name":"orders","project":"a.csproj","scaling":{"min":0,"max":3}}`},
	{"cpu percent of 0", `{"name":"orders","project":"a.csproj","scaling":{"min":1,"max":3,"cpuPercent":0}}`},
	{"cpu percent of 101", `{"name":"orders","project":"a.csproj","scaling":{"min":1,"max":3,"cpuPercent":101}}`},
	// leader.lease：DNS 标签
	{"lease ending with a hyphen", `{"name":"orders","project":"a.csproj","leader":{"lease":"orders-leader-"}}`},
	{"lease in upper case", `{"name":"orders","project":"a.csproj","leader":{"lease":"OrdersLeader"}}`},
	{"lease of 64 characters", `{"name":"orders","project":"a.csproj","leader":{"lease":"a234567890123456789012345678901234567890123456789012345678901234"}}`},
	// environment：键 ^[A-Za-z_][A-Za-z0-9_]*$，值是字符串
	{"environment key starting with a digit", `{"name":"orders","project":"a.csproj","environment":{"1FLAG":"x"}}`},
	{"environment key with a hyphen", `{"name":"orders","project":"a.csproj","environment":{"MY-FLAG":"x"}}`},
	{"environment value that is a number", `{"name":"orders","project":"a.csproj","environment":{"FLAG":1}}`},
}

// appBeyondTheSchema 是正文禁止、JSON Schema 却表达不了的写法（两个成员之间的比较、连同缺省值的比较）：
// Schema 放行它们，xihan pack 读取清单时拒绝。packaging/app-manifest.md「Schema 之外的约束」列的就是这几条；
// 哪一条将来能在 Schema 里表达了，这里会失败，届时把它挪到 appRejected。清单指向的文件与目录是否存在同样不在 Schema 里，无从举例。
var appBeyondTheSchema = []appExample{
	{"grpc port equal to the http port", `{"name":"orders","project":"a.csproj","ports":{"http":8080,"grpc":8080}}`},
	{"grpc port equal to the default http port", `{"name":"orders","project":"a.csproj","ports":{"grpc":8080}}`},
	{"startup shorter than the period", `{"name":"orders","project":"a.csproj","probes":{"startupSeconds":5,"periodSeconds":10}}`},
	{"period longer than the default startup", `{"name":"orders","project":"a.csproj","probes":{"periodSeconds":120}}`},
	{"scaling max below min", `{"name":"orders","project":"a.csproj","scaling":{"min":3,"max":2}}`},
}

func TestAppSchemaRejectsWhatTheBodyForbids(t *testing.T) {
	root := repoRoot(t)
	schema := compileSchema(t, root, "packaging/app.schema.json")

	for _, example := range appAccepted {
		if err := validate(t, schema, example.name, json.RawMessage(example.text)); err != nil {
			t.Errorf("app manifest %q is allowed by packaging/app-manifest.md, so the schema must accept it: %v", example.name, err)
		}
	}
	for _, example := range appRejected {
		if err := validate(t, schema, example.name, json.RawMessage(example.text)); err == nil {
			t.Errorf("app manifest %q is forbidden by packaging/app-manifest.md, so the schema must reject it", example.name)
		}
	}
	for _, example := range appBeyondTheSchema {
		if err := validate(t, schema, example.name, json.RawMessage(example.text)); err != nil {
			t.Errorf("app manifest %q breaks a cross-field rule the schema could not express; if it can now, move the example to appRejected: %v", example.name, err)
		}
	}

	// 防止三张表被清空后照样通过
	if len(appAccepted) < 5 || len(appRejected) < 40 || len(appBeyondTheSchema) < 5 {
		t.Fatalf("checked %d accepted, %d rejected and %d cross-field app manifests", len(appAccepted), len(appRejected), len(appBeyondTheSchema))
	}
}
