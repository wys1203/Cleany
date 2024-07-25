package resource

import (
	"encoding/json"
	"fmt"

	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type UnstructuredResource struct {
	unstructured.Unstructured
}

type evaluateStatus struct {
	Matching bool   `json:"matching"`
	Message  string `json:"message"`
}

const (
	luaTableError = "lua script output is not a lua table"
	luaBoolError  = "lua script output is not a lua bool"
)

func (r *UnstructuredResource) Match(script string) (bool, string, error) {
	if script == "" {
		return true, "", nil
	}

	l := lua.NewState()
	defer l.Close()

	obj := mapToTable(r.UnstructuredContent())

	if err := l.DoString(script); err != nil {
		// logger.Info(fmt.Sprintf("doString failed: %v", err))
		return false, "", err
	}

	l.SetGlobal("obj", obj)

	if err := l.CallByParam(lua.P{
		Fn:      l.GetGlobal("evaluate"), // name of Lua function
		NRet:    1,                       // number of returned values
		Protect: true,                    // return err or panic
	}, obj); err != nil {
		// logger.Info(fmt.Sprintf("failed to evaluate health for resource: %v", err))
		return false, "", err
	}

	lv := l.Get(-1)
	tbl, ok := lv.(*lua.LTable)
	if !ok {
		// logger.Info(luaTableError)
		return false, "", fmt.Errorf("%s", luaTableError)
	}

	goResult := toGoValue(tbl)
	var resultJson []byte
	resultJson, err := json.Marshal(goResult)
	if err != nil {
		// logger.Info(fmt.Sprintf("failed to marshal result: %v", err))
		return false, "", err
	}

	var result evaluateStatus
	err = json.Unmarshal(resultJson, &result)
	if err != nil {
		// logger.Info(fmt.Sprintf("failed to marshal result: %v", err))
		return false, "", err
	}

	if result.Message != "" {
		// logger.Info(fmt.Sprintf("message: %s", result.Message))
	}

	// logger.V(logs.LogDebug).Info(fmt.Sprintf("is a match: %t", result.Matching))

	return result.Matching, result.Message, nil
}
