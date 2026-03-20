package config

import "fmt"

// ACPConfig holds all ACP-related configuration.
type ACPConfig struct {
	DefaultAgent string                      `yaml:"default_agent"`
	Cwd          string                      `yaml:"cwd"`
	Agents       map[string]ACPAgentConfig   `yaml:"agents"`
	Channels     map[string]ACPChannelConfig `yaml:"channels"`
}

// ACPAgentConfig configures a single ACP agent process.
type ACPAgentConfig struct {
	Command      string            `yaml:"command"`
	Args         []string          `yaml:"args"`
	Env          map[string]string `yaml:"env"`
	Cwd          string            `yaml:"cwd"`
	MCPServers   []ACPMCPServer    `yaml:"mcp_servers"`
	DefaultMode  string            `yaml:"default_mode"`
	DefaultModel string            `yaml:"default_model"`
}

// ACPMCPServer configures an MCP server to attach to an agent session.
type ACPMCPServer struct {
	Name      string            `yaml:"name"`
	Transport string            `yaml:"transport"` // "stdio", "http", "sse"
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args"`
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers"`
	Env       map[string]string `yaml:"env"`
}

// ACPChannelConfig maps a Slack channel to an agent and working directory.
type ACPChannelConfig struct {
	Agent string `yaml:"agent"`
	Cwd   string `yaml:"cwd"`
}

// Validate checks the ACPConfig for consistency.
func (c *ACPConfig) Validate() error {
	if len(c.Agents) == 0 {
		return fmt.Errorf("acp: at least one agent must be configured")
	}

	if c.DefaultAgent != "" {
		if _, ok := c.Agents[c.DefaultAgent]; !ok {
			return fmt.Errorf("acp: default_agent %q does not reference a configured agent", c.DefaultAgent)
		}
	}

	for name, agent := range c.Agents {
		if agent.Command == "" {
			return fmt.Errorf("acp: agent %q must have a non-empty command", name)
		}
	}

	for ch, chCfg := range c.Channels {
		if _, ok := c.Agents[chCfg.Agent]; !ok {
			return fmt.Errorf("acp: channel %q references unknown agent %q", ch, chCfg.Agent)
		}
	}

	return nil
}
