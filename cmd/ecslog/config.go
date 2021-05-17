package main

// Config file support. Load a config file from "~/.ecslog.toml".

import (
	"fmt"
	"os"
	"runtime"

	"github.com/pelletier/go-toml"
	"github.com/trentm/go-ecslog/internal/lg"
)

type config struct {
	tree *toml.Tree
}

func (c *config) GetBool(key string) (val bool, ok bool) {
	if c.tree == nil {
		return false, false
	}
	item := c.tree.Get(key)
	if item == nil {
		return false, false
	}
	val, ok = item.(bool)
	if !ok {
		lg.Printf("ignore config value: not bool: %s=%v (%T)\n", key, item, item)
		return false, false
	}
	return
}

// GetInt gets the value of the `key` from the config file if it is a number
// value.
func (c *config) GetInt(key string) (val int, ok bool) {
	if c.tree == nil {
		return 0, false
	}
	item := c.tree.Get(key)
	if item == nil {
		return 0, false
	}
	val64, ok := item.(int64)
	if !ok {
		lg.Printf("ignore config value: not int: %s=%v (%T)\n", key, item, item)
		return 0, false
	}
	val = int(val64)
	if int64(val) != val64 {
		lg.Printf("ignore config value: int too large: %s=%d\n", key, val64)
		return 0, false
	}
	return
}

func (c *config) GetString(key string) (val string, ok bool) {
	if c.tree == nil {
		return "", false
	}
	item := c.tree.Get(key)
	if item == nil {
		return "", false
	}
	val, ok = item.(string)
	if !ok {
		lg.Printf("ignore config value: not string: %s=%v (%T)\n", key, item, item)
		return "", false
	}
	return
}

func globalConfigFilePath() string {
	var homeEnvVar string
	if runtime.GOOS == "windows" {
		homeEnvVar = "UserProfile"
	} else {
		homeEnvVar = "HOME"
	}
	homeDir, ok := os.LookupEnv(homeEnvVar)
	if !ok {
		return ""
	}
	return homeDir + string(os.PathSeparator) + ".ecslog.toml"
}

func loadConfig() (*config, error) {
	cfgPath := globalConfigFilePath()
	if cfgPath == "" {
		return &config{}, nil
	}

	globalTree, err := toml.LoadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file. No worries.
			return &config{}, nil
		}
		return nil, fmt.Errorf("error loading '%s': %s", cfgPath, err)
	}

	localTree, err := toml.LoadFile(".")
	if err != nil || localTree == nil {
		return &config{globalTree}, nil
	}
	merged := merge(localTree.ToMap(), globalTree.ToMap())
	tree, err := toml.TreeFromMap(merged)
	if err != nil {
		return nil, err
	}
	return &config{tree}, nil
}

func merge(base, overrides map[string]interface{}) map[string]interface{} {
	for key, override := range overrides {
		if value, ok := base[key]; ok {
			nestedBase, baseKeyIsMap := value.(map[string]interface{})
			nestedOverrides, overrideIsMap := override.(map[string]interface{})
			if baseKeyIsMap && overrideIsMap {
				base[key] = merge(nestedBase, nestedOverrides)
			}
		} else {
			base[key] = override
		}
	}
	return base
}
