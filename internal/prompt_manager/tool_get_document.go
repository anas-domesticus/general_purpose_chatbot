package prompt_manager

import (
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetDocumentArgs represents the arguments for the get_document tool.
type GetDocumentArgs struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"The path to the document relative to the docs directory (e.g., 'api/reference.md' or 'guide.md')."`
}

// GetDocumentResult represents the result of the get_document tool.
type GetDocumentResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Found   bool   `json:"found"`
}

func (m *PromptManager) createGetDocumentTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "get_document",
		Description: "Retrieve a document by its path from the docs directory. Use this to access reference documentation, guides, or other supporting materials.",
	}, func(ctx tool.Context, args GetDocumentArgs) (GetDocumentResult, error) {
		content, err := m.GetDocument(ctx, args.Path)
		if err != nil {
			return GetDocumentResult{
				Path:  args.Path,
				Found: false,
			}, err
		}

		return GetDocumentResult{
			Path:    args.Path,
			Content: content,
			Found:   true,
		}, nil
	})
}

// Tools returns all ADK tools for the prompt manager.
func (m *PromptManager) Tools() ([]tool.Tool, error) {
	var tools []tool.Tool

	getDocTool, err := m.createGetDocumentTool()
	if err != nil {
		return nil, err
	}
	tools = append(tools, getDocTool)

	return tools, nil
}
