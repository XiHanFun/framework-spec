package rules_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestAINamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.AINames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxAINameLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxAINameLength)
	}
	replayGrammar(t, "AI name", vectors.MaxLength, vectors.Valid, vectors.Invalid, rules.IsAIName)
}

func TestAIToolNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.AIToolNames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxAIToolNameLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxAIToolNameLength)
	}
	replayGrammar(t, "AI tool name", vectors.MaxLength, vectors.Valid, vectors.Invalid, rules.IsAIToolName)
	atLimit, shortened, trimmed := false, false, false
	seen := map[string]string{}
	for _, c := range vectors.Cases {
		if !rules.IsOperationID(c.Operation) {
			t.Errorf("%s: %q is not an operation ID", c.Name, c.Operation)
		}
		if got := rules.AIToolName(c.Operation); got != c.Tool {
			t.Errorf("%s: tool name = %q, want %q", c.Name, got, c.Tool)
		}
		if got := rules.AIToolDescriptionKey(c.Operation); got != c.DescriptionKey {
			t.Errorf("%s: description key = %q, want %q", c.Name, got, c.DescriptionKey)
		}
		if !rules.IsAIToolName(c.Tool) || strings.Contains(c.Tool, "__") {
			t.Errorf("%s: the tool name %q must be a valid tool name without two underscores in a row", c.Name, c.Tool)
		}
		// 推出的名字要稳定且互不相同：不同的操作 ID 不撞名
		if other, taken := seen[c.Tool]; taken {
			t.Errorf("%s: %q and %q derive the same tool name %q", c.Name, c.Operation, other, c.Tool)
		}
		seen[c.Tool] = c.Operation
		// 不带下划线的资源与动词：ID 的长度就是推出的全名的长度
		atLimit = atLimit || (len(c.Operation) == vectors.MaxLength && c.Tool == rules.AIToolName(c.Operation) && len(c.Tool) == vectors.MaxLength)
		shortened = shortened || len(c.Operation) == vectors.MaxLength+1
		// 前 55 个字符正好以下划线结尾的：去掉它之后名字比上限短一个字符
		trimmed = trimmed || (len(c.Operation) > vectors.MaxLength && len(c.Tool) == vectors.MaxLength-1)
	}
	if !atLimit || !shortened || !trimmed {
		t.Errorf("the vector must keep a name of exactly %d characters that is kept (found: %v), one of %d that is shortened (found: %v) and one whose prefix ends with an underscore (found: %v)", vectors.MaxLength, atLimit, vectors.MaxLength+1, shortened, trimmed)
	}
}

func TestKnowledgeChunksFollowTheVectors(t *testing.T) {
	vectors, err := conformance.KnowledgeChunks()
	if err != nil {
		t.Fatal(err)
	}
	if !rules.IsErrorCode(vectors.Code) || !rules.IsAIName(vectors.Defaults.Collection) {
		t.Errorf("code %q must be an error code and the default collection %q a valid name", vectors.Code, vectors.Defaults.Collection)
	}
	for _, c := range vectors.Chunks {
		if got := rules.ChunkFixedWindow(c.Text, c.MaxChunkSize, c.Overlap); !slices.Equal(got, c.Chunks) {
			t.Errorf("%s: chunks = %q, want %q", c.Name, got, c.Chunks)
		}
	}
	for _, c := range vectors.RecordIDs {
		if got := rules.KnowledgeRecordID(c.DocumentID, c.ChunkIndex); got != c.ID {
			t.Errorf("record ID of %q chunk %d = %q, want %q", c.DocumentID, c.ChunkIndex, got, c.ID)
		}
	}
	for _, c := range vectors.DefaultCollections {
		tenant := ""
		if c.Tenant != nil {
			tenant = *c.Tenant
		}
		collection, ok := rules.DefaultKnowledgeCollection(c.Collection, tenant)
		if ok != (c.DefaultCollection != nil) || (ok && collection != *c.DefaultCollection) {
			t.Errorf("%s: default collection = %q (valid %v), want %v", c.Name, collection, ok, c.DefaultCollection)
		}
	}
}

func TestAIProviderDefaultsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.AIProviderDefaults()
	if err != nil {
		t.Fatal(err)
	}
	if !rules.IsErrorCode(vectors.Code) || !rules.IsMessageKey(vectors.ChatMessageKey) || !rules.IsMessageKey(vectors.EmbeddingMessageKey) {
		t.Errorf("code %q and message keys %q, %q must be well-formed", vectors.Code, vectors.ChatMessageKey, vectors.EmbeddingMessageKey)
	}
	text := func(value *string) string {
		if value == nil {
			return ""
		}
		return *value
	}
	for _, c := range vectors.Cases {
		chat, embedding := rules.DefaultAIProviders(c.Providers, text(c.DefaultProvider), text(c.DefaultEmbeddingProvider))
		if chat != text(c.Chat) || embedding != text(c.Embedding) {
			t.Errorf("%s: defaults = %q and %q, want %q and %q", c.Name, chat, embedding, text(c.Chat), text(c.Embedding))
		}
	}
}

func TestAIPromptTemplatesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.AIPromptTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if !rules.IsErrorCode(vectors.Code) || !rules.IsMessageKey(vectors.MessageKey) {
		t.Errorf("code %q and message key %q must be well-formed", vectors.Code, vectors.MessageKey)
	}
	for _, c := range vectors.Cases {
		if got, _, ok := rules.RenderAIPrompt(c.Template, c.Arguments); !ok || got != c.Text {
			t.Errorf("%s: rendered %q (ok %v), want %q", c.Name, got, ok, c.Text)
		}
	}
	for _, c := range vectors.Missing {
		if got, missing, ok := rules.RenderAIPrompt(c.Template, c.Arguments); ok || missing != c.Argument {
			t.Errorf("%s: rendered %q (ok %v) with %q missing, want a failure with %q missing", c.Name, got, ok, missing, c.Argument)
		}
	}
}
