package skills_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// UpsertSkillArgs represents the arguments for the upsert skill tool.
type UpsertSkillArgs struct {
	Name        string `json:"name" jsonschema:"required" jsonschema_description:"The name of the skill (used as unique identifier)."`
	Description string `json:"description" jsonschema:"required" jsonschema_description:"A brief description of what the skill does."`
	Text        string `json:"text" jsonschema:"required" jsonschema_description:"The full text content of the skill."`
}

// UpsertSkillResult represents the result of the upsert skill tool.
type UpsertSkillResult struct {
	Success bool   `json:"success"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (sm *skillsManager) createUpsertTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "upsert_skill",
		Description: "Create a new skill or update an existing one. Skills are identified by their name.",
	}, func(ctx tool.Context, args UpsertSkillArgs) (UpsertSkillResult, error) {
		skill := Skill(args)

		if err := sm.UpsertSkill(ctx, skill); err != nil {
			return UpsertSkillResult{Success: false, Name: args.Name, Message: err.Error()}, err
		}

		return UpsertSkillResult{Success: true, Name: args.Name, Message: "Skill saved successfully"}, nil
	})
}
