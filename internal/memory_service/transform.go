package memory_service

import (
	"google.golang.org/genai"
)

// contentToData converts genai.Content to a JSON-serializable ContentData.
func contentToData(c *genai.Content) *ContentData {
	if c == nil {
		return nil
	}

	data := &ContentData{
		Role:  c.Role,
		Parts: make([]PartData, 0, len(c.Parts)),
	}

	for _, part := range c.Parts {
		// Only store text parts for now
		if part.Text != "" {
			data.Parts = append(data.Parts, PartData{
				Text: part.Text,
			})
		}
	}

	return data
}

// dataToContent converts ContentData back to genai.Content.
func dataToContent(d *ContentData) *genai.Content {
	if d == nil {
		return nil
	}

	content := &genai.Content{
		Role:  d.Role,
		Parts: make([]*genai.Part, 0, len(d.Parts)),
	}

	for _, part := range d.Parts {
		if part.Text != "" {
			content.Parts = append(content.Parts, &genai.Part{
				Text: part.Text,
			})
		}
	}

	return content
}

// extractTextFromContent extracts all text from a genai.Content for word indexing.
func extractTextFromContent(c *genai.Content) string {
	if c == nil {
		return ""
	}

	var text string
	for _, part := range c.Parts {
		if part.Text != "" {
			if text != "" {
				text += " "
			}
			text += part.Text
		}
	}

	return text
}
