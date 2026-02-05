package skills_manager //nolint:revive // var-naming: using underscores for domain clarity

import (
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// SearchSkillsArgs represents the arguments for the search skills tool.
type SearchSkillsArgs struct {
	// Query to match against skill names and descriptions. Use '*' to return all skills.
	Query string `json:"query" jsonschema:"required" jsonschema_description:"Search query for skill names and descriptions. Use '*' for all."`
}

// SkillSummary represents a skill in search results (without full text).
type SkillSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SearchSkillsResult represents the result of the search skills tool.
type SearchSkillsResult struct {
	Skills []SkillSummary `json:"skills"`
	Count  int            `json:"count"`
}

func (sm *skillsManager) createSearchTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "search_skills",
		Description: "Search for skills by name or description. Use '*' to list all available skills.",
	}, func(ctx tool.Context, args SearchSkillsArgs) (SearchSkillsResult, error) {
		skills, err := sm.SearchSkills(ctx, args.Query)
		if err != nil {
			return SearchSkillsResult{}, err
		}

		summaries := make([]SkillSummary, len(skills))
		for i, s := range skills {
			summaries[i] = SkillSummary{Name: s.Name, Description: s.Description}
		}

		return SearchSkillsResult{Skills: summaries, Count: len(summaries)}, nil
	})
}
