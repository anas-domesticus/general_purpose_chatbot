package skills_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// RetrieveSkillArgs represents the arguments for the retrieve skill tool.
type RetrieveSkillArgs struct {
	Name string `json:"name" jsonschema:"The exact name of the skill to retrieve."`
}

// RetrieveSkillResult represents the result of the retrieve skill tool.
type RetrieveSkillResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Text        string `json:"text"`
	Found       bool   `json:"found"`
}

func (sm *skillsManager) createRetrieveTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "retrieve_skill",
		Description: "Retrieve a skill by its exact name to get the full skill text.",
	}, func(ctx tool.Context, args RetrieveSkillArgs) (RetrieveSkillResult, error) {
		skill, err := sm.RetrieveSkill(ctx, args.Name)
		if err != nil {
			return RetrieveSkillResult{}, err
		}

		if skill == nil {
			return RetrieveSkillResult{Found: false}, fmt.Errorf("skill not found: %s", args.Name)
		}

		return RetrieveSkillResult{
			Name:        skill.Name,
			Description: skill.Description,
			Text:        skill.Text,
			Found:       true,
		}, nil
	})
}
