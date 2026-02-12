package slack

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestExtractMessageText_PlainText(t *testing.T) {
	msg := slack.Message{}
	msg.Text = "Hello world"

	got := extractMessageText(msg)
	if got != "Hello world" {
		t.Errorf("expected %q, got %q", "Hello world", got)
	}
}

func TestExtractMessageText_PlainTextTakesPrecedence(t *testing.T) {
	msg := slack.Message{}
	msg.Text = "plain text"
	msg.Attachments = []slack.Attachment{{Text: "attachment text"}}

	got := extractMessageText(msg)
	if got != "plain text" {
		t.Errorf("expected plain text to take precedence, got %q", got)
	}
}

func TestExtractMessageText_EmptyMessage(t *testing.T) {
	msg := slack.Message{}

	got := extractMessageText(msg)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractMessageText_AttachmentText(t *testing.T) {
	msg := slack.Message{}
	msg.Attachments = []slack.Attachment{
		{Text: "Alert: pods not ready"},
	}

	got := extractMessageText(msg)
	if got != "Alert: pods not ready" {
		t.Errorf("expected %q, got %q", "Alert: pods not ready", got)
	}
}

func TestExtractMessageText_AttachmentPretextTitleText(t *testing.T) {
	msg := slack.Message{}
	msg.Attachments = []slack.Attachment{
		{
			Pretext: "New alert fired",
			Title:   "Pod Health Check Failed",
			Text:    "2 pods not ready in namespace codewords",
		},
	}

	expected := "New alert fired\nPod Health Check Failed\n2 pods not ready in namespace codewords"
	got := extractMessageText(msg)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractMessageText_AttachmentFields(t *testing.T) {
	msg := slack.Message{}
	msg.Attachments = []slack.Attachment{
		{
			Title: "Deployment Status",
			Fields: []slack.AttachmentField{
				{Title: "Namespace", Value: "codewords", Short: true},
				{Title: "Pods", Value: "2/4 ready", Short: true},
			},
		},
	}

	expected := "Deployment Status\nNamespace: codewords\nPods: 2/4 ready"
	got := extractMessageText(msg)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractMessageText_AttachmentFieldValueOnly(t *testing.T) {
	msg := slack.Message{}
	msg.Attachments = []slack.Attachment{
		{
			Fields: []slack.AttachmentField{
				{Title: "", Value: "some value"},
			},
		},
	}

	got := extractMessageText(msg)
	if got != "some value" {
		t.Errorf("expected %q, got %q", "some value", got)
	}
}

func TestExtractMessageText_AttachmentFallback(t *testing.T) {
	msg := slack.Message{}
	msg.Attachments = []slack.Attachment{
		{Fallback: "fallback text only"},
	}

	got := extractMessageText(msg)
	if got != "fallback text only" {
		t.Errorf("expected %q, got %q", "fallback text only", got)
	}
}

func TestExtractMessageText_AttachmentFallbackNotUsedWhenTextPresent(t *testing.T) {
	msg := slack.Message{}
	msg.Attachments = []slack.Attachment{
		{Text: "real text", Fallback: "fallback text"},
	}

	got := extractMessageText(msg)
	if got != "real text" {
		t.Errorf("expected %q, got %q", "real text", got)
	}
}

func TestExtractMessageText_BlockKitHeader(t *testing.T) {
	msg := slack.Message{}
	msg.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			&slack.HeaderBlock{
				Type: slack.MBTHeader,
				Text: &slack.TextBlockObject{Type: "plain_text", Text: "Alert Header"},
			},
		},
	}

	got := extractMessageText(msg)
	if got != "Alert Header" {
		t.Errorf("expected %q, got %q", "Alert Header", got)
	}
}

func TestExtractMessageText_BlockKitSection(t *testing.T) {
	msg := slack.Message{}
	msg.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			&slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{Type: "mrkdwn", Text: "Deployment *codewords* is unhealthy"},
			},
		},
	}

	got := extractMessageText(msg)
	if got != "Deployment *codewords* is unhealthy" {
		t.Errorf("expected %q, got %q", "Deployment *codewords* is unhealthy", got)
	}
}

func TestExtractMessageText_BlockKitSectionWithFields(t *testing.T) {
	msg := slack.Message{}
	msg.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			&slack.SectionBlock{
				Type: slack.MBTSection,
				Fields: []*slack.TextBlockObject{
					{Type: "mrkdwn", Text: "*Status:* Failing"},
					{Type: "mrkdwn", Text: "*Pods:* 0/2"},
				},
			},
		},
	}

	expected := "*Status:* Failing\n*Pods:* 0/2"
	got := extractMessageText(msg)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractMessageText_BlockKitHeaderAndSection(t *testing.T) {
	msg := slack.Message{}
	msg.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			&slack.HeaderBlock{
				Type: slack.MBTHeader,
				Text: &slack.TextBlockObject{Type: "plain_text", Text: "Alert Fired"},
			},
			&slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{Type: "mrkdwn", Text: "Pod health check failed"},
			},
		},
	}

	expected := "Alert Fired\nPod health check failed"
	got := extractMessageText(msg)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractMessageText_FilesOnly(t *testing.T) {
	msg := slack.Message{}
	msg.Files = []slack.File{
		{Title: "screenshot.png"},
	}

	got := extractMessageText(msg)
	if got != "[File: screenshot.png]" {
		t.Errorf("expected %q, got %q", "[File: screenshot.png]", got)
	}
}

func TestExtractMessageText_FilesFallbackToName(t *testing.T) {
	msg := slack.Message{}
	msg.Files = []slack.File{
		{Name: "report.pdf"},
	}

	got := extractMessageText(msg)
	if got != "[File: report.pdf]" {
		t.Errorf("expected %q, got %q", "[File: report.pdf]", got)
	}
}

func TestExtractMessageText_MultipleFiles(t *testing.T) {
	msg := slack.Message{}
	msg.Files = []slack.File{
		{Title: "logs.txt"},
		{Title: "config.yaml"},
	}

	expected := "[File: logs.txt]\n[File: config.yaml]"
	got := extractMessageText(msg)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractMessageText_AttachmentsTakePrecedenceOverBlocks(t *testing.T) {
	msg := slack.Message{}
	msg.Attachments = []slack.Attachment{
		{Text: "from attachment"},
	}
	msg.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			&slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{Type: "mrkdwn", Text: "from blocks"},
			},
		},
	}

	got := extractMessageText(msg)
	if got != "from attachment" {
		t.Errorf("expected attachments to take precedence over blocks, got %q", got)
	}
}

func TestExtractRichTextBlock_Section(t *testing.T) {
	block := &slack.RichTextBlock{
		Type: slack.MBTRichText,
		Elements: []slack.RichTextElement{
			&slack.RichTextSection{
				Type: slack.RTESection,
				Elements: []slack.RichTextSectionElement{
					&slack.RichTextSectionTextElement{
						Type: slack.RTSEText,
						Text: "Hello ",
					},
					&slack.RichTextSectionTextElement{
						Type: slack.RTSEText,
						Text: "world",
					},
				},
			},
		},
	}

	parts := extractRichTextBlock(block)
	if len(parts) != 1 || parts[0] != "Hello world" {
		t.Errorf("expected [\"Hello world\"], got %v", parts)
	}
}

func TestExtractRichTextBlock_Link(t *testing.T) {
	block := &slack.RichTextBlock{
		Type: slack.MBTRichText,
		Elements: []slack.RichTextElement{
			&slack.RichTextSection{
				Type: slack.RTESection,
				Elements: []slack.RichTextSectionElement{
					&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "Visit "},
					&slack.RichTextSectionLinkElement{Type: slack.RTSELink, Text: "Google", URL: "https://google.com"},
				},
			},
		},
	}

	parts := extractRichTextBlock(block)
	if len(parts) != 1 || parts[0] != "Visit Google" {
		t.Errorf("expected [\"Visit Google\"], got %v", parts)
	}
}

func TestExtractRichTextBlock_LinkFallbackToURL(t *testing.T) {
	block := &slack.RichTextBlock{
		Type: slack.MBTRichText,
		Elements: []slack.RichTextElement{
			&slack.RichTextSection{
				Type: slack.RTESection,
				Elements: []slack.RichTextSectionElement{
					&slack.RichTextSectionLinkElement{Type: slack.RTSELink, URL: "https://example.com"},
				},
			},
		},
	}

	parts := extractRichTextBlock(block)
	if len(parts) != 1 || parts[0] != "https://example.com" {
		t.Errorf("expected [\"https://example.com\"], got %v", parts)
	}
}

func TestExtractRichTextBlock_List(t *testing.T) {
	block := &slack.RichTextBlock{
		Type: slack.MBTRichText,
		Elements: []slack.RichTextElement{
			&slack.RichTextList{
				Type: slack.RTEList,
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: slack.RTESection,
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "First item"},
						},
					},
					&slack.RichTextSection{
						Type: slack.RTESection,
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "Second item"},
						},
					},
				},
			},
		},
	}

	parts := extractRichTextBlock(block)
	if len(parts) != 2 || parts[0] != "- First item" || parts[1] != "- Second item" {
		t.Errorf("expected [\"- First item\", \"- Second item\"], got %v", parts)
	}
}

func TestExtractRichTextBlock_Quote(t *testing.T) {
	block := &slack.RichTextBlock{
		Type: slack.MBTRichText,
		Elements: []slack.RichTextElement{
			&slack.RichTextQuote{
				Type: slack.RTEQuote,
				Elements: []slack.RichTextSectionElement{
					&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "quoted text"},
				},
			},
		},
	}

	parts := extractRichTextBlock(block)
	if len(parts) != 1 || parts[0] != "> quoted text" {
		t.Errorf("expected [\"> quoted text\"], got %v", parts)
	}
}

func TestExtractRichTextBlock_Preformatted(t *testing.T) {
	block := &slack.RichTextBlock{
		Type: slack.MBTRichText,
		Elements: []slack.RichTextElement{
			&slack.RichTextPreformatted{
				RichTextSection: slack.RichTextSection{
					Type: slack.RTEPreformatted,
					Elements: []slack.RichTextSectionElement{
						&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "fmt.Println(\"hi\")"},
					},
				},
			},
		},
	}

	parts := extractRichTextBlock(block)
	expected := "```\nfmt.Println(\"hi\")\n```"
	if len(parts) != 1 || parts[0] != expected {
		t.Errorf("expected [%q], got %v", expected, parts)
	}
}

func TestExtractRichTextBlock_MixedElements(t *testing.T) {
	block := &slack.RichTextBlock{
		Type: slack.MBTRichText,
		Elements: []slack.RichTextElement{
			&slack.RichTextSection{
				Type: slack.RTESection,
				Elements: []slack.RichTextSectionElement{
					&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "intro text"},
				},
			},
			&slack.RichTextQuote{
				Type: slack.RTEQuote,
				Elements: []slack.RichTextSectionElement{
					&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "a quote"},
				},
			},
			&slack.RichTextPreformatted{
				RichTextSection: slack.RichTextSection{
					Type: slack.RTEPreformatted,
					Elements: []slack.RichTextSectionElement{
						&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "code here"},
					},
				},
			},
		},
	}

	parts := extractRichTextBlock(block)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %v", len(parts), parts)
	}
	if parts[0] != "intro text" {
		t.Errorf("parts[0]: expected %q, got %q", "intro text", parts[0])
	}
	if parts[1] != "> a quote" {
		t.Errorf("parts[1]: expected %q, got %q", "> a quote", parts[1])
	}
	if parts[2] != "```\ncode here\n```" {
		t.Errorf("parts[2]: expected %q, got %q", "```\ncode here\n```", parts[2])
	}
}

func TestExtractRichTextBlock_EmptyBlock(t *testing.T) {
	block := &slack.RichTextBlock{
		Type:     slack.MBTRichText,
		Elements: []slack.RichTextElement{},
	}

	parts := extractRichTextBlock(block)
	if len(parts) != 0 {
		t.Errorf("expected empty slice, got %v", parts)
	}
}

func TestExtractMessageText_RichTextBlock(t *testing.T) {
	msg := slack.Message{}
	msg.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			&slack.RichTextBlock{
				Type: slack.MBTRichText,
				Elements: []slack.RichTextElement{
					&slack.RichTextSection{
						Type: slack.RTESection,
						Elements: []slack.RichTextSectionElement{
							&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "Alert: pods not ready"},
						},
					},
					&slack.RichTextList{
						Type: slack.RTEList,
						Elements: []slack.RichTextElement{
							&slack.RichTextSection{
								Type: slack.RTESection,
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "Pod A: CrashLoopBackOff"},
								},
							},
							&slack.RichTextSection{
								Type: slack.RTESection,
								Elements: []slack.RichTextSectionElement{
									&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "Pod B: ImagePullBackOff"},
								},
							},
						},
					},
				},
			},
		},
	}

	expected := "Alert: pods not ready\n- Pod A: CrashLoopBackOff\n- Pod B: ImagePullBackOff"
	got := extractMessageText(msg)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
