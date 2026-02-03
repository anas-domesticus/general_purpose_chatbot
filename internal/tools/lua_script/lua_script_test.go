package lua_script

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_BasicScript(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	output, logs, err := executor.execute(context.Background(), `result = 1 + 2`, nil, 5*time.Second)
	require.NoError(t, err)
	assert.Empty(t, logs)
	assert.Equal(t, float64(3), output)
}

func TestExecutor_StringManipulation(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local str = "hello world"
		result = string.upper(str)
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "HELLO WORLD", output)
}

func TestExecutor_PatternMatching(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local log = "192.168.1.100 - - [10/Oct/2023] GET /api/users 200"
		local ip = log:match("^(%d+%.%d+%.%d+%.%d+)")
		local status = log:match("(%d+)$")
		result = {ip = ip, status = tonumber(status)}
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	outputMap, ok := output.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "192.168.1.100", outputMap["ip"])
	assert.Equal(t, float64(200), outputMap["status"])
}

func TestExecutor_Variables(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `result = input * 2`
	variables := map[string]any{"input": float64(21)}

	output, _, err := executor.execute(context.Background(), script, variables, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, float64(42), output)
}

func TestExecutor_ComplexVariables(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local total = 0
		for _, pod in ipairs(pods) do
			total = total + pod.restarts
		end
		result = total
	`
	variables := map[string]any{
		"pods": []any{
			map[string]any{"name": "api-1", "restarts": float64(2)},
			map[string]any{"name": "api-2", "restarts": float64(3)},
			map[string]any{"name": "db-1", "restarts": float64(1)},
		},
	}

	output, _, err := executor.execute(context.Background(), script, variables, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, float64(6), output)
}

func TestExecutor_PrintCapture(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		print("line 1")
		print("line 2", "extra")
		result = "done"
	`
	output, logs, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "done", output)
	assert.Len(t, logs, 2)
	assert.Equal(t, "line 1", logs[0])
	assert.Equal(t, "line 2\textra", logs[1])
}

func TestExecutor_TableOperations(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local items = {3, 1, 4, 1, 5, 9}
		table.sort(items)
		result = items
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	outputSlice, ok := output.([]any)
	require.True(t, ok)
	assert.Equal(t, []any{float64(1), float64(1), float64(3), float64(4), float64(5), float64(9)}, outputSlice)
}

func TestExecutor_MathOperations(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		result = {
			abs = math.abs(-5),
			ceil = math.ceil(4.2),
			floor = math.floor(4.8),
			max = math.max(1, 5, 3),
			min = math.min(1, 5, 3),
			sqrt = math.sqrt(16)
		}
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	outputMap, ok := output.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(5), outputMap["abs"])
	assert.Equal(t, float64(5), outputMap["ceil"])
	assert.Equal(t, float64(4), outputMap["floor"])
	assert.Equal(t, float64(5), outputMap["max"])
	assert.Equal(t, float64(1), outputMap["min"])
	assert.Equal(t, float64(4), outputMap["sqrt"])
}

func TestExecutor_JSONEncode(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local data = {name = "test", value = 42}
		result = json.encode(data)
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	// JSON output is a string
	outputStr, ok := output.(string)
	require.True(t, ok)
	assert.Contains(t, outputStr, `"name":"test"`)
	assert.Contains(t, outputStr, `"value":42`)
}

func TestExecutor_JSONDecode(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local str = '{"name":"pod-1","status":"Running","restarts":3}'
		local data = json.decode(str)
		result = {
			name = data.name,
			restarts = data.restarts
		}
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	outputMap, ok := output.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pod-1", outputMap["name"])
	assert.Equal(t, float64(3), outputMap["restarts"])
}

func TestExecutor_SafeOSTime(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local now = os.time()
		result = now > 0
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, true, output)
}

func TestExecutor_SafeOSDate(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		result = os.date("%Y-%m-%d")
	`
	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	outputStr, ok := output.(string)
	require.True(t, ok)
	// Just verify it returns something in the right format
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, outputStr)
}

func TestExecutor_Timeout(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	// Infinite loop script
	script := `while true do end`

	_, _, err := executor.execute(context.Background(), script, nil, 100*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestExecutor_SyntaxError(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `this is not valid lua`

	_, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "script execution failed")
}

func TestExecutor_RuntimeError(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local x = nil
		result = x.foo  -- nil index error
	`

	_, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.Error(t, err)
}

func TestExecutor_DangerousFunctionsRemoved(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	tests := []struct {
		name   string
		script string
	}{
		{"dofile", `dofile("test.lua")`},
		{"loadfile", `loadfile("test.lua")`},
		{"load", `load("return 1")()`},
		{"loadstring", `loadstring("return 1")()`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := executor.execute(context.Background(), tt.script, nil, 5*time.Second)
			require.Error(t, err)
		})
	}
}

func TestExecutor_NoIOModule(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `io.open("test.txt", "r")`

	_, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "script execution failed")
}

func TestExecutor_NoOSExecute(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `os.execute("ls")`

	_, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.Error(t, err)
}

func TestExecutor_NoResult(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `local x = 1 + 2`  // no result set

	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)
	assert.Nil(t, output)
}

func TestExecutor_ArrayResult(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `result = {10, 20, 30}`

	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	outputSlice, ok := output.([]any)
	require.True(t, ok)
	assert.Equal(t, []any{float64(10), float64(20), float64(30)}, outputSlice)
}

func TestExecutor_BooleanResult(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `result = 5 > 3`

	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, true, output)
}

func TestExecutor_LogParsingExample(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	// Realistic log parsing scenario
	script := `
		local errors = {}
		for line in logs:gmatch("[^\n]+") do
			local level, msg = line:match("level=(%w+)%s+msg=(.+)")
			if level == "error" then
				table.insert(errors, msg)
			end
		end
		result = errors
	`
	variables := map[string]any{
		"logs": `level=info msg=starting up
level=error msg=connection refused to db-01
level=info msg=retrying
level=error msg=timeout waiting for response`,
	}

	output, _, err := executor.execute(context.Background(), script, variables, 5*time.Second)
	require.NoError(t, err)

	outputSlice, ok := output.([]any)
	require.True(t, ok)
	assert.Len(t, outputSlice, 2)
	assert.Equal(t, "connection refused to db-01", outputSlice[0])
	assert.Equal(t, "timeout waiting for response", outputSlice[1])
}

func TestExecutor_KubectlOutputParsing(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local pods = {}
		local first = true
		for line in output:gmatch("[^\n]+") do
			if first then
				first = false
			else
				local name, ready, status, restarts = line:match("^(%S+)%s+(%S+)%s+(%S+)%s+(%d+)")
				if name then
					table.insert(pods, {
						name = name,
						ready = ready,
						status = status,
						restarts = tonumber(restarts)
					})
				end
			end
		end

		-- Find pods with restarts > 0
		local troubled = {}
		for _, pod in ipairs(pods) do
			if pod.restarts > 0 then
				table.insert(troubled, pod.name)
			end
		end
		result = troubled
	`
	variables := map[string]any{
		"output": `NAME                     READY   STATUS    RESTARTS
api-deployment-abc123    1/1     Running   0
api-deployment-def456    1/1     Running   3
db-statefulset-0         1/1     Running   1`,
	}

	output, _, err := executor.execute(context.Background(), script, variables, 5*time.Second)
	require.NoError(t, err)

	outputSlice, ok := output.([]any)
	require.True(t, ok)
	assert.Len(t, outputSlice, 2)
	assert.Contains(t, outputSlice, "api-deployment-def456")
	assert.Contains(t, outputSlice, "db-statefulset-0")
}

func TestNew(t *testing.T) {
	tool, err := New()
	require.NoError(t, err)
	assert.NotNil(t, tool)
	assert.Equal(t, "run_lua_script", tool.Name())
}

func TestNewWithConfig(t *testing.T) {
	config := Config{
		MaxTimeout:  10 * time.Second,
		MaxMemoryMB: 32,
	}
	tool, err := NewWithConfig(config)
	require.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestGoValueToLua_AllTypes(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	variables := map[string]any{
		"str":     "hello",
		"num":     float64(42),
		"numInt":  123,
		"numInt64": int64(456),
		"boolean": true,
		"null":    nil,
		"arr":     []any{float64(1), float64(2), float64(3)},
		"obj": map[string]any{
			"nested": "value",
		},
	}

	script := `
		result = {
			str_ok = str == "hello",
			num_ok = num == 42,
			numInt_ok = numInt == 123,
			numInt64_ok = numInt64 == 456,
			bool_ok = boolean == true,
			null_ok = null == nil,
			arr_ok = arr[1] == 1 and arr[2] == 2 and arr[3] == 3,
			obj_ok = obj.nested == "value"
		}
	`

	output, _, err := executor.execute(context.Background(), script, variables, 5*time.Second)
	require.NoError(t, err)

	outputMap, ok := output.(map[string]any)
	require.True(t, ok)

	for key, val := range outputMap {
		assert.True(t, val.(bool), "expected %s to be true", key)
	}
}

func TestJSONDecodeError(t *testing.T) {
	executor := newExecutor(DefaultConfig())

	script := `
		local data, err = json.decode("not valid json")
		if err then
			result = "error: " .. err
		else
			result = "unexpected success"
		end
	`

	output, _, err := executor.execute(context.Background(), script, nil, 5*time.Second)
	require.NoError(t, err)

	outputStr, ok := output.(string)
	require.True(t, ok)
	assert.Contains(t, outputStr, "error:")
}
