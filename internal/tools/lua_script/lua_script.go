// Package lua_script provides Lua script execution tools for the chatbot.
package lua_script

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// Default limits for sandboxed execution
const (
	DefaultTimeout     = 5 * time.Second
	DefaultMaxMemoryMB = 64
)

// Args represents the arguments for the Lua script tool
type Args struct {
	Script    string         `json:"script" jsonschema:"required" jsonschema_description:"Lua script code to execute"`
	Variables map[string]any `json:"variables,omitempty" jsonschema_description:"Variables to pass to the script (accessible as globals)"`
	Timeout   int            `json:"timeout,omitempty" jsonschema_description:"Execution timeout in seconds (default: 5, max: 30)"`
}

// Result represents the result of the Lua script tool
type Result struct {
	Output   any      `json:"output,omitempty"`
	Logs     []string `json:"logs,omitempty"`
	Error    string   `json:"error,omitempty"`
	Duration string   `json:"duration"`
}

// Config holds configuration for creating the Lua script tool
type Config struct {
	MaxTimeout     time.Duration
	MaxMemoryMB    int
	AllowedModules []string
}

// DefaultConfig returns a secure default configuration
func DefaultConfig() Config {
	return Config{
		MaxTimeout:     30 * time.Second,
		MaxMemoryMB:    DefaultMaxMemoryMB,
		AllowedModules: []string{"string", "table", "math", "os_safe"},
	}
}

// luaExecutor handles sandboxed Lua execution
type luaExecutor struct {
	config Config
}

// newExecutor creates a new Lua executor with the given config
func newExecutor(config Config) *luaExecutor {
	return &luaExecutor{config: config}
}

// execute runs a Lua script with sandboxing
func (e *luaExecutor) execute(ctx context.Context, script string, variables map[string]any, timeout time.Duration) (any, []string, error) {
	// Create Lua state with restricted libraries
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})
	defer L.Close()

	// Open only safe libraries
	openSafeLibraries(L)

	// Capture logs
	var logs []string
	registerPrintFunction(L, &logs)

	// Register JSON helper
	registerJSONModule(L)

	// Set input variables as globals
	setVariables(L, variables)

	// Create a context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Set up execution limit via context
	L.SetContext(execCtx)

	// Execute the script
	if err := L.DoString(script); err != nil {
		return nil, logs, fmt.Errorf("script execution failed: %w", err)
	}

	// Get the result (if the script sets a global "result" variable)
	result := L.GetGlobal("result")
	output := luaValueToGo(result)

	return output, logs, nil
}

// openSafeLibraries opens only safe Lua standard libraries
func openSafeLibraries(ls *lua.LState) {
	// Base library (without dangerous functions)
	lua.OpenBase(ls)

	// Remove dangerous base functions
	ls.SetGlobal("dofile", lua.LNil)
	ls.SetGlobal("loadfile", lua.LNil)
	ls.SetGlobal("load", lua.LNil)
	ls.SetGlobal("loadstring", lua.LNil)

	// Safe libraries
	lua.OpenString(ls)
	lua.OpenTable(ls)
	lua.OpenMath(ls)

	// Safe subset of os (only time functions)
	registerSafeOS(ls)
}

// registerSafeOS registers a safe subset of os functions
func registerSafeOS(ls *lua.LState) {
	osMod := ls.NewTable()

	// os.time() - returns current time
	ls.SetField(osMod, "time", ls.NewFunction(func(l *lua.LState) int {
		l.Push(lua.LNumber(time.Now().Unix()))
		return 1
	}))

	// os.date() - format date string
	ls.SetField(osMod, "date", ls.NewFunction(func(l *lua.LState) int {
		format := l.OptString(1, "%c")
		t := time.Now()
		if l.GetTop() >= 2 {
			t = time.Unix(int64(l.CheckNumber(2)), 0)
		}
		// Simple format conversion (subset)
		result := formatTime(format, t)
		l.Push(lua.LString(result))
		return 1
	}))

	// os.difftime() - difference between times
	ls.SetField(osMod, "difftime", ls.NewFunction(func(l *lua.LState) int {
		t2 := l.CheckNumber(1)
		t1 := l.CheckNumber(2)
		l.Push(lua.LNumber(t2 - t1))
		return 1
	}))

	// os.clock() - CPU time used
	ls.SetField(osMod, "clock", ls.NewFunction(func(l *lua.LState) int {
		l.Push(lua.LNumber(float64(time.Now().UnixNano()) / 1e9))
		return 1
	}))

	ls.SetGlobal("os", osMod)
}

// formatTime converts Lua date format to Go time format (subset)
func formatTime(format string, t time.Time) string {
	switch format {
	case "%Y":
		return t.Format("2006")
	case "%m":
		return t.Format("01")
	case "%d":
		return t.Format("02")
	case "%H":
		return t.Format("15")
	case "%M":
		return t.Format("04")
	case "%S":
		return t.Format("05")
	case "%Y-%m-%d":
		return t.Format("2006-01-02")
	case "%Y-%m-%d %H:%M:%S":
		return t.Format("2006-01-02 15:04:05")
	case "*t":
		return t.Format(time.RFC3339)
	default:
		return t.Format(time.RFC3339)
	}
}

// registerPrintFunction registers a print function that captures output
func registerPrintFunction(ls *lua.LState, logs *[]string) {
	ls.SetGlobal("print", ls.NewFunction(func(l *lua.LState) int {
		top := l.GetTop()
		var parts []string
		for i := 1; i <= top; i++ {
			parts = append(parts, l.ToStringMeta(l.Get(i)).String())
		}
		msg := ""
		for i, p := range parts {
			if i > 0 {
				msg += "\t"
			}
			msg += p
		}
		*logs = append(*logs, msg)
		return 0
	}))
}

// registerJSONModule registers JSON encode/decode functions
func registerJSONModule(ls *lua.LState) {
	jsonMod := ls.NewTable()

	// json.encode(value) - convert Lua value to JSON string
	ls.SetField(jsonMod, "encode", ls.NewFunction(func(l *lua.LState) int {
		value := l.Get(1)
		goValue := luaValueToGo(value)
		data, err := json.Marshal(goValue)
		if err != nil {
			l.Push(lua.LNil)
			l.Push(lua.LString(err.Error()))
			return 2
		}
		l.Push(lua.LString(string(data)))
		return 1
	}))

	// json.decode(string) - parse JSON string to Lua value
	ls.SetField(jsonMod, "decode", ls.NewFunction(func(l *lua.LState) int {
		str := l.CheckString(1)
		var value any
		if err := json.Unmarshal([]byte(str), &value); err != nil {
			l.Push(lua.LNil)
			l.Push(lua.LString(err.Error()))
			return 2
		}
		l.Push(goValueToLua(l, value))
		return 1
	}))

	ls.SetGlobal("json", jsonMod)
}

// setVariables sets Go values as Lua globals
func setVariables(ls *lua.LState, variables map[string]any) {
	for name, value := range variables {
		ls.SetGlobal(name, goValueToLua(ls, value))
	}
}

// goValueToLua converts a Go value to a Lua value
func goValueToLua(ls *lua.LState, value any) lua.LValue {
	if value == nil {
		return lua.LNil
	}

	switch v := value.(type) {
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case float32:
		return lua.LNumber(v)
	case int:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case int32:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []any:
		tbl := ls.NewTable()
		for i, item := range v {
			tbl.RawSetInt(i+1, goValueToLua(ls, item))
		}
		return tbl
	case map[string]any:
		tbl := ls.NewTable()
		for key, val := range v {
			tbl.RawSetString(key, goValueToLua(ls, val))
		}
		return tbl
	default:
		// Try JSON marshaling for complex types
		data, err := json.Marshal(v)
		if err != nil {
			return lua.LString(fmt.Sprintf("%v", v))
		}
		return lua.LString(string(data))
	}
}

// luaValueToGo converts a Lua value to a Go value
func luaValueToGo(value lua.LValue) any {
	switch v := value.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		return luaTableToGo(v)
	case *lua.LNilType:
		return nil
	default:
		return nil
	}
}

// luaTableToGo converts a Lua table to either a Go slice or map
func luaTableToGo(tbl *lua.LTable) any {
	// Check if it's an array (sequential integer keys starting at 1)
	isArray := true
	maxIndex := 0
	tbl.ForEach(func(key, _ lua.LValue) {
		if num, ok := key.(lua.LNumber); ok {
			idx := int(num)
			if idx > maxIndex {
				maxIndex = idx
			}
		} else {
			isArray = false
		}
	})

	if isArray && maxIndex > 0 {
		// Convert to slice
		result := make([]any, maxIndex)
		tbl.ForEach(func(key, value lua.LValue) {
			if num, ok := key.(lua.LNumber); ok {
				idx := int(num) - 1
				if idx >= 0 && idx < maxIndex {
					result[idx] = luaValueToGo(value)
				}
			}
		})
		return result
	}

	// Convert to map
	result := make(map[string]any)
	tbl.ForEach(func(key, value lua.LValue) {
		keyStr := lua.LVAsString(key)
		result[keyStr] = luaValueToGo(value)
	})
	return result
}

// createHandler creates the tool handler
func createHandler(config Config) func(tool.Context, Args) (Result, error) {
	executor := newExecutor(config)

	return func(ctx tool.Context, args Args) (Result, error) {
		start := time.Now()

		// Validate script
		if args.Script == "" {
			return Result{
				Error:    "script is required",
				Duration: time.Since(start).String(),
			}, nil
		}

		// Determine timeout
		timeout := DefaultTimeout
		if args.Timeout > 0 {
			timeout = time.Duration(args.Timeout) * time.Second
			if timeout > config.MaxTimeout {
				timeout = config.MaxTimeout
			}
		}

		// Execute script (tool.Context embeds context.Context)
		output, logs, err := executor.execute(ctx, args.Script, args.Variables, timeout)

		duration := time.Since(start).String()

		if err != nil {
			// Check if it's a context deadline exceeded error
			if errors.Is(err, context.DeadlineExceeded) {
				return Result{
					Logs:     logs,
					Error:    "script execution timed out",
					Duration: duration,
				}, nil
			}
			return Result{
				Logs:     logs,
				Error:    err.Error(),
				Duration: duration,
			}, nil
		}

		return Result{
			Output:   output,
			Logs:     logs,
			Duration: duration,
		}, nil
	}
}

// New creates a new Lua script tool with default configuration
func New() (tool.Tool, error) {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new Lua script tool with custom configuration
func NewWithConfig(config Config) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name: "run_lua_script",
		Description: `Execute a sandboxed Lua script for data processing and automation tasks.

Available modules:
- string: String manipulation (find, gsub, match, format, etc.)
- table: Table operations (insert, remove, sort, concat)
- math: Math functions (abs, ceil, floor, max, min, random, etc.)
- os: Safe time functions only (time, date, difftime, clock)
- json: JSON encoding/decoding (json.encode, json.decode)

Set 'result' global to return a value. Use 'print()' for logging.

Example:
  -- Parse log line and extract data
  local ip = log_line:match("^(%d+%.%d+%.%d+%.%d+)")
  result = {ip = ip, parsed = true}`,
	}, createHandler(config))
}
